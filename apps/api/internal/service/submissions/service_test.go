package submissions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"labkit.local/apps/api/internal/storage"
	"labkit.local/packages/go/auth"
	"labkit.local/packages/go/db/sqlc"
	"labkit.local/packages/go/manifest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/ssh"
)

func TestIntakeRejectsMissingFiles(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-1",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-1", []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
		}),
	})
	if !errors.Is(err, ErrMissingFiles) {
		t.Fatalf("Intake() error = %v, want ErrMissingFiles", err)
	}
	if repo.beginCalls != 0 {
		t.Fatalf("begin calls = %d, want 0", repo.beginCalls)
	}
}

func TestIntakeRejectsInvalidSignature(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-2",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		},
		Signature: []byte("bad-signature"),
	})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("Intake() error = %v, want ErrInvalidSignature", err)
	}
	if repo.artifacts.saveCalls != 0 {
		t.Fatalf("artifact save calls = %d, want 0", repo.artifacts.saveCalls)
	}
}

func TestIntakeRejectsClosedLab(t *testing.T) {
	repo, signer := newTestRepo(t)
	repo.labManifest = mustStoredManifest(t, testManifest(`
visible = 2026-03-01T00:00:00Z
open = 2026-03-01T00:00:00Z
close = 2026-03-30T00:00:00Z
`))
	svc := newTestService(repo)

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-3",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-3", []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		}),
	})
	if !errors.Is(err, ErrLabClosed) {
		t.Fatalf("Intake() error = %v, want ErrLabClosed", err)
	}
	if repo.artifacts.saveCalls != 0 {
		t.Fatalf("artifact save calls = %d, want 0", repo.artifacts.saveCalls)
	}
}

func TestIntakeAcceptsSubmissionWhenEachFileFitsManifestMaxSize(t *testing.T) {
	repo, signer := newTestRepo(t)
	repo.labManifest = mustStoredManifest(t, testManifestWithMaxSize("10B", `
visible = 2026-03-01T00:00:00Z
open = 2026-03-01T00:00:00Z
close = 2026-04-30T00:00:00Z
`))
	svc := newTestService(repo)

	result, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-per-file-ok",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("1234567890")},
			{Name: "README.md", Content: []byte("12345")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-per-file-ok", []UploadFile{
			{Name: "main.c", Content: []byte("1234567890")},
			{Name: "README.md", Content: []byte("12345")},
		}),
	})
	if err != nil {
		t.Fatalf("Intake() error = %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("status = %q, want %q", result.Status, "queued")
	}
	if repo.artifacts.saveCalls != 1 {
		t.Fatalf("artifact save calls = %d, want 1", repo.artifacts.saveCalls)
	}
}

func TestIntakeRejectsFileExceedingManifestMaxSize(t *testing.T) {
	repo, signer := newTestRepo(t)
	repo.labManifest = mustStoredManifest(t, testManifestWithMaxSize("10B", `
visible = 2026-03-01T00:00:00Z
open = 2026-03-01T00:00:00Z
close = 2026-04-30T00:00:00Z
`))
	svc := newTestService(repo)

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-oversize",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("12345678901")},
			{Name: "README.md", Content: []byte("12345")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-oversize", []UploadFile{
			{Name: "main.c", Content: []byte("12345678901")},
			{Name: "README.md", Content: []byte("12345")},
		}),
	})
	if !errors.Is(err, ErrSubmissionTooLarge) {
		t.Fatalf("Intake() error = %v, want ErrSubmissionTooLarge", err)
	}
	if repo.artifacts.saveCalls != 0 {
		t.Fatalf("artifact save calls = %d, want 0", repo.artifacts.saveCalls)
	}
	if repo.beginCalls != 0 {
		t.Fatalf("begin calls = %d, want 0", repo.beginCalls)
	}
}

func TestIntakeAcceptsValidSubmissionAndPersistsArtifact(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)

	result, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-4",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-4", []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		}),
	})
	if err != nil {
		t.Fatalf("Intake() error = %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("status = %q, want %q", result.Status, "queued")
	}
	if result.ArtifactKey == "" {
		t.Fatalf("artifact key was empty")
	}
	if result.ContentHash == "" {
		t.Fatalf("content hash was empty")
	}
	if repo.artifacts.saveCalls != 1 {
		t.Fatalf("artifact save calls = %d, want 1", repo.artifacts.saveCalls)
	}
	if repo.lastCommittedSubmission.ArtifactKey != result.ArtifactKey {
		t.Fatalf("stored artifact key = %q, want %q", repo.lastCommittedSubmission.ArtifactKey, result.ArtifactKey)
	}
	if repo.lastCommittedJob.SubmissionID != result.ID {
		t.Fatalf("job submission id = %v, want %v", repo.lastCommittedJob.SubmissionID, result.ID)
	}
	if result.Quota == nil {
		t.Fatal("Quota = nil, want summary")
	}
	if result.Quota.Daily != 3 || result.Quota.Used != 1 || result.Quota.Left != 2 {
		t.Fatalf("quota summary = %#v, want daily=3 used=1 left=2", result.Quota)
	}

	files, err := untarGzip(repo.artifacts.lastArchive)
	if err != nil {
		t.Fatalf("untar artifact: %v", err)
	}
	if string(files["main.c"]) != "int main(void) { return 0; }\n" {
		t.Fatalf("artifact main.c = %q", string(files["main.c"]))
	}
	if string(files["README.md"]) != "# sorting\n" {
		t.Fatalf("artifact README.md = %q", string(files["README.md"]))
	}
}

func TestGetSubmitPrecheckReturnsQuotaSummaryAndLatestSubmission(t *testing.T) {
	repo, _ := newTestRepo(t)
	repo.quotaUsage = 1
	repo.latestSubmission = sqlc.Submissions{
		ID:          uuid.MustParse("99999999-9999-7999-8999-999999999999"),
		UserID:      7,
		LabID:       "sorting",
		ContentHash: "hash-latest",
		CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC), Valid: true},
	}
	svc := newTestService(repo)
	svc.SetQuotaLocation(mustLoadLocation(t, "Asia/Shanghai"))

	got, err := svc.GetSubmitPrecheck(context.Background(), 7, "sorting")
	if err != nil {
		t.Fatalf("GetSubmitPrecheck() error = %v", err)
	}
	if got.Quota == nil {
		t.Fatal("Quota = nil, want summary")
	}
	if got.Quota.Daily != 3 || got.Quota.Used != 1 || got.Quota.Left != 2 {
		t.Fatalf("quota summary = %#v, want daily=3 used=1 left=2", got.Quota)
	}
	if got.LatestSubmission == nil {
		t.Fatal("LatestSubmission = nil, want metadata")
	}
	if got.LatestSubmission.ContentHash != "hash-latest" {
		t.Fatalf("latest content hash = %q, want %q", got.LatestSubmission.ContentHash, "hash-latest")
	}
	if !got.LatestSubmission.CreatedAt.Equal(time.Date(2026, 3, 31, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("latest created_at = %v", got.LatestSubmission.CreatedAt)
	}
}

func TestGetSubmitPrecheckReturnsLabNotFound(t *testing.T) {
	repo, _ := newTestRepo(t)
	svc := newTestService(repo)

	_, err := svc.GetSubmitPrecheck(context.Background(), 7, "missing")
	if !errors.Is(err, ErrLabNotFound) {
		t.Fatalf("GetSubmitPrecheck() error = %v, want ErrLabNotFound", err)
	}
}

func TestIntakeDailyQuotaRejectsWhenQuotaDayIsExhausted(t *testing.T) {
	repo, signer := newTestRepo(t)
	repo.now = time.Date(2026, 3, 31, 16, 30, 0, 0, time.UTC)
	repo.quotaUsage = 3
	svc := newTestService(repo)
	svc.SetQuotaLocation(mustLoadLocation(t, "Asia/Shanghai"))

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-quota-hit",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-quota-hit", []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		}),
	})
	if err == nil {
		t.Fatal("Intake() error = nil, want daily quota error")
	}
	if !strings.Contains(err.Error(), "Daily submission limit reached") {
		t.Fatalf("Intake() error = %v, want daily quota message", err)
	}
	if repo.beginCalls != 1 {
		t.Fatalf("begin calls = %d, want 1", repo.beginCalls)
	}
	if repo.artifacts.saveCalls != 0 {
		t.Fatalf("artifact save calls = %d, want 0", repo.artifacts.saveCalls)
	}
	if repo.lastTx == nil {
		t.Fatal("transaction was not started")
	}
	if repo.lastTx.quotaLockCalls != 1 {
		t.Fatalf("quota lock calls = %d, want 1", repo.lastTx.quotaLockCalls)
	}
	if repo.lastTx.quotaCountCalls != 1 {
		t.Fatalf("quota count calls = %d, want 1", repo.lastTx.quotaCountCalls)
	}
	if repo.lastTx.reserveNonceCalls != 0 {
		t.Fatalf("reserve nonce calls = %d, want 0", repo.lastTx.reserveNonceCalls)
	}
	if _, ok := repo.reservedNonces["nonce-quota-hit"]; ok {
		t.Fatal("quota rejection unexpectedly reserved nonce")
	}

	wantStart := time.Date(2026, 4, 1, 0, 0, 0, 0, mustLoadLocation(t, "Asia/Shanghai"))
	wantEnd := wantStart.AddDate(0, 0, 1)
	if !repo.lastQuotaWindowStart.Equal(wantStart) {
		t.Fatalf("quota window start = %v, want %v", repo.lastQuotaWindowStart, wantStart)
	}
	if !repo.lastQuotaWindowEnd.Equal(wantEnd) {
		t.Fatalf("quota window end = %v, want %v", repo.lastQuotaWindowEnd, wantEnd)
	}

	repo.quotaUsage = 0
	_, err = svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-quota-hit",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-quota-hit", []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		}),
	})
	if err != nil {
		t.Fatalf("retry Intake() error = %v, want success after quota frees up", err)
	}
}

func TestIntakeCreatesSubmissionAndEvaluationJobAtomically(t *testing.T) {
	repo, signer := newTestRepo(t)
	repo.failCreateJob = errors.New("queue insert failed")
	svc := newTestService(repo)

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-5",
		Files: []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		},
		Signature: signer.mustSign(t, "sorting", repo.now, "nonce-5", []UploadFile{
			{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
			{Name: "README.md", Content: []byte("# sorting\n")},
		}),
	})
	if err == nil {
		t.Fatalf("Intake() error = nil, want error")
	}
	if repo.lastTx == nil {
		t.Fatalf("transaction was not started")
	}
	if repo.lastTx.commitCalls != 0 {
		t.Fatalf("commit calls = %d, want 0", repo.lastTx.commitCalls)
	}
	if repo.lastTx.rollbackCalls != 1 {
		t.Fatalf("rollback calls = %d, want 1", repo.lastTx.rollbackCalls)
	}
	if repo.lastCommittedSubmission.ID != uuid.Nil {
		t.Fatalf("submission committed unexpectedly: %v", repo.lastCommittedSubmission.ID)
	}
	if repo.lastCommittedJob.ID != uuid.Nil {
		t.Fatalf("job committed unexpectedly: %v", repo.lastCommittedJob.ID)
	}
	if repo.artifacts.deleteCalls != 1 {
		t.Fatalf("artifact delete calls = %d, want 1", repo.artifacts.deleteCalls)
	}
}

func TestIntakeRejectsTamperedContentWhenSignatureHashDoesNotMatch(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)

	signedFiles := []UploadFile{
		{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
		{Name: "README.md", Content: []byte("# sorting\n")},
	}
	submittedFiles := []UploadFile{
		{Name: "main.c", Content: []byte("int main(void) { return 1; }\n")},
		{Name: "README.md", Content: []byte("# sorting\n")},
	}

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-tampered",
		Files:          submittedFiles,
		Signature:      signer.mustSign(t, "sorting", repo.now, "nonce-tampered", signedFiles),
	})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("Intake() error = %v, want ErrInvalidSignature", err)
	}
	if repo.beginCalls != 0 {
		t.Fatalf("begin calls = %d, want 0", repo.beginCalls)
	}
}

func TestIntakeRejectsReplayedNonceAcrossServiceInstances(t *testing.T) {
	repo, signer := newTestRepo(t)
	files := []UploadFile{
		{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
		{Name: "README.md", Content: []byte("# sorting\n")},
	}
	signature := signer.mustSign(t, "sorting", repo.now, "nonce-replay", files)

	first := newTestService(repo)
	if _, err := first.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-replay",
		Files:          files,
		Signature:      signature,
	}); err != nil {
		t.Fatalf("first Intake() error = %v", err)
	}

	second := newTestService(repo)
	_, err := second.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-replay",
		Files:          files,
		Signature:      signature,
	})
	if !errors.Is(err, auth.ErrNonceReplayed) {
		t.Fatalf("second Intake() error = %v, want auth.ErrNonceReplayed", err)
	}
}

func TestIntakeRejectsUndeclaredFiles(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)
	files := []UploadFile{
		{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
		{Name: "README.md", Content: []byte("# sorting\n")},
		{Name: "notes.txt", Content: []byte("extra\n")},
	}

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-extra-file",
		Files:          files,
		Signature:      signer.mustSign(t, "sorting", repo.now, "nonce-extra-file", files),
	})
	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("Intake() error = %v, want ErrInvalidSubmission", err)
	}
}

func TestIntakeRejectsDuplicateFilenames(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)
	files := []UploadFile{
		{Name: "main.c", Content: []byte("int main(void) { return 0; }\n")},
		{Name: "README.md", Content: []byte("# sorting\n")},
		{Name: "README.md", Content: []byte("# duplicate\n")},
	}

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-duplicate-file",
		Files:          files,
		Signature:      signer.mustSign(t, "sorting", repo.now, "nonce-duplicate-file", files),
	})
	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("Intake() error = %v, want ErrInvalidSubmission", err)
	}
}

func TestIntakeRejectsFilenamesWithLeadingOrTrailingWhitespace(t *testing.T) {
	repo, signer := newTestRepo(t)
	svc := newTestService(repo)
	files := []UploadFile{
		{Name: " main.c", Content: []byte("int main(void) { return 0; }\n")},
		{Name: "README.md", Content: []byte("# sorting\n")},
	}

	_, err := svc.Intake(context.Background(), SubmitInput{
		LabID:          "sorting",
		KeyFingerprint: signer.fingerprint,
		Timestamp:      repo.now,
		Nonce:          "nonce-whitespace-file",
		Files:          files,
		Signature:      signer.mustSign(t, "sorting", repo.now, "nonce-whitespace-file", files),
	})
	if !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("Intake() error = %v, want ErrInvalidSubmission", err)
	}
	if repo.artifacts.saveCalls != 0 {
		t.Fatalf("artifact save calls = %d, want 0", repo.artifacts.saveCalls)
	}
}

func newTestService(repo *fakeRepository) *Service {
	svc := NewService(repo, repo.artifacts)
	svc.now = func() time.Time { return repo.now }
	return svc
}

type testSigner struct {
	privateKey  ed25519.PrivateKey
	fingerprint string
}

func (s testSigner) mustSign(t *testing.T, labID string, ts time.Time, nonce string, files []UploadFile) []byte {
	t.Helper()
	contentHash := mustContentHash(t, files)
	payload := auth.NewPayload(labID, ts, nonce, fileNames(files)).WithContentHash(contentHash)
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		t.Fatalf("SigningBytes() error = %v", err)
	}
	return ed25519.Sign(s.privateKey, signingBytes)
}

type fakeRepository struct {
	now                     time.Time
	labManifest             []byte
	userKeys                map[int64]sqlc.UserKeys
	artifacts               *fakeArtifactStore
	beginCalls              int
	quotaUsage              int64
	latestSubmission        sqlc.Submissions
	lastQuotaUserID         int64
	lastQuotaLabID          string
	lastQuotaWindowStart    time.Time
	lastQuotaWindowEnd      time.Time
	failCreateJob           error
	lastTx                  *fakeTx
	lastCommittedSubmission sqlc.Submissions
	lastCommittedJob        sqlc.EvaluationJobs
	reservedNonces          map[string]struct{}
}

func newTestRepo(t *testing.T) (*fakeRepository, testSigner) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	sshKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("NewPublicKey() error = %v", err)
	}
	fingerprint := auth.PublicKeyFingerprint(publicKey)
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		now: now,
		labManifest: mustStoredManifest(t, testManifest(`
visible = 2026-03-01T00:00:00Z
open = 2026-03-01T00:00:00Z
close = 2026-04-30T00:00:00Z
`)),
		userKeys: map[int64]sqlc.UserKeys{
			11: {
				ID:        11,
				UserID:    7,
				PublicKey: string(bytes.TrimSpace(ssh.MarshalAuthorizedKey(sshKey))),
			},
		},
		artifacts:      newFakeArtifactStore(),
		reservedNonces: map[string]struct{}{},
	}
	return repo, testSigner{privateKey: privateKey, fingerprint: fingerprint}
}

func (r *fakeRepository) GetLab(_ context.Context, labID string) (sqlc.Labs, error) {
	if labID != "sorting" {
		return sqlc.Labs{}, pgx.ErrNoRows
	}
	return sqlc.Labs{
		ID:       "sorting",
		Name:     "Sorting Lab",
		Manifest: append([]byte(nil), r.labManifest...),
	}, nil
}

func (r *fakeRepository) GetUserKeyByFingerprint(_ context.Context, fingerprint string) (sqlc.UserKeys, error) {
	for _, key := range r.userKeys {
		publicKey, err := parseAuthorizedKey(key.PublicKey)
		if err != nil {
			return sqlc.UserKeys{}, err
		}
		if auth.PublicKeyFingerprint(publicKey) == fingerprint {
			return key, nil
		}
	}
	return sqlc.UserKeys{}, pgx.ErrNoRows
}

func (r *fakeRepository) ListUserKeys(_ context.Context, userID int64) ([]sqlc.UserKeys, error) {
	keys := make([]sqlc.UserKeys, 0, len(r.userKeys))
	for _, key := range r.userKeys {
		if key.UserID == userID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (r *fakeRepository) BeginTx(context.Context) (Tx, error) {
	r.beginCalls++
	tx := &fakeTx{repo: r}
	r.lastTx = tx
	return tx, nil
}

func (r *fakeRepository) ReserveNonce(_ context.Context, nonce string) (bool, error) {
	if _, ok := r.reservedNonces[nonce]; ok {
		return false, nil
	}
	r.reservedNonces[nonce] = struct{}{}
	return true, nil
}

func (r *fakeRepository) CountSubmissionQuotaUsage(_ context.Context, userID int64, labID string, start, end time.Time) (int64, error) {
	r.lastQuotaUserID = userID
	r.lastQuotaLabID = labID
	r.lastQuotaWindowStart = start
	r.lastQuotaWindowEnd = end
	return r.quotaUsage, nil
}

func (r *fakeRepository) GetLatestSubmissionByUserLab(_ context.Context, userID int64, labID string) (sqlc.Submissions, error) {
	if r.latestSubmission.ID == uuid.Nil || r.latestSubmission.UserID != userID || r.latestSubmission.LabID != labID {
		return sqlc.Submissions{}, pgx.ErrNoRows
	}
	return r.latestSubmission, nil
}

type fakeTx struct {
	repo              *fakeRepository
	stagedSubmission  sqlc.Submissions
	stagedJob         sqlc.EvaluationJobs
	quotaLockCalls    int
	quotaCountCalls   int
	reserveNonceCalls int
	commitCalls       int
	rollbackCalls     int
}

func (tx *fakeTx) LockSubmissionQuota(_ context.Context, userID int64, labID string, dayStart time.Time) error {
	tx.quotaLockCalls++
	return nil
}

func (tx *fakeTx) CountSubmissionQuotaUsage(_ context.Context, userID int64, labID string, start, end time.Time) (int64, error) {
	tx.quotaCountCalls++
	tx.repo.lastQuotaUserID = userID
	tx.repo.lastQuotaLabID = labID
	tx.repo.lastQuotaWindowStart = start
	tx.repo.lastQuotaWindowEnd = end
	return tx.repo.quotaUsage, nil
}

func (tx *fakeTx) ReserveNonce(_ context.Context, nonce string) (bool, error) {
	tx.reserveNonceCalls++
	return tx.repo.ReserveNonce(context.Background(), nonce)
}

func (tx *fakeTx) CreateSubmission(_ context.Context, arg sqlc.CreateSubmissionParams) (sqlc.Submissions, error) {
	tx.stagedSubmission = sqlc.Submissions{
		ID:          uuid.MustParse("11111111-1111-7111-8111-111111111111"),
		UserID:      arg.UserID,
		LabID:       arg.LabID,
		KeyID:       arg.KeyID,
		ArtifactKey: arg.ArtifactKey,
		ContentHash: arg.ContentHash,
		Status:      arg.Status,
	}
	return tx.stagedSubmission, nil
}

func (tx *fakeTx) CreateEvaluationJob(_ context.Context, submissionID uuid.UUID) (sqlc.EvaluationJobs, error) {
	if tx.repo.failCreateJob != nil {
		return sqlc.EvaluationJobs{}, tx.repo.failCreateJob
	}
	tx.stagedJob = sqlc.EvaluationJobs{
		ID:           uuid.MustParse("22222222-2222-7222-8222-222222222222"),
		SubmissionID: submissionID,
		Status:       "queued",
		AvailableAt:  pgtype.Timestamptz{Time: tx.repo.now, Valid: true},
	}
	return tx.stagedJob, nil
}

func (tx *fakeTx) Commit(context.Context) error {
	tx.commitCalls++
	tx.repo.lastCommittedSubmission = tx.stagedSubmission
	tx.repo.lastCommittedJob = tx.stagedJob
	return nil
}

func (tx *fakeTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type fakeArtifactStore struct {
	saveCalls   int
	lastKey     string
	lastArchive []byte
	deleteCalls int
}

func newFakeArtifactStore() *fakeArtifactStore {
	return &fakeArtifactStore{}
}

func (s *fakeArtifactStore) Save(_ context.Context, key string, archive []byte) error {
	s.saveCalls++
	s.lastKey = key
	s.lastArchive = append([]byte(nil), archive...)
	return nil
}

func (s *fakeArtifactStore) Delete(_ context.Context, key string) error {
	s.deleteCalls++
	if s.lastKey == key {
		s.lastArchive = nil
	}
	return nil
}

func mustContentHash(t *testing.T, files []UploadFile) string {
	t.Helper()
	artifactFiles := make([]storage.ArtifactFile, 0, len(files))
	for _, file := range files {
		artifactFiles = append(artifactFiles, storage.ArtifactFile{
			Name:    file.Name,
			Content: append([]byte(nil), file.Content...),
		})
	}
	_, contentHash, err := storage.Archive(artifactFiles)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	return contentHash
}

func fileNames(files []UploadFile) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return names
}

func mustStoredManifest(t *testing.T, raw string) []byte {
	t.Helper()
	parsed, err := manifest.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("manifest.Parse() error = %v", err)
	}
	data, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func testManifest(schedule string) string {
	return testManifestWithMaxSize("1MB", schedule)
}

func testManifestWithMaxSize(maxSize, schedule string) string {
	return `
[lab]
id = "sorting"
name = "Sorting Lab"

[submit]
files = ["main.c", "README.md"]
max_size = "` + maxSize + `"

[eval]
image = "ghcr.io/labkit/sorting:latest"
timeout = 60

[quota]
daily = 3

[[metric]]
id = "time_ms"
name = "Time"
sort = "asc"
unit = "ms"

[board]
rank_by = "time_ms"
pick = true

[schedule]
` + schedule
}

func untarGzip(data []byte) (map[string][]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[hdr.Name] = content
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q) error = %v", name, err)
	}
	return location
}
