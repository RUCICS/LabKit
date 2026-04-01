package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	configFileName   = "config.toml"
	keyFileName      = "id_ed25519"
	defaultServerURL = "http://localhost:8080"
	serverURLEnvName = "LABKIT_SERVER_URL"
	localConfigDir   = ".labkit"
)

type Config struct {
	ServerURL string `toml:"server_url"`
	Lab       string `toml:"lab"`
	KeyPath   string `toml:"key_path"`
	KeyID     int64  `toml:"key_id"`
}

type ProjectConfig struct {
	ServerURL string `toml:"server_url"`
	Lab       string `toml:"lab"`
}

type ServerEntry struct {
	KeyPath        string `toml:"key_path,omitempty"`
	KeyFingerprint string `toml:"key_fingerprint,omitempty"`
	Encrypted      bool   `toml:"encrypted,omitempty"`
}

type GlobalConfig struct {
	DefaultServerURL string                 `toml:"default_server_url,omitempty"`
	Servers          map[string]ServerEntry `toml:"servers,omitempty"`
}

func ResolveDir() string {
	if override := strings.TrimSpace(os.Getenv("LABKIT_CONFIG_DIR")); override != "" {
		return filepath.Clean(override)
	}

	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "labkit")
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".", ".labkit")
	}
	return filepath.Join(home, ".config", "labkit")
}

func ConfigPath(dir string) string {
	return filepath.Join(dir, configFileName)
}

func ProjectConfigPath(dir string) string {
	return filepath.Join(dir, localConfigDir, configFileName)
}

func ResolveKeyPath(dir string) string {
	return filepath.Join(dir, keyFileName)
}

func ResolveServerURL() string {
	if override := strings.TrimSpace(os.Getenv(serverURLEnvName)); override != "" {
		return override
	}
	return defaultServerURL
}

func Read(dir string) (Config, error) {
	path := ConfigPath(dir)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{KeyPath: ResolveKeyPath(dir)}, nil
		}
		return Config{}, err
	}
	return readFile(path, ResolveKeyPath(dir))
}

func ReadGlobal(dir string) (GlobalConfig, error) {
	path := ConfigPath(dir)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GlobalConfig{}, nil
		}
		return GlobalConfig{}, err
	}
	return readGlobalFile(path)
}

func FindLocalConfig(startDir string) (ProjectConfig, string, error) {
	current := strings.TrimSpace(startDir)
	if current == "" {
		wd, err := os.Getwd()
		if err != nil {
			return ProjectConfig{}, "", err
		}
		current = wd
	}
	current = filepath.Clean(current)

	for {
		path := ProjectConfigPath(current)
		if _, err := os.Stat(path); err == nil {
			cfg, readErr := readProjectFile(path)
			return cfg, path, readErr
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return ProjectConfig{}, "", nil
		}
		current = parent
	}
}

func Write(dir string, cfg Config) error {
	if cfg.KeyPath == "" {
		cfg.KeyPath = ResolveKeyPath(dir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}

	data := buf.Bytes()
	data = append(data, '\n')
	return os.WriteFile(ConfigPath(dir), data, 0o600)
}

func WriteGlobal(dir string, cfg GlobalConfig) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	normalized, err := normalizeGlobalConfig(cfg)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(normalized); err != nil {
		return err
	}
	data := buf.Bytes()
	data = append(data, '\n')
	return os.WriteFile(ConfigPath(dir), data, 0o600)
}

func NormalizeServerOrigin(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("server URL is required")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if !u.IsAbs() || strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
		return "", fmt.Errorf("server URL must be absolute")
	}
	if strings.TrimSpace(u.Path) != "" && strings.TrimSpace(u.Path) != "/" {
		return "", fmt.Errorf("server URL must not include a path")
	}
	if strings.TrimSpace(u.RawQuery) != "" || strings.TrimSpace(u.Fragment) != "" || u.User != nil {
		return "", fmt.Errorf("server URL must be an origin")
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == defaultPortForScheme(scheme) {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host, nil
}

func (c *GlobalConfig) LookupServerEntry(rawServerURL string) (ServerEntry, bool, error) {
	if c == nil {
		return ServerEntry{}, false, nil
	}
	origin, err := NormalizeServerOrigin(rawServerURL)
	if err != nil {
		return ServerEntry{}, false, err
	}
	entry, ok := c.Servers[origin]
	return entry, ok, nil
}

func (c *GlobalConfig) SetServerEntry(rawServerURL string, entry ServerEntry) error {
	if c == nil {
		return fmt.Errorf("global config is nil")
	}
	origin, err := NormalizeServerOrigin(rawServerURL)
	if err != nil {
		return err
	}
	if c.Servers == nil {
		c.Servers = make(map[string]ServerEntry)
	}
	c.Servers[origin] = entry
	return nil
}

func readFile(path, defaultKeyPath string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&cfg)
	if err != nil {
		return Config{}, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		key := undecoded[0].String()
		if isGlobalKeyringOnlyKey(key) {
			return Config{}, fmt.Errorf("use ReadGlobal for keyring config: unexpected key %q", key)
		}
		return Config{}, fmt.Errorf("unexpected config key %q", key)
	}
	if cfg.KeyPath == "" && strings.TrimSpace(defaultKeyPath) != "" {
		cfg.KeyPath = defaultKeyPath
	}
	return cfg, nil
}

func readProjectFile(path string) (ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectConfig{}, err
	}

	var cfg ProjectConfig
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&cfg)
	if err != nil {
		return ProjectConfig{}, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return ProjectConfig{}, fmt.Errorf("unexpected config key %q", undecoded[0].String())
	}
	return cfg, nil
}

func readGlobalFile(path string) (GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GlobalConfig{}, err
	}

	var raw struct {
		DefaultServerURL string                 `toml:"default_server_url"`
		Servers          map[string]ServerEntry `toml:"servers"`
	}
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&raw)
	if err != nil {
		return GlobalConfig{}, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return GlobalConfig{}, fmt.Errorf("unexpected config key %q", undecoded[0].String())
	}
	cfg := GlobalConfig{
		DefaultServerURL: strings.TrimSpace(raw.DefaultServerURL),
		Servers:          make(map[string]ServerEntry, len(raw.Servers)),
	}
	for origin, entry := range raw.Servers {
		cfg.Servers[origin] = entry
	}
	return normalizeGlobalConfig(cfg)
}

func normalizeGlobalConfig(cfg GlobalConfig) (GlobalConfig, error) {
	out := GlobalConfig{
		DefaultServerURL: strings.TrimSpace(cfg.DefaultServerURL),
		Servers:          make(map[string]ServerEntry, len(cfg.Servers)),
	}
	for origin, entry := range cfg.Servers {
		normalizedOrigin, err := NormalizeServerOrigin(origin)
		if err != nil {
			return GlobalConfig{}, err
		}
		out.Servers[normalizedOrigin] = entry
	}
	if out.DefaultServerURL != "" {
		origin, err := NormalizeServerOrigin(out.DefaultServerURL)
		if err != nil {
			return GlobalConfig{}, err
		}
		out.DefaultServerURL = origin
	}
	if out.DefaultServerURL != "" {
		if _, ok := out.Servers[out.DefaultServerURL]; !ok {
			return GlobalConfig{}, fmt.Errorf("default_server_url %q has no matching server entry", out.DefaultServerURL)
		}
	}
	return out, nil
}

func defaultPortForScheme(scheme string) string {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func isGlobalKeyringOnlyKey(key string) bool {
	switch key {
	case "default_server_url", "servers", "key_fingerprint", "encrypted":
		return true
	default:
		return false
	}
}
