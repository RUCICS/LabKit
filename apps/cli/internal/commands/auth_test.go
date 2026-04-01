package commands

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"labkit.local/apps/cli/internal/config"
	keycrypto "labkit.local/apps/cli/internal/crypto"
	"labkit.local/packages/go/auth"

	"golang.org/x/crypto/ssh"
)

func TestAuthCommandCreatesNewServerKeyAndWritesKeyring(t *testing.T) {
	configDir := t.TempDir()

	var authorizeCalls int
	var pollCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/authorize":
			authorizeCalls++
			var req struct {
				PublicKey string `json:"public_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode authorize body: %v", err)
			}
			if req.PublicKey == "" {
				t.Fatal("public_key was empty")
			}
			if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.PublicKey)); err != nil {
				t.Fatalf("public key was invalid authorized-key format: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_url": "/custom/device/verify",
				"status":           "pending",
				"expires_at":       "2026-03-31T12:10:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/poll":
			pollCalls++
			if got := r.URL.Query().Get("device_code"); got != "device-123" {
				t.Fatalf("device_code = %q, want %q", got, "device-123")
			}
			if pollCalls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"device_code": "device-123",
					"user_code":   "ABCD-1234",
					"status":      "pending",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-123",
				"user_code":   "ABCD-1234",
				"status":      "approved",
				"student_id":  "2026001",
				"key_id":      11,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	deps := &Dependencies{
		ConfigDir:         configDir,
		ServerURLOverride: srv.URL,
		HTTPClient:        srv.Client(),
		Now:               func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Sleep:             func(time.Duration) {},
		PollInterval:      time.Millisecond,
		Out:               &stdout,
		Err:               &stderr,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"auth", "--no-encrypt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v stderr=%s", err, stderr.String())
	}

	if authorizeCalls != 1 {
		t.Fatalf("authorizeCalls = %d, want 1", authorizeCalls)
	}
	if pollCalls != 2 {
		t.Fatalf("pollCalls = %d, want 2", pollCalls)
	}

	for _, want := range []string{
		"● Authorizing",
		"Creating new key",
		"fingerprint",
		srv.URL,
		"Open this URL in your browser:",
		"/custom/device/verify",
		"Enter code:",
		"ABCD-1234",
		"◐ Waiting for authorization",
		"✓ Authorized",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}

	cfg, err := config.ReadGlobal(configDir)
	if err != nil {
		t.Fatalf("ReadGlobal(config) error = %v", err)
	}
	if cfg.DefaultServerURL != srv.URL {
		t.Fatalf("config.DefaultServerURL = %q, want %q", cfg.DefaultServerURL, srv.URL)
	}
	entry, ok, err := cfg.LookupServerEntry(srv.URL)
	if err != nil {
		t.Fatalf("LookupServerEntry() error = %v", err)
	}
	if !ok {
		t.Fatal("server entry missing from global keyring")
	}
	if entry.KeyPath == "" {
		t.Fatal("key_path was empty")
	}
	if _, err := os.Stat(entry.KeyPath); err != nil {
		t.Fatalf("stat(%q) error = %v", entry.KeyPath, err)
	}
	privateKey, err := ReadPrivateKey(entry.KeyPath)
	if err != nil {
		t.Fatalf("ReadPrivateKey() error = %v", err)
	}
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	if entry.KeyFingerprint != wantFingerprint {
		t.Fatalf("config fingerprint = %q, want %q", entry.KeyFingerprint, wantFingerprint)
	}
	if entry.Encrypted {
		t.Fatal("expected unencrypted key entry")
	}
}

func TestAuthCommandReusesExistingServerKeyByDefault(t *testing.T) {
	configDir := t.TempDir()
	serverURL := "http://localhost:8083"
	keyPath := filepath.Join(configDir, "keys", "server.pem")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	if err := config.WriteGlobal(configDir, config.GlobalConfig{
		DefaultServerURL: serverURL,
		Servers: map[string]config.ServerEntry{
			serverURL: {
				KeyPath:        keyPath,
				KeyFingerprint: wantFingerprint,
				Encrypted:      false,
			},
		},
	}); err != nil {
		t.Fatalf("config.WriteGlobal() error = %v", err)
	}

	var authorizeCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/authorize":
			authorizeCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_url": "/api/device/verify",
				"status":           "pending",
				"expires_at":       "2026-03-31T12:10:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-123",
				"user_code":   "ABCD-1234",
				"status":      "approved",
				"key_id":      11,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	if err := config.WriteGlobal(configDir, config.GlobalConfig{
		DefaultServerURL: srv.URL,
		Servers: map[string]config.ServerEntry{
			srv.URL: {
				KeyPath:        keyPath,
				KeyFingerprint: wantFingerprint,
				Encrypted:      false,
			},
		},
	}); err != nil {
		t.Fatalf("config.WriteGlobal() error = %v", err)
	}

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Sleep:        func(time.Duration) {},
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if authorizeCalls != 1 {
		t.Fatalf("authorizeCalls = %d, want 1", authorizeCalls)
	}
	if !strings.Contains(stdout.String(), "Reusing existing key") {
		t.Fatalf("stdout = %q, want reuse banner", stdout.String())
	}
	if !strings.Contains(stdout.String(), "● Authorizing") {
		t.Fatalf("stdout = %q, want auth title", stdout.String())
	}
	cfg, err := config.ReadGlobal(configDir)
	if err != nil {
		t.Fatalf("ReadGlobal(config) error = %v", err)
	}
	entry, ok, err := cfg.LookupServerEntry(srv.URL)
	if err != nil {
		t.Fatalf("LookupServerEntry() error = %v", err)
	}
	if !ok {
		t.Fatal("server entry missing from global keyring")
	}
	if entry.KeyPath != keyPath {
		t.Fatalf("config key_path = %q, want %q", entry.KeyPath, keyPath)
	}
	if entry.KeyFingerprint != wantFingerprint {
		t.Fatalf("config fingerprint = %q, want %q", entry.KeyFingerprint, wantFingerprint)
	}
}

func TestAuthCommandRotatesExistingServerKeyWhenRequested(t *testing.T) {
	configDir := t.TempDir()
	serverURL := "http://localhost:8083"
	keyPath := filepath.Join(configDir, "keys", "server.pem")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}
	oldFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	if err := config.WriteGlobal(configDir, config.GlobalConfig{
		DefaultServerURL: serverURL,
		Servers: map[string]config.ServerEntry{
			serverURL: {
				KeyPath:        keyPath,
				KeyFingerprint: oldFingerprint,
				Encrypted:      false,
			},
		},
	}); err != nil {
		t.Fatalf("config.WriteGlobal() error = %v", err)
	}

	var authorizeCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/authorize":
			authorizeCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_url": "/api/device/verify",
				"status":           "pending",
				"expires_at":       "2026-03-31T12:10:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-123",
				"user_code":   "ABCD-1234",
				"status":      "approved",
				"key_id":      11,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := config.WriteGlobal(configDir, config.GlobalConfig{
		DefaultServerURL: srv.URL,
		Servers: map[string]config.ServerEntry{
			srv.URL: {
				KeyPath:        keyPath,
				KeyFingerprint: oldFingerprint,
				Encrypted:      false,
			},
		},
	}); err != nil {
		t.Fatalf("config.WriteGlobal() error = %v", err)
	}

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Sleep:        func(time.Duration) {},
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"auth", "--rotate-key", "--no-encrypt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if authorizeCalls != 1 {
		t.Fatalf("authorizeCalls = %d, want 1", authorizeCalls)
	}
	if !strings.Contains(stdout.String(), "Rotating key") {
		t.Fatalf("stdout = %q, want rotation banner", stdout.String())
	}
	if !strings.Contains(stdout.String(), "● Authorizing") {
		t.Fatalf("stdout = %q, want auth title", stdout.String())
	}

	cfg, err := config.ReadGlobal(configDir)
	if err != nil {
		t.Fatalf("ReadGlobal(config) error = %v", err)
	}
	entry, ok, err := cfg.LookupServerEntry(srv.URL)
	if err != nil {
		t.Fatalf("LookupServerEntry() error = %v", err)
	}
	if !ok {
		t.Fatal("server entry missing from global keyring")
	}
	if entry.KeyPath != keyPath {
		t.Fatalf("config key_path = %q, want %q", entry.KeyPath, keyPath)
	}
	if entry.KeyFingerprint == oldFingerprint {
		t.Fatalf("fingerprint = %q, want a rotated key", entry.KeyFingerprint)
	}
}

func TestAuthCommandEncryptsNewServerKeyWhenPromptAccepted(t *testing.T) {
	configDir := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/authorize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_url": "/api/device/verify",
				"status":           "pending",
				"expires_at":       "2026-03-31T12:10:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-123",
				"user_code":   "ABCD-1234",
				"status":      "approved",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var prompts int
	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:         configDir,
		ServerURLOverride: srv.URL,
		HTTPClient:        srv.Client(),
		Now:               func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Sleep:             func(time.Duration) {},
		PollInterval:      time.Millisecond,
		Out:               &stdout,
		Err:               io.Discard,
		In:                strings.NewReader("y\n"),
		IsTTY:             func() bool { return true },
		ReadPassword: func(_ io.Writer, _ string) ([]byte, error) {
			prompts++
			return []byte("correct horse battery staple"), nil
		},
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if prompts != 2 {
		t.Fatalf("ReadPassword called %d times, want 2", prompts)
	}
	cfg, err := config.ReadGlobal(configDir)
	if err != nil {
		t.Fatalf("ReadGlobal(config) error = %v", err)
	}
	entry, ok, err := cfg.LookupServerEntry(srv.URL)
	if err != nil {
		t.Fatalf("LookupServerEntry() error = %v", err)
	}
	if !ok {
		t.Fatal("server entry missing from global keyring")
	}
	if !entry.Encrypted {
		t.Fatal("entry.Encrypted = false, want true")
	}
	if _, err := keycrypto.ReadPrivateKey(entry.KeyPath); err == nil {
		t.Fatal("ReadPrivateKey() error = nil, want encrypted-key failure")
	}
	if _, err := keycrypto.ReadEncryptedPrivateKey(entry.KeyPath, []byte("correct horse battery staple")); err != nil {
		t.Fatalf("ReadEncryptedPrivateKey() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Encrypt private key with a passphrase?") {
		t.Fatalf("stdout = %q, want encryption prompt", stdout.String())
	}
}

func TestAuthCommandRejectsNonInteractiveCreateWithoutEncryptionFlag(t *testing.T) {
	configDir := t.TempDir()

	deps := &Dependencies{
		ConfigDir:         configDir,
		ServerURLOverride: "http://localhost:8083",
		HTTPClient:        http.DefaultClient,
		Out:               io.Discard,
		Err:               io.Discard,
		IsTTY:             func() bool { return false },
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"auth"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-interactive encryption error")
	}
	if !strings.Contains(err.Error(), "non-interactive auth requires either --encrypt or --no-encrypt") {
		t.Fatalf("error = %v, want explicit non-interactive guidance", err)
	}
}

func TestReadPrivateKeyWithDepsPromptsForEncryptedKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := keycrypto.WriteEncryptedPrivateKey(keyPath, priv, []byte("secret")); err != nil {
		t.Fatalf("WriteEncryptedPrivateKey() error = %v", err)
	}

	var stdout bytes.Buffer
	private, err := readPrivateKeyWithDeps(&Dependencies{
		Out:   &stdout,
		Err:   io.Discard,
		IsTTY: func() bool { return true },
		ReadPassword: func(_ io.Writer, prompt string) ([]byte, error) {
			if prompt == "" {
				t.Fatal("prompt was empty")
			}
			return []byte("secret"), nil
		},
	}, keyPath)
	if err != nil {
		t.Fatalf("readPrivateKeyWithDeps() error = %v", err)
	}
	if string(private.Public().(ed25519.PublicKey)) != string(pub) {
		t.Fatalf("public key mismatch after decrypt")
	}
}

func TestAuthCommandPrefersEnvironmentServerURLOverLocalAndGlobalConfig(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()
	workDir := filepath.Join(projectDir, "pkg", "student")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	localSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unexpected local server", http.StatusInternalServerError)
	}))
	defer localSrv.Close()

	envCalls := 0
	envSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		envCalls++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/authorize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "device-123",
				"user_code":        "ABCD-1234",
				"verification_url": "/api/device/verify",
				"status":           "pending",
				"expires_at":       "2026-03-31T12:10:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/device/poll":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-123",
				"user_code":   "ABCD-1234",
				"status":      "approved",
				"key_id":      11,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer envSrv.Close()

	if err := config.WriteGlobal(configDir, config.GlobalConfig{
		DefaultServerURL: localSrv.URL,
		Servers: map[string]config.ServerEntry{
			localSrv.URL: {
				KeyPath:        filepath.Join(configDir, "keys", "placeholder.pem"),
				KeyFingerprint: "SHA256:placeholder",
			},
		},
	}); err != nil {
		t.Fatalf("config.WriteGlobal() error = %v", err)
	}
	writeProjectConfig(t, projectDir, localSrv.URL, "sorting")
	t.Setenv("LABKIT_SERVER_URL", envSrv.URL)
	t.Chdir(workDir)

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   envSrv.Client(),
		Now:          func() time.Time { return time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC) },
		Sleep:        func(time.Duration) {},
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"auth", "--no-encrypt"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if envCalls == 0 {
		t.Fatal("environment server was not used")
	}
	if strings.Contains(stdout.String(), "unexpected local server") {
		t.Fatalf("stdout = %q, want env server only", stdout.String())
	}
	cfg, err := config.ReadGlobal(configDir)
	if err != nil {
		t.Fatalf("ReadGlobal(config) error = %v", err)
	}
	if cfg.DefaultServerURL != envSrv.URL {
		t.Fatalf("config.DefaultServerURL = %q, want %q", cfg.DefaultServerURL, envSrv.URL)
	}
}

func TestResolveConfigUsesGlobalKeyringEntry(t *testing.T) {
	configDir := t.TempDir()
	serverURL := "http://localhost:8083"
	keyPath := filepath.Join(configDir, "keys", "localhost_8083_ed25519")
	if err := config.WriteGlobal(configDir, config.GlobalConfig{
		DefaultServerURL: serverURL,
		Servers: map[string]config.ServerEntry{
			serverURL: {
				KeyPath:        keyPath,
				KeyFingerprint: "SHA256:test",
			},
		},
	}); err != nil {
		t.Fatalf("config.WriteGlobal() error = %v", err)
	}

	cfg, err := resolveConfig(&Dependencies{ConfigDir: configDir})
	if err != nil {
		t.Fatalf("resolveConfig() error = %v", err)
	}
	if cfg.ServerURL != serverURL {
		t.Fatalf("cfg.ServerURL = %q, want %q", cfg.ServerURL, serverURL)
	}
	if cfg.KeyPath != keyPath {
		t.Fatalf("cfg.KeyPath = %q, want %q", cfg.KeyPath, keyPath)
	}
	if cfg.KeyID == 0 {
		t.Fatal("cfg.KeyID = 0, want non-zero auth marker")
	}
}

func TestBoardCommandUsesLocalProjectConfigDefaults(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()
	workDir := filepath.Join(projectDir, "pkg", "student")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeProjectConfig(t, projectDir, "http://placeholder.invalid", "sorting")

	var stdout bytes.Buffer
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "sorting",
				"name": "Sorting",
				"manifest": map[string]any{
					"lab":    map[string]any{"id": "sorting", "name": "Sorting"},
					"metric": []map[string]any{{"id": "throughput", "name": "Throughput", "sort": "desc"}},
					"board":  map[string]any{"rank_by": "throughput"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/board":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":          "sorting",
				"selected_metric": "throughput",
				"metrics":         []map[string]any{{"id": "throughput", "name": "Throughput", "sort": "desc", "selected": true}},
				"rows":            []map[string]any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	writeProjectConfig(t, projectDir, srv.URL, "sorting")
	t.Chdir(workDir)

	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: srv.Client(),
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"board"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if calls == 0 {
		t.Fatal("local project config was not used")
	}
	if !strings.Contains(stdout.String(), "Leaderboard") {
		t.Fatalf("stdout = %q, want board output", stdout.String())
	}
}

func TestBoardCommandPrefersEnvironmentServerURLOverLocalProjectConfig(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()
	workDir := filepath.Join(projectDir, "pkg", "student")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	var localCalls int
	localSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localCalls++
		http.Error(w, "unexpected local server", http.StatusInternalServerError)
	}))
	defer localSrv.Close()

	var envCalls int
	envSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		envCalls++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "sorting",
				"name": "Sorting",
				"manifest": map[string]any{
					"lab":    map[string]any{"id": "sorting", "name": "Sorting"},
					"metric": []map[string]any{{"id": "throughput", "name": "Throughput", "sort": "desc"}},
					"board":  map[string]any{"rank_by": "throughput"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/labs/sorting/board":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lab_id":          "sorting",
				"selected_metric": "throughput",
				"metrics":         []map[string]any{{"id": "throughput", "name": "Throughput", "sort": "desc", "selected": true}},
				"rows":            []map[string]any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer envSrv.Close()

	writeProjectConfig(t, projectDir, localSrv.URL, "sorting")
	t.Setenv("LABKIT_SERVER_URL", envSrv.URL)
	t.Chdir(workDir)

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:  configDir,
		HTTPClient: envSrv.Client(),
		Out:        &stdout,
		Err:        io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"board"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if envCalls == 0 {
		t.Fatal("environment server was not used")
	}
	if localCalls != 0 {
		t.Fatalf("localCalls = %d, want 0", localCalls)
	}
}

func TestKeysCommandsListAndRevokeSignedRequests(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}
	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}

	fixedNow := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	nonceStore := auth.NewMemoryNonceStore()
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-ID")); got != "" {
			t.Fatalf("X-LabKit-Key-ID = %q, want empty", got)
		}
		if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint")); got != wantFingerprint {
			t.Fatalf("X-LabKit-Key-Fingerprint = %q, want %q", got, wantFingerprint)
		}
		ts, err := time.Parse(time.RFC3339Nano, r.Header.Get("X-LabKit-Timestamp"))
		if err != nil {
			t.Fatalf("parse timestamp: %v", err)
		}
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(r.Header.Get("X-LabKit-Signature")))
		if err != nil {
			t.Fatalf("decode signature: %v", err)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		payload := auth.NewPayload(strings.ToUpper(r.Method)+" "+r.URL.Path, ts, strings.TrimSpace(r.Header.Get("X-LabKit-Nonce")), nil).
			WithContentHash(sha256Hex(body))
		verifier := auth.Verifier{
			Now:     func() time.Time { return fixedNow },
			MaxSkew: 5 * time.Minute,
			Nonces:  nonceStore,
		}
		if err := verifier.Verify(context.Background(), pub, payload, sig); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]any{{
					"id":          11,
					"user_id":     99,
					"public_key":  "ssh-ed25519 AAAA",
					"device_name": "laptop",
					"created_at":  fixedNow.Format(time.RFC3339Nano),
				}},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/keys/11":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return fixedNow },
		Sleep:        func(time.Duration) {},
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	root := NewRootCommand(deps)
	root.SetArgs([]string{"keys"})
	if err := root.Execute(); err != nil {
		t.Fatalf("keys execute error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Bound keys") {
		t.Fatalf("keys output = %q, want structured heading", stdout.String())
	}
	if !strings.Contains(stdout.String(), "laptop") {
		t.Fatalf("keys output = %q, want device name", stdout.String())
	}

	stdout.Reset()
	root = NewRootCommand(deps)
	root.SetArgs([]string{"revoke", "11"})
	if err := root.Execute(); err != nil {
		t.Fatalf("revoke execute error = %v", err)
	}
	if !strings.Contains(stdout.String(), "✓ Revoked  key 11") {
		t.Fatalf("revoke output = %q, want success confirmation", stdout.String())
	}
}

func TestRevokeCommandShowsStructuredServerErrorMessage(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	if err := WritePrivateKey(keyPath, priv); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}
	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}

	fixedNow := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	nonceStore := auth.NewMemoryNonceStore()
	wantFingerprint, err := keycrypto.PublicKeyFingerprint(pub)
	if err != nil {
		t.Fatalf("PublicKeyFingerprint() error = %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-ID")); got != "" {
			t.Fatalf("X-LabKit-Key-ID = %q, want empty", got)
		}
		if got := strings.TrimSpace(r.Header.Get("X-LabKit-Key-Fingerprint")); got != wantFingerprint {
			t.Fatalf("X-LabKit-Key-Fingerprint = %q, want %q", got, wantFingerprint)
		}
		ts, err := time.Parse(time.RFC3339Nano, r.Header.Get("X-LabKit-Timestamp"))
		if err != nil {
			t.Fatalf("parse timestamp: %v", err)
		}
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(r.Header.Get("X-LabKit-Signature")))
		if err != nil {
			t.Fatalf("decode signature: %v", err)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		payload := auth.NewPayload(strings.ToUpper(r.Method)+" "+r.URL.Path, ts, strings.TrimSpace(r.Header.Get("X-LabKit-Nonce")), nil).
			WithContentHash(sha256Hex(body))
		verifier := auth.Verifier{
			Now:     func() time.Time { return fixedNow },
			MaxSkew: 5 * time.Minute,
			Nonces:  nonceStore,
		}
		if err := verifier.Verify(context.Background(), pub, payload, sig); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		if r.Method == http.MethodDelete && r.URL.Path == "/api/keys/11" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":       "key_in_use",
					"message":    "Key has submission history and cannot be revoked",
					"request_id": "req-123",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv("LABKIT_SERVER_URL", srv.URL)

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:    configDir,
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return fixedNow },
		Sleep:        func(time.Duration) {},
		PollInterval: time.Millisecond,
		Out:          &stdout,
		Err:          io.Discard,
	}

	root := NewRootCommand(deps)
	root.SetArgs([]string{"revoke", "11"})
	err = root.Execute()
	if err == nil {
		t.Fatal("revoke execute error = nil, want conflict")
	}
	if !strings.Contains(err.Error(), "Key has submission history and cannot be revoked") {
		t.Fatalf("revoke error = %v, want structured server message", err)
	}
	if strings.Contains(err.Error(), "\"request_id\"") || strings.Contains(err.Error(), "{\"error\"") {
		t.Fatalf("revoke error = %v, want decoded message instead of raw json", err)
	}
}

func writeProjectConfig(t *testing.T, dir, serverURL, labID string) {
	t.Helper()
	configPath := config.ProjectConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(configPath), err)
	}
	content := "server_url = " + strconv.Quote(serverURL) + "\nlab = " + strconv.Quote(labID) + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", configPath, err)
	}
}
