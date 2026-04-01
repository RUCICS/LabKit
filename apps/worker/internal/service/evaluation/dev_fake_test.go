package evaluation

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"labkit.local/packages/go/evaluator"
)

func TestEvaluateDevArtifactReadsEvaluatorResultFromSubmissionArchive(t *testing.T) {
	m := testManifest(t)
	archive, err := archiveFiles(map[string]string{
		"main.c": `{
  "verdict": "scored",
  "scores": {
    "throughput": 88.25,
    "latency": 1.25
  },
  "message": "dev fake evaluation"
}`,
		"README.md": "# ignored by fake evaluator\n",
	})
	if err != nil {
		t.Fatalf("archiveFiles() error = %v", err)
	}

	result, err := EvaluateDevArtifact(context.Background(), m, archive)
	if err != nil {
		t.Fatalf("EvaluateDevArtifact() error = %v", err)
	}
	if result.Verdict != evaluator.VerdictScored {
		t.Fatalf("Verdict = %q, want %q", result.Verdict, evaluator.VerdictScored)
	}
	if got := result.Scores["throughput"]; got != 88.25 {
		t.Fatalf("throughput = %v, want 88.25", got)
	}
	if got := result.Scores["latency"]; got != 1.25 {
		t.Fatalf("latency = %v, want 1.25", got)
	}
}

func TestEvaluateDevArtifactRejectsMissingExpectedSubmissionFile(t *testing.T) {
	m := testManifest(t)
	archive, err := archiveFiles(map[string]string{
		"README.md": "# missing main.c\n",
	})
	if err != nil {
		t.Fatalf("archiveFiles() error = %v", err)
	}

	_, err = EvaluateDevArtifact(context.Background(), m, archive)
	if err == nil {
		t.Fatal("EvaluateDevArtifact() error = nil, want missing file error")
	}
}

func archiveFiles(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
