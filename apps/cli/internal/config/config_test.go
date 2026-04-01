package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDir(t *testing.T) {
	t.Setenv("LABKIT_CONFIG_DIR", "")

	t.Run("explicit override", func(t *testing.T) {
		want := filepath.Join(t.TempDir(), "custom")
		t.Setenv("LABKIT_CONFIG_DIR", want)
		got := ResolveDir()
		if got != want {
			t.Fatalf("ResolveDir() = %q, want %q", got, want)
		}
	})

	t.Run("xdg config home", func(t *testing.T) {
		t.Setenv("LABKIT_CONFIG_DIR", "")
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		got := ResolveDir()
		if want := filepath.Join(xdg, "labkit"); got != want {
			t.Fatalf("ResolveDir() = %q, want %q", got, want)
		}
	})
}

func TestResolveKeyPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "labkit")
	got := ResolveKeyPath(dir)
	if want := filepath.Join(dir, "id_ed25519"); got != want {
		t.Fatalf("ResolveKeyPath() = %q, want %q", got, want)
	}
}

func TestResolveServerURL(t *testing.T) {
	t.Setenv("LABKIT_SERVER_URL", "")
	if got := ResolveServerURL(); got != "http://localhost:8080" {
		t.Fatalf("ResolveServerURL() = %q, want %q", got, "http://localhost:8080")
	}

	t.Setenv("LABKIT_SERVER_URL", "https://labkit.example.edu")
	if got := ResolveServerURL(); got != "https://labkit.example.edu" {
		t.Fatalf("ResolveServerURL() = %q, want %q", got, "https://labkit.example.edu")
	}
}

func TestReadWriteGlobalConfigKeyring(t *testing.T) {
	dir := t.TempDir()
	want := GlobalConfig{
		DefaultServerURL: "https://labkit.example.edu",
		Servers: map[string]ServerEntry{
			"https://labkit.example.edu": {
				KeyPath:        filepath.Join(dir, "id_ed25519"),
				KeyFingerprint: "SHA256:abc123",
				Encrypted:      true,
			},
		},
	}

	if err := WriteGlobal(dir, want); err != nil {
		t.Fatalf("WriteGlobal() error = %v", err)
	}

	got, err := ReadGlobal(dir)
	if err != nil {
		t.Fatalf("ReadGlobal() error = %v", err)
	}
	if got.DefaultServerURL != want.DefaultServerURL {
		t.Fatalf("ReadGlobal().DefaultServerURL = %q, want %q", got.DefaultServerURL, want.DefaultServerURL)
	}
	gotEntry, ok, err := got.LookupServerEntry("https://labkit.example.edu")
	if err != nil {
		t.Fatalf("LookupServerEntry() error = %v", err)
	}
	if !ok {
		t.Fatal("LookupServerEntry() missing expected server entry")
	}
	wantEntry := want.Servers["https://labkit.example.edu"]
	if gotEntry != wantEntry {
		t.Fatalf("LookupServerEntry() = %#v, want %#v", gotEntry, wantEntry)
	}

	data, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("config file is empty")
	}
	text := string(data)
	for _, want := range []string{
		`default_server_url = "https://labkit.example.edu"`,
		`key_path = "` + filepath.Join(dir, "id_ed25519") + `"`,
		`key_fingerprint = "SHA256:abc123"`,
		`encrypted = true`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config text = %q, want %q", text, want)
		}
	}
}

func TestReadGlobalRejectsLegacyFlatKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(ConfigPath(dir), []byte("server_url = \"https://labkit.example.edu\"\nkey_path = \"/tmp/id_ed25519\"\nkey_id = 11\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := ReadGlobal(dir); err == nil {
		t.Fatal("ReadGlobal() error = nil, want failure for legacy flat config")
	}
}

func TestReadGlobalRejectsMissingDefaultServerEntry(t *testing.T) {
	dir := t.TempDir()
	content := "default_server_url = \"https://labkit.example.edu\"\n[servers.\"https://other.example.edu\"]\nkey_path = \"/tmp/id_ed25519\"\n"
	if err := os.WriteFile(ConfigPath(dir), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := ReadGlobal(dir); err == nil || !strings.Contains(err.Error(), "default_server_url") {
		t.Fatalf("ReadGlobal() error = %v, want missing default server entry failure", err)
	}
}

func TestNormalizeServerOrigin(t *testing.T) {
	got, err := NormalizeServerOrigin("https://LabKit.Example.edu:8083/")
	if err != nil {
		t.Fatalf("NormalizeServerOrigin() error = %v", err)
	}
	if want := "https://labkit.example.edu:8083"; got != want {
		t.Fatalf("NormalizeServerOrigin() = %q, want %q", got, want)
	}
}

func TestNormalizeServerOriginCanonicalizesDefaultPorts(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https without port", raw: "https://LabKit.Example.edu", want: "https://labkit.example.edu"},
		{name: "https explicit default port", raw: "https://labkit.example.edu:443", want: "https://labkit.example.edu"},
		{name: "http without port", raw: "http://LabKit.Example.edu", want: "http://labkit.example.edu"},
		{name: "http explicit default port", raw: "http://labkit.example.edu:80", want: "http://labkit.example.edu"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeServerOrigin(tc.raw)
			if err != nil {
				t.Fatalf("NormalizeServerOrigin() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeServerOrigin() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadRejectsGlobalKeyringOnlyFields(t *testing.T) {
	dir := t.TempDir()
	content := "default_server_url = \"https://labkit.example.edu\"\n[servers.\"https://labkit.example.edu\"]\nkey_path = \"/tmp/id_ed25519\"\n"
	if err := os.WriteFile(ConfigPath(dir), []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Read(dir); err == nil || !strings.Contains(err.Error(), "use ReadGlobal for keyring config") {
		t.Fatalf("Read() error = %v, want keyring-specific failure", err)
	}
}

func TestFindLocalConfigReadsProjectConfigWithoutIdentityFields(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "student")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".labkit"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.labkit) error = %v", err)
	}
	localPath := ProjectConfigPath(root)
	if err := os.WriteFile(localPath, []byte("server_url = \"https://labkit.example.edu\"\nlab = \"sorting\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, path, err := FindLocalConfig(nested)
	if err != nil {
		t.Fatalf("FindLocalConfig() error = %v", err)
	}
	if path != localPath {
		t.Fatalf("FindLocalConfig() path = %q, want %q", path, localPath)
	}
	if got.ServerURL != "https://labkit.example.edu" {
		t.Fatalf("FindLocalConfig().ServerURL = %q, want %q", got.ServerURL, "https://labkit.example.edu")
	}
	if got.Lab != "sorting" {
		t.Fatalf("FindLocalConfig().Lab = %q, want %q", got.Lab, "sorting")
	}
}

func TestFindLocalConfigRejectsIdentityFields(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "student")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".labkit"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.labkit) error = %v", err)
	}
	localPath := ProjectConfigPath(root)
	if err := os.WriteFile(localPath, []byte("server_url = \"https://labkit.example.edu\"\nlab = \"sorting\"\nkey_path = \"/tmp/id_ed25519\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, _, err := FindLocalConfig(nested); err == nil {
		t.Fatal("FindLocalConfig() error = nil, want failure for identity fields in project config")
	}
}

func TestGlobalConfigSetAndLookupServerEntry(t *testing.T) {
	cfg := GlobalConfig{}
	if err := cfg.SetServerEntry("https://labkit.example.edu", ServerEntry{
		KeyPath:        "/tmp/id_ed25519",
		KeyFingerprint: "SHA256:xyz",
		Encrypted:      false,
	}); err != nil {
		t.Fatalf("SetServerEntry() error = %v", err)
	}

	entry, ok, err := cfg.LookupServerEntry("https://labkit.example.edu/")
	if err != nil {
		t.Fatalf("LookupServerEntry() error = %v", err)
	}
	if !ok {
		t.Fatal("LookupServerEntry() missing expected server entry")
	}
	if entry.KeyPath != "/tmp/id_ed25519" {
		t.Fatalf("entry.KeyPath = %q, want %q", entry.KeyPath, "/tmp/id_ed25519")
	}
	if entry.KeyFingerprint != "SHA256:xyz" {
		t.Fatalf("entry.KeyFingerprint = %q, want %q", entry.KeyFingerprint, "SHA256:xyz")
	}
	if entry.Encrypted {
		t.Fatal("entry.Encrypted = true, want false")
	}
}

func TestFindLocalConfigClimbsParents(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "labs", "sorting", "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".labkit"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.labkit) error = %v", err)
	}
	localPath := ProjectConfigPath(root)
	if err := os.WriteFile(localPath, []byte("server_url = \"http://localhost:8083\"\nlab = \"local-smoke\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", localPath, err)
	}

	got, path, err := FindLocalConfig(nested)
	if err != nil {
		t.Fatalf("FindLocalConfig() error = %v", err)
	}
	if path != localPath {
		t.Fatalf("FindLocalConfig() path = %q, want %q", path, localPath)
	}
	if got.ServerURL != "http://localhost:8083" {
		t.Fatalf("FindLocalConfig().ServerURL = %q, want %q", got.ServerURL, "http://localhost:8083")
	}
	if got.Lab != "local-smoke" {
		t.Fatalf("FindLocalConfig().Lab = %q, want %q", got.Lab, "local-smoke")
	}
}

func TestFindLocalConfigReturnsEmptyWhenMissing(t *testing.T) {
	start := filepath.Join(t.TempDir(), "labs", "sorting")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	got, path, err := FindLocalConfig(start)
	if err != nil {
		t.Fatalf("FindLocalConfig() error = %v", err)
	}
	if path != "" {
		t.Fatalf("FindLocalConfig() path = %q, want empty", path)
	}
	if got != (ProjectConfig{}) {
		t.Fatalf("FindLocalConfig() config = %#v, want zero value", got)
	}
}

func TestFindLocalConfigUsesWorkingDirectoryWhenStartIsEmpty(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "pkg", "student")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".labkit"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.labkit) error = %v", err)
	}
	localPath := ProjectConfigPath(root)
	if err := os.WriteFile(localPath, []byte("server_url = \"http://localhost:8083\"\nlab = \"local-smoke\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", localPath, err)
	}
	t.Chdir(workDir)

	got, path, err := FindLocalConfig("")
	if err != nil {
		t.Fatalf("FindLocalConfig(\"\") error = %v", err)
	}
	if path != localPath {
		t.Fatalf("FindLocalConfig(\"\") path = %q, want %q", path, localPath)
	}
	if got.Lab != "local-smoke" {
		t.Fatalf("FindLocalConfig(\"\").Lab = %q, want %q", got.Lab, "local-smoke")
	}
}
