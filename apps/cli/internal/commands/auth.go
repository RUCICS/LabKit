package commands

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"labkit.local/apps/cli/internal/config"
	keycrypto "labkit.local/apps/cli/internal/crypto"
	httpclient "labkit.local/apps/cli/internal/http"
	"labkit.local/apps/cli/internal/ui"
	"labkit.local/packages/go/auth"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const serverURLEnvName = "LABKIT_SERVER_URL"

type Dependencies struct {
	ConfigDir         string
	BinaryName        string
	DeviceName        string
	ServerURL         string
	ServerURLOverride string
	Lab               string
	HTTPClient        *http.Client
	Now               func() time.Time
	Sleep             func(time.Duration)
	PollInterval      time.Duration
	OpenBrowser       func(string) error
	Out               io.Writer
	Err               io.Writer
	In                io.Reader
	IsTTY             func() bool
	ReadPassword      func(io.Writer, string) ([]byte, error)
}

func NewRootCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)

	cmd := &cobra.Command{
		Use:           deps.BinaryName,
		Short:         deps.BinaryName + " CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetOut(deps.Out)
	cmd.SetErr(deps.Err)
	cmd.PersistentFlags().StringVar(&deps.ConfigDir, "config-dir", deps.ConfigDir, "Path to the local LabKit config directory")
	if flag := cmd.PersistentFlags().Lookup("config-dir"); flag != nil {
		flag.DefValue = friendlyPathForHelp(deps.ConfigDir)
	}
	cmd.PersistentFlags().StringVar(&deps.ServerURLOverride, "server-url", deps.ServerURLOverride, "LabKit server URL")
	cmd.PersistentFlags().StringVar(&deps.Lab, "lab", deps.Lab, "Active lab id")
	cmd.AddCommand(NewAuthCommand(deps))
	cmd.AddCommand(NewKeysCommand(deps))
	cmd.AddCommand(NewRevokeCommand(deps))
	cmd.AddCommand(NewSubmitCommand(deps))
	cmd.AddCommand(NewWebCommand(deps))
	cmd.AddCommand(NewBoardCommand(deps))
	cmd.AddCommand(NewHistoryCommand(deps))
	cmd.AddCommand(NewNickCommand(deps))
	cmd.AddCommand(NewTrackCommand(deps))
	return cmd
}

func NewAuthCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Bind this machine to your account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rotateKey, err := cmd.Flags().GetBool("rotate-key")
			if err != nil {
				return err
			}
			encrypt, err := cmd.Flags().GetBool("encrypt")
			if err != nil {
				return err
			}
			noEncrypt, err := cmd.Flags().GetBool("no-encrypt")
			if err != nil {
				return err
			}
			if encrypt && noEncrypt {
				return fmt.Errorf("--encrypt and --no-encrypt cannot be used together")
			}
			localCfg, _, err := config.FindLocalConfig("")
			if err != nil {
				return err
			}
			globalCfg, legacyCfg, err := readAuthConfigs(deps.ConfigDir)
			if err != nil {
				return err
			}
			serverURL, err := resolveAuthServerURL(deps, localCfg, globalCfg, legacyCfg)
			if err != nil {
				return err
			}
			if serverURL == "" {
				return fmt.Errorf("server URL is required")
			}

			serverOrigin, err := config.NormalizeServerOrigin(serverURL)
			if err != nil {
				return err
			}
			keySelection, err := selectAuthKey(deps, serverOrigin, globalCfg, rotateKey, encrypt, noEncrypt)
			if err != nil {
				return err
			}

			client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
			if err != nil {
				return err
			}

			theme := ui.DefaultTheme()
			printAuthKeyBanner(deps.Out, theme, keySelection.Action, serverOrigin, keySelection.Fingerprint)
			fmt.Fprintln(deps.Out)
			authorization, err := client.deviceAuthorize(cmd.Context(), keySelection.PublicKey, deps.DeviceName)
			if err != nil {
				return err
			}
			verificationURL, err := resolveVerificationURL(serverURL, authorization.VerificationURL)
			if err != nil {
				return err
			}
			fmt.Fprintln(deps.Out, "  Open this URL in your browser:")
			fmt.Fprintln(deps.Out, "  "+theme.InfoStyle.Render(verificationURL))
			fmt.Fprintln(deps.Out)
			fmt.Fprintf(deps.Out, "  Enter code:  %s\n", theme.TitleStyle.Render(authorization.UserCode))
			fmt.Fprintln(deps.Out)

			pollInterval := deps.PollInterval
			if pollInterval <= 0 {
				pollInterval = time.Second
			}
			waitSpinner := newLineSpinner(deps.Out, deps.IsTTY(), "Waiting for authorization…")
			if err := waitSpinner.Start(); err != nil {
				return err
			}
			stopWaitSpinner := func() {
				_ = waitSpinner.Stop()
			}

			for {
				poll, err := client.devicePoll(cmd.Context(), authorization.DeviceCode)
				if err != nil {
					stopWaitSpinner()
					return err
				}
				switch poll.Status {
				case "pending":
					if deps.Sleep != nil {
						deps.Sleep(pollInterval)
					} else {
						time.Sleep(pollInterval)
					}
				case "approved":
					stopWaitSpinner()
					goto finished
				case "expired":
					stopWaitSpinner()
					return fmt.Errorf("device authorization expired")
				default:
					stopWaitSpinner()
					return fmt.Errorf("unexpected device authorization status %q", poll.Status)
				}
			}

		finished:
			if err := persistAuthKeyringEntry(deps.ConfigDir, globalCfg, serverOrigin, keySelection); err != nil {
				return err
			}
			completionTitle := theme.SuccessStyle.Render("✓") + " " + theme.TitleStyle.Render("Authorized") + "  " + theme.MutedStyle.Render(serverOrigin)
			fmt.Fprintln(deps.Out, completionTitle)
			fmt.Fprintln(deps.Out)
			fmt.Fprintf(deps.Out, "  %s  %s\n", theme.MutedStyle.Render("fingerprint"), theme.ValueStyle.Render(keySelection.Fingerprint))
			fmt.Fprintf(deps.Out, "  %s  %s\n", theme.MutedStyle.Render("server"), theme.ValueStyle.Render(serverOrigin))
			return nil
		},
	}
	cmd.Flags().Bool("rotate-key", false, "Generate a new key for the active server")
	cmd.Flags().Bool("encrypt", false, "Encrypt the generated private key with a passphrase")
	cmd.Flags().Bool("no-encrypt", false, "Store the generated private key without a passphrase")
	return cmd
}

type apiClient struct {
	base *httpclient.Client
	now  func() time.Time
}

type deviceAuthorizeResponse struct {
	DeviceCode      string    `json:"device_code"`
	UserCode        string    `json:"user_code"`
	VerificationURL string    `json:"verification_url"`
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expires_at"`
	KeyID           int64     `json:"key_id,omitempty"`
}

type devicePollResponse struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	Status     string `json:"status"`
	StudentID  string `json:"student_id,omitempty"`
	KeyID      int64  `json:"key_id,omitempty"`
}

type keyListResponse struct {
	Keys []keyListEntry `json:"keys"`
}

type keyListEntry struct {
	ID         int64     `json:"id"`
	PublicKey  string    `json:"public_key"`
	DeviceName string    `json:"device_name"`
	CreatedAt  time.Time `json:"created_at"`
}

func newAPIClient(baseURL string, httpClient *http.Client, now func() time.Time) (*apiClient, error) {
	base, err := httpclient.New(baseURL, httpClient)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &apiClient{base: base, now: now}, nil
}

func (c *apiClient) deviceAuthorize(ctx context.Context, publicKey, deviceName string) (deviceAuthorizeResponse, error) {
	req, err := c.base.NewRequest(ctx, http.MethodPost, "/api/device/authorize", map[string]string{
		"public_key":  publicKey,
		"device_name": strings.TrimSpace(deviceName),
	})
	if err != nil {
		return deviceAuthorizeResponse{}, err
	}
	var result deviceAuthorizeResponse
	if err := c.doJSON(req, &result); err != nil {
		return deviceAuthorizeResponse{}, err
	}
	return result, nil
}

func (c *apiClient) devicePoll(ctx context.Context, deviceCode string) (devicePollResponse, error) {
	req, err := c.base.NewRequest(ctx, http.MethodPost, "/api/device/poll?device_code="+url.QueryEscape(deviceCode), nil)
	if err != nil {
		return devicePollResponse{}, err
	}
	var result devicePollResponse
	if err := c.doJSON(req, &result); err != nil {
		return devicePollResponse{}, err
	}
	return result, nil
}

func (c *apiClient) listKeys(ctx context.Context, cfg config.Config, private ed25519.PrivateKey) ([]keyListEntry, error) {
	req, err := c.signedRequest(ctx, http.MethodGet, "/api/keys", nil, cfg, private)
	if err != nil {
		return nil, err
	}
	var result keyListResponse
	if err := c.doJSON(req, &result); err != nil {
		return nil, err
	}
	return result.Keys, nil
}

func (c *apiClient) revokeKey(ctx context.Context, cfg config.Config, private ed25519.PrivateKey, keyID int64) error {
	path := "/api/keys/" + strconv.FormatInt(keyID, 10)
	req, err := c.signedRequest(ctx, http.MethodDelete, path, nil, cfg, private)
	if err != nil {
		return err
	}
	resp, err := c.base.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke key failed: %s", decodeAPIErrorMessage(body))
	}
	return nil
}

func (c *apiClient) signedRequest(ctx context.Context, method, path string, body any, cfg config.Config, private ed25519.PrivateKey) (*http.Request, error) {
	if cfg.KeyID == 0 {
		return nil, fmt.Errorf("key id is required")
	}
	var payloadBytes []byte
	if body != nil {
		var err error
		payloadBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := c.base.NewRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}
	now := c.now().UTC()
	payload := auth.NewPayload(strings.ToUpper(strings.TrimSpace(method))+" "+path, now, nonce, nil).
		WithContentHash(sha256Hex(payloadBytes))
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(private, signingBytes)
	fingerprint, err := keyFingerprintFromPrivateKey(private)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-LabKit-Key-Fingerprint", fingerprint)
	req.Header.Set("X-LabKit-Timestamp", now.Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", nonce)
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString(sig))
	return req, nil
}

func keyFingerprintFromPrivateKey(private ed25519.PrivateKey) (string, error) {
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("private key does not expose ed25519 public key")
	}
	return auth.PublicKeyFingerprint(public), nil
}

func (c *apiClient) doJSON(req *http.Request, target any) error {
	resp, err := c.base.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: %s", decodeAPIErrorMessage(body))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func decodeAPIErrorMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "unknown error"
	}
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error.Message) != "" {
		return strings.TrimSpace(payload.Error.Message)
	}
	return trimmed
}

func normalizeDependencies(deps *Dependencies) *Dependencies {
	if deps == nil {
		deps = &Dependencies{}
	}
	if strings.TrimSpace(deps.BinaryName) == "" {
		deps.BinaryName = defaultBinaryName()
	}
	if strings.TrimSpace(deps.DeviceName) == "" {
		deps.DeviceName = defaultDeviceName()
	}
	if deps.ConfigDir == "" {
		deps.ConfigDir = config.ResolveDir()
	}
	if strings.TrimSpace(deps.ServerURL) == "" {
		deps.ServerURL = config.ResolveServerURL()
	}
	if deps.HTTPClient == nil {
		deps.HTTPClient = http.DefaultClient
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Sleep == nil {
		deps.Sleep = time.Sleep
	}
	if deps.PollInterval <= 0 {
		deps.PollInterval = time.Second
	}
	if deps.OpenBrowser == nil {
		deps.OpenBrowser = defaultOpenBrowser
	}
	if deps.Out == nil {
		deps.Out = os.Stdout
	}
	if deps.Err == nil {
		deps.Err = os.Stderr
	}
	if deps.In == nil {
		deps.In = os.Stdin
	}
	if deps.IsTTY == nil {
		deps.IsTTY = defaultIsTTY
	}
	if deps.ReadPassword == nil {
		deps.ReadPassword = defaultReadPassword
	}
	return deps
}

func defaultBinaryName() string {
	name := strings.TrimSpace(filepath.Base(os.Args[0]))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "labkit"
	}
	return name
}

func friendlyPathForHelp(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return cleaned
	}
	home = filepath.Clean(strings.TrimSpace(home))
	if home == "" {
		return cleaned
	}
	if cleaned == home {
		return "~"
	}
	prefix := home + string(filepath.Separator)
	if strings.HasPrefix(cleaned, prefix) {
		return "~" + string(filepath.Separator) + strings.TrimPrefix(cleaned, prefix)
	}
	return cleaned
}

func defaultDeviceName() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "unknown"
	}
	return name
}

func resolveConfig(deps *Dependencies) (config.Config, error) {
	globalCfg, legacyCfg, err := readAuthConfigs(deps.ConfigDir)
	if err != nil {
		return config.Config{}, err
	}
	cfg := config.Config{}
	if legacyCfg != nil {
		cfg = *legacyCfg
	}
	localCfg, _, err := config.FindLocalConfig("")
	if err != nil {
		return config.Config{}, err
	}
	if strings.TrimSpace(localCfg.Lab) != "" {
		cfg.Lab = strings.TrimSpace(localCfg.Lab)
	}
	serverURL, err := resolveAuthServerURL(deps, localCfg, globalCfg, legacyCfg)
	if err != nil {
		return config.Config{}, err
	}
	cfg.ServerURL = serverURL
	if legacyCfg == nil {
		entry, ok, err := globalCfg.LookupServerEntry(serverURL)
		if err != nil {
			return config.Config{}, err
		}
		if ok {
			cfg.KeyPath = strings.TrimSpace(entry.KeyPath)
			if cfg.KeyPath == "" {
				origin, err := config.NormalizeServerOrigin(serverURL)
				if err != nil {
					return config.Config{}, err
				}
				cfg.KeyPath = serverKeyPath(deps.ConfigDir, origin)
			}
			if cfg.KeyPath != "" {
				cfg.KeyID = 1
			}
		}
	}
	if cfg.KeyPath == "" {
		cfg.KeyPath = config.ResolveKeyPath(deps.ConfigDir)
	}
	if strings.TrimSpace(deps.Lab) != "" {
		cfg.Lab = strings.TrimSpace(deps.Lab)
	}
	return cfg, nil
}

type authKeySelection struct {
	Private     ed25519.PrivateKey
	PublicKey   string
	Fingerprint string
	KeyPath     string
	Action      string
	Encrypted   bool
}

func readAuthConfigs(configDir string) (config.GlobalConfig, *config.Config, error) {
	globalCfg, globalErr := config.ReadGlobal(configDir)
	if globalErr == nil {
		return globalCfg, nil, nil
	}
	legacyCfg, legacyErr := config.Read(configDir)
	if legacyErr == nil {
		return config.GlobalConfig{}, &legacyCfg, nil
	}
	return config.GlobalConfig{}, nil, globalErr
}

func resolveAuthServerURL(deps *Dependencies, localCfg config.ProjectConfig, globalCfg config.GlobalConfig, legacyCfg *config.Config) (string, error) {
	serverURL := ""
	if legacyCfg != nil {
		serverURL = strings.TrimSpace(legacyCfg.ServerURL)
	}
	if global := strings.TrimSpace(globalCfg.DefaultServerURL); global != "" {
		serverURL = global
	}
	if local := strings.TrimSpace(localCfg.ServerURL); local != "" {
		serverURL = local
	}
	if envServerURL := strings.TrimSpace(os.Getenv(serverURLEnvName)); envServerURL != "" {
		serverURL = envServerURL
	}
	if override := strings.TrimSpace(deps.ServerURLOverride); override != "" {
		serverURL = override
	}
	if serverURL == "" {
		serverURL = strings.TrimSpace(deps.ServerURL)
	}
	if serverURL == "" {
		return "", fmt.Errorf("server URL is required")
	}
	return serverURL, nil
}

func selectAuthKey(deps *Dependencies, serverOrigin string, globalCfg config.GlobalConfig, rotate, encrypt, noEncrypt bool) (authKeySelection, error) {
	deps = normalizeDependencies(deps)
	configDir := deps.ConfigDir
	entry, _, err := globalCfg.LookupServerEntry(serverOrigin)
	if err != nil {
		return authKeySelection{}, err
	}

	keyPath := strings.TrimSpace(entry.KeyPath)
	if keyPath == "" {
		keyPath = serverKeyPath(configDir, serverOrigin)
	}

	if !rotate {
		private, err := readPrivateKeyWithDeps(deps, keyPath)
		if err == nil {
			selection, err := authKeySelectionFromPrivate(private, keyPath, "Reusing existing key")
			if err != nil {
				return authKeySelection{}, err
			}
			selection.Encrypted = entry.Encrypted
			return selection, nil
		}
		if !os.IsNotExist(err) {
			return authKeySelection{}, err
		}
	}

	pair, err := keycrypto.GenerateKeyPair()
	if err != nil {
		return authKeySelection{}, err
	}
	action := "Creating new key"
	if rotate {
		action = "Rotating key"
	}
	selection, err := authKeySelectionFromPair(pair, keyPath, action)
	if err != nil {
		return authKeySelection{}, err
	}
	shouldEncrypt, err := resolveEncryptionChoice(deps, encrypt, noEncrypt)
	if err != nil {
		return authKeySelection{}, err
	}
	if shouldEncrypt {
		passphrase, err := promptPassphraseTwice(deps)
		if err != nil {
			return authKeySelection{}, err
		}
		if err := keycrypto.WriteEncryptedPrivateKey(keyPath, pair.Private, passphrase); err != nil {
			return authKeySelection{}, err
		}
		selection.Encrypted = true
	} else {
		if err := keycrypto.WritePrivateKey(keyPath, pair.Private); err != nil {
			return authKeySelection{}, err
		}
	}
	return selection, nil
}

func authKeySelectionFromPrivate(private ed25519.PrivateKey, keyPath, action string) (authKeySelection, error) {
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok {
		return authKeySelection{}, fmt.Errorf("private key does not expose ed25519 public key")
	}
	fingerprint, err := keycrypto.PublicKeyFingerprint(public)
	if err != nil {
		return authKeySelection{}, err
	}
	publicKey, err := keycrypto.PublicKeyString(public)
	if err != nil {
		return authKeySelection{}, err
	}
	return authKeySelection{
		Private:     private,
		PublicKey:   publicKey,
		Fingerprint: fingerprint,
		KeyPath:     keyPath,
		Action:      action,
		Encrypted:   false,
	}, nil
}

func authKeySelectionFromPair(pair keycrypto.KeyPair, keyPath, action string) (authKeySelection, error) {
	publicKey, err := keycrypto.PublicKeyString(pair.Public)
	if err != nil {
		return authKeySelection{}, err
	}
	fingerprint, err := keycrypto.PublicKeyFingerprint(pair.Public)
	if err != nil {
		return authKeySelection{}, err
	}
	return authKeySelection{
		Private:     pair.Private,
		PublicKey:   publicKey,
		Fingerprint: fingerprint,
		KeyPath:     keyPath,
		Action:      action,
		Encrypted:   false,
	}, nil
}

func persistAuthKeyringEntry(configDir string, globalCfg config.GlobalConfig, serverOrigin string, selection authKeySelection) error {
	globalCfg.DefaultServerURL = serverOrigin
	if err := globalCfg.SetServerEntry(serverOrigin, config.ServerEntry{
		KeyPath:        selection.KeyPath,
		KeyFingerprint: selection.Fingerprint,
		Encrypted:      selection.Encrypted,
	}); err != nil {
		return err
	}
	return config.WriteGlobal(configDir, globalCfg)
}

func printAuthKeyBanner(out io.Writer, theme ui.Theme, action, serverOrigin, fingerprint string) {
	if out == nil {
		out = io.Discard
	}
	titleLine := theme.InfoStyle.Render("●") + " " + theme.TitleStyle.Render("Authorizing") + "  " + theme.MutedStyle.Render(serverOrigin)
	fmt.Fprintln(out, titleLine)
	fmt.Fprintln(out)
	switch action {
	case "Reusing existing key":
		fmt.Fprintln(out, "  "+theme.MutedStyle.Render(action))
	case "Rotating key":
		fmt.Fprintln(out, "  "+theme.WarningStyle.Render(action))
	default:
		fmt.Fprintln(out, "  "+theme.InfoStyle.Render(action))
	}
	fmt.Fprintf(out, "  %s  %s\n", theme.MutedStyle.Render("fingerprint"), theme.ValueStyle.Render(fingerprint))
}

func serverKeyPath(configDir, serverOrigin string) string {
	parsed, err := url.Parse(serverOrigin)
	if err != nil {
		sum := sha256.Sum256([]byte(serverOrigin))
		return filepath.Join(configDir, "keys", hex.EncodeToString(sum[:8])+"_ed25519")
	}
	name := sanitizeKeyFileComponent(parsed.Hostname())
	if port := sanitizeKeyFileComponent(parsed.Port()); port != "" {
		name += "_" + port
	}
	if name == "" {
		sum := sha256.Sum256([]byte(serverOrigin))
		name = hex.EncodeToString(sum[:8])
	}
	return filepath.Join(configDir, "keys", name+"_ed25519")
}

func sanitizeKeyFileComponent(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "._-")
}

func newNonce() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func ReadPrivateKey(path string) (ed25519.PrivateKey, error) {
	return keycrypto.ReadPrivateKey(path)
}

func readPrivateKeyWithDeps(deps *Dependencies, path string) (ed25519.PrivateKey, error) {
	private, err := keycrypto.ReadPrivateKey(path)
	if err == nil {
		return private, nil
	}
	if !requiresPassphrase(err) {
		return nil, err
	}
	deps = normalizeDependencies(deps)
	if !deps.IsTTY() {
		return nil, fmt.Errorf("private key is encrypted; rerun in an interactive terminal")
	}
	passphrase, err := deps.ReadPassword(deps.Out, "  Passphrase: ")
	if err != nil {
		return nil, err
	}
	if len(passphrase) == 0 {
		return nil, fmt.Errorf("passphrase cannot be empty")
	}
	return keycrypto.ReadEncryptedPrivateKey(path, passphrase)
}

func WritePrivateKey(path string, private ed25519.PrivateKey) error {
	return keycrypto.WritePrivateKey(path, private)
}

func PublicKeyString(public ed25519.PublicKey) (string, error) {
	return keycrypto.PublicKeyString(public)
}

func resolveVerificationURL(baseURL, verificationURL string) (string, error) {
	ref, err := url.Parse(strings.TrimSpace(verificationURL))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ref.String()) == "" {
		return "", fmt.Errorf("verification URL is required")
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func defaultOpenBrowser(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target URL is required")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

func resolveEncryptionChoice(deps *Dependencies, encrypt, noEncrypt bool) (bool, error) {
	switch {
	case encrypt:
		return true, nil
	case noEncrypt:
		return false, nil
	}
	deps = normalizeDependencies(deps)
	if !deps.IsTTY() {
		return false, fmt.Errorf("non-interactive auth requires either --encrypt or --no-encrypt")
	}
	fmt.Fprint(deps.Out, "  Encrypt private key with a passphrase? [y/N]: ")
	reader := bufio.NewReader(deps.In)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func promptPassphraseTwice(deps *Dependencies) ([]byte, error) {
	deps = normalizeDependencies(deps)
	first, err := deps.ReadPassword(deps.Out, "  Passphrase: ")
	if err != nil {
		return nil, err
	}
	if len(first) == 0 {
		return nil, fmt.Errorf("passphrase cannot be empty")
	}
	second, err := deps.ReadPassword(deps.Out, "  Confirm passphrase: ")
	if err != nil {
		return nil, err
	}
	if string(first) != string(second) {
		return nil, fmt.Errorf("passphrases do not match")
	}
	return first, nil
}

func requiresPassphrase(err error) bool {
	return err != nil && strings.Contains(err.Error(), "passphrase required")
}

func defaultIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func defaultReadPassword(out io.Writer, prompt string) ([]byte, error) {
	if out == nil {
		out = io.Discard
	}
	fmt.Fprint(out, prompt)
	passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out)
	return passphrase, err
}
