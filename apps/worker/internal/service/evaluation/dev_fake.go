package evaluation

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"labkit.local/packages/go/evaluator"
	"labkit.local/packages/go/manifest"
)

func EvaluateDevArtifact(_ context.Context, m *manifest.Manifest, archive []byte) (evaluator.Result, error) {
	if m == nil {
		return evaluator.Result{}, fmt.Errorf("manifest is required")
	}

	files, err := extractArchiveFiles(archive)
	if err != nil {
		return evaluator.Result{}, err
	}

	for _, name := range m.Submit.Files {
		content, ok := files[name]
		if !ok {
			continue
		}
		var result evaluator.Result
		if err := json.Unmarshal(content, &result); err != nil {
			return evaluator.Result{}, fmt.Errorf("decode fake evaluator result %q: %w", name, err)
		}
		if err := evaluator.ValidateResult(m, result); err != nil {
			return evaluator.Result{}, err
		}
		return result, nil
	}

	return evaluator.Result{}, fmt.Errorf("expected one of %v in artifact archive", m.Submit.Files)
}

func extractArchiveFiles(archive []byte) (map[string][]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := make(map[string][]byte)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read archive entry: %w", err)
		}
		if header == nil || header.FileInfo().IsDir() {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read archive file %q: %w", header.Name, err)
		}
		files[header.Name] = append([]byte(nil), content...)
	}
}
