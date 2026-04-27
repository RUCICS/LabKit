package commands

import "testing"

func TestPrettyVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "dev"},
		{"dev", "dev"},
		{"0.1.0", "v0.1.0"},
		{"v0.1.0", "v0.1.0"},
		{"not-a-version", "not-a-version"},
	}
	for _, tc := range cases {
		if got := prettyVersion(tc.in); got != tc.want {
			t.Fatalf("prettyVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLooksLikeSemver(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"0.0.0", true},
		{"1.2.3", true},
		{"1.2", false},
		{"v1.2.3", false},
		{"1.2.3-rc1", false},
		{"1.2.x", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeSemver(tc.in); got != tc.want {
			t.Fatalf("looksLikeSemver(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

