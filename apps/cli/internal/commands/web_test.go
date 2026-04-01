package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"labkit.local/apps/cli/internal/config"
	keycrypto "labkit.local/apps/cli/internal/crypto"
)

func TestWebCommandRequestsSessionTicketOpensBrowserAndUsesProvidedPath(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pair, err := keycrypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	if err := WritePrivateKey(keyPath, pair.Private); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}
	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}

	fixedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	var browserURL string
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/web/session-ticket" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if err := verifySignedRequest(t, r, "/api/web/session-ticket", body, pair.Public); err != nil {
			t.Fatalf("verifySignedRequest() error = %v", err)
		}
		var req struct {
			RedirectPath string `json:"redirect_path"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if req.RedirectPath != "/labs/local-smoke/board" {
			t.Fatalf("redirect_path = %q, want %q", req.RedirectPath, "/labs/local-smoke/board")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"redirect_url": "/auth/session#ticket=ticket-123",
		})
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	deps := &Dependencies{
		ConfigDir:         configDir,
		ServerURLOverride: srv.URL,
		HTTPClient:        srv.Client(),
		Now:               func() time.Time { return fixedNow },
		OpenBrowser: func(target string) error {
			browserURL = target
			calls++
			return nil
		},
		Out: &stdout,
		Err: io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"web", "/labs/local-smoke/board"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if calls != 1 {
		t.Fatalf("browser open calls = %d, want 1", calls)
	}
	wantURL := srv.URL + "/auth/session#ticket=ticket-123"
	if browserURL != wantURL {
		t.Fatalf("browserURL = %q, want %q", browserURL, wantURL)
	}
	if strings.Contains(stdout.String(), wantURL) || strings.Contains(stdout.String(), "ticket=") {
		t.Fatalf("stdout = %q, want no ticket URL leakage", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Opened browser session.") {
		t.Fatalf("stdout = %q, want success confirmation", stdout.String())
	}
}

func TestWebCommandPrintsFallbackURLWhenBrowserOpenFails(t *testing.T) {
	configDir := t.TempDir()
	keyPath := filepath.Join(configDir, "id_ed25519")
	pair, err := keycrypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}
	if err := WritePrivateKey(keyPath, pair.Private); err != nil {
		t.Fatalf("WritePrivateKey() error = %v", err)
	}
	if err := config.Write(configDir, config.Config{
		ServerURL: "",
		KeyPath:   keyPath,
		KeyID:     11,
	}); err != nil {
		t.Fatalf("config.Write() error = %v", err)
	}

	fixedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	var browserURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/web/session-ticket" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if err := verifySignedRequest(t, r, "/api/web/session-ticket", body, pair.Public); err != nil {
			t.Fatalf("verifySignedRequest() error = %v", err)
		}
		var req struct {
			RedirectPath string `json:"redirect_path"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if req.RedirectPath != "/profile" {
			t.Fatalf("redirect_path = %q, want %q", req.RedirectPath, "/profile")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"redirect_url": "/auth/session#ticket=ticket-456",
		})
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	openErr := errors.New("browser unavailable")
	deps := &Dependencies{
		ConfigDir:         configDir,
		ServerURLOverride: srv.URL,
		HTTPClient:        srv.Client(),
		Now:               func() time.Time { return fixedNow },
		OpenBrowser: func(target string) error {
			browserURL = target
			return openErr
		},
		Out: &stdout,
		Err: io.Discard,
	}

	cmd := NewRootCommand(deps)
	cmd.SetArgs([]string{"web"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	wantURL := srv.URL + "/auth/session#ticket=ticket-456"
	if browserURL != wantURL {
		t.Fatalf("browserURL = %q, want %q", browserURL, wantURL)
	}
	if !strings.Contains(stdout.String(), wantURL) {
		t.Fatalf("stdout = %q, want fallback url", stdout.String())
	}
}

func TestWebCommandRejectsNonSameOriginRedirectURLs(t *testing.T) {
	tests := []struct {
		name         string
		redirectURL  string
		wantFragment string
	}{
		{
			name:         "external absolute url",
			redirectURL:  "https://evil.example/auth/session#ticket=ticket-999",
			wantFragment: "same origin",
		},
		{
			name:         "unexpected scheme",
			redirectURL:  "javascript:alert(1)",
			wantFragment: "scheme",
		},
		{
			name:         "empty url",
			redirectURL:  "",
			wantFragment: "redirect_url",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configDir := t.TempDir()
			keyPath := filepath.Join(configDir, "id_ed25519")
			pair, err := keycrypto.GenerateKeyPair()
			if err != nil {
				t.Fatalf("GenerateKeyPair() error = %v", err)
			}
			if err := WritePrivateKey(keyPath, pair.Private); err != nil {
				t.Fatalf("WritePrivateKey() error = %v", err)
			}
			if err := config.Write(configDir, config.Config{
				ServerURL: "",
				KeyPath:   keyPath,
				KeyID:     11,
			}); err != nil {
				t.Fatalf("config.Write() error = %v", err)
			}

			fixedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
			var browserCalls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/web/session-ticket" {
					http.NotFound(w, r)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if err := verifySignedRequest(t, r, "/api/web/session-ticket", body, pair.Public); err != nil {
					t.Fatalf("verifySignedRequest() error = %v", err)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"redirect_url": tc.redirectURL,
				})
			}))
			defer srv.Close()

			var stdout bytes.Buffer
			deps := &Dependencies{
				ConfigDir:         configDir,
				ServerURLOverride: srv.URL,
				HTTPClient:        srv.Client(),
				Now:               func() time.Time { return fixedNow },
				OpenBrowser: func(target string) error {
					browserCalls++
					return nil
				},
				Out: &stdout,
				Err: io.Discard,
			}

			cmd := NewRootCommand(deps)
			cmd.SetArgs([]string{"web"})
			execErr := cmd.Execute()
			if execErr == nil {
				t.Fatal("Execute() error = nil, want redirect_url validation failure")
			}
			if !strings.Contains(strings.ToLower(execErr.Error()), strings.ToLower(tc.wantFragment)) {
				t.Fatalf("Execute() error = %v, want fragment %q", execErr, tc.wantFragment)
			}
			if browserCalls != 0 {
				t.Fatalf("browser open calls = %d, want 0", browserCalls)
			}
		})
	}
}
