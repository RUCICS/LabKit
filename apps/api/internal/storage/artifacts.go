package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type ArtifactFile struct {
	Name    string
	Content []byte
}

type LocalArtifactStore struct {
	root string
}

func NewLocalArtifactStore(root string) *LocalArtifactStore {
	return &LocalArtifactStore{root: root}
}

func (s *LocalArtifactStore) Save(_ context.Context, key string, archive []byte) error {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	if err := os.WriteFile(path, archive, 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}
	return nil
}

func (s *LocalArtifactStore) Delete(_ context.Context, key string) error {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete artifact: %w", err)
	}
	return nil
}

func Archive(files []ArtifactFile) ([]byte, string, error) {
	sorted := append([]ArtifactFile(nil), files...)
	slices.SortFunc(sorted, func(a, b ArtifactFile) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		default:
			return 0
		}
	})

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for _, file := range sorted {
		header := &tar.Header{
			Name: file.Name,
			Mode: 0o644,
			Size: int64(len(file.Content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, "", err
		}
		if _, err := tw.Write(file.Content); err != nil {
			return nil, "", err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gzw.Close(); err != nil {
		return nil, "", err
	}

	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	return append([]byte(nil), archive...), hex.EncodeToString(sum[:]), nil
}
