package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"runtime"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

func TestArchiveBinaryNameMatchesGoreleaserOutput(t *testing.T) {
	// LabKit's release tarballs (per .goreleaser-cli.yml) contain a binary
	// named "labkit-<os>-<arch>". The pinned archiveBinaryName and
	// go-selfupdate's regex anchor `^<cmd>([_-]v?<semver>)?([_-]<os>[_-]<arch>)?(\.exe)?$`
	// must agree on the prefix, or self-update silently breaks for every
	// downstream rename (CoLab's "colab" wrapper is the first example).
	binaryInArchive := "labkit-" + runtime.GOOS + "-" + runtime.GOARCH
	archive := buildTarGz(t, map[string][]byte{
		"README.md":     []byte("# release\n"),
		binaryInArchive: []byte("fake binary payload"),
	})

	reader, err := selfupdate.DecompressCommand(
		bytes.NewReader(archive),
		"labkit_v0.1.6_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz",
		archiveBinaryName,
		runtime.GOOS,
		runtime.GOARCH,
	)
	if err != nil {
		t.Fatalf("DecompressCommand() error = %v; archiveBinaryName=%q must match GoReleaser output", err, archiveBinaryName)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != "fake binary payload" {
		t.Fatalf("decompressed bytes = %q, want %q", string(got), "fake binary payload")
	}
}

func TestApplyUpdateExtractionIgnoresRenamedExe(t *testing.T) {
	// Regression guard for the colab/labkit rename: passing a renamed exe's
	// basename ("colab") as cmd must fail to find the labkit-* asset, while
	// the pinned archiveBinaryName must succeed. If either invariant flips,
	// the matching strategy in go-selfupdate has changed and we should
	// re-evaluate this workaround.
	binaryInArchive := "labkit-" + runtime.GOOS + "-" + runtime.GOARCH
	archive := buildTarGz(t, map[string][]byte{
		binaryInArchive: []byte("payload"),
	})

	if _, err := selfupdate.DecompressCommand(
		bytes.NewReader(archive),
		"labkit_v0.1.6_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz",
		"colab",
		runtime.GOOS,
		runtime.GOARCH,
	); err == nil {
		t.Fatal("expected DecompressCommand to fail when cmd is derived from a renamed binary; if this passes, go-selfupdate's matcher relaxed and the pinning may no longer be needed")
	}

	if _, err := selfupdate.DecompressCommand(
		bytes.NewReader(archive),
		"labkit_v0.1.6_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz",
		archiveBinaryName,
		runtime.GOOS,
		runtime.GOARCH,
	); err != nil {
		t.Fatalf("DecompressCommand with archiveBinaryName failed: %v", err)
	}
}

func buildTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0755,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("Write(%q) error = %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close() error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close() error = %v", err)
	}
	return buf.Bytes()
}

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

