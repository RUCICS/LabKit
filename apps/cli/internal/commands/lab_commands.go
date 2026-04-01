package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"labkit.local/apps/cli/internal/config"
	"labkit.local/apps/cli/internal/ui"
	"labkit.local/packages/go/auth"
	"labkit.local/packages/go/manifest"

	"github.com/spf13/cobra"
)

type labResponse struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Manifest          manifest.PublicManifest `json:"manifest"`
	ManifestUpdatedAt time.Time               `json:"manifest_updated_at,omitempty"`
}

type boardResponse struct {
	LabID          string                `json:"lab_id"`
	SelectedMetric string                `json:"selected_metric"`
	Metrics        []boardMetricResponse `json:"metrics"`
	Rows           []boardRowResponse    `json:"rows"`
}

type boardMetricResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Sort     string `json:"sort"`
	Selected bool   `json:"selected,omitempty"`
}

type boardRowResponse struct {
	Rank        int                  `json:"rank"`
	Nickname    string               `json:"nickname"`
	Track       string               `json:"track,omitempty"`
	Scores      []boardScoreResponse `json:"scores"`
	UpdatedAt   time.Time            `json:"updated_at"`
	CurrentUser bool                 `json:"current_user,omitempty"`
}

type boardScoreResponse struct {
	MetricID string  `json:"metric_id"`
	Value    float32 `json:"value"`
}

type historyResponse struct {
	Submissions []historyItemResponse `json:"submissions"`
}

type historyItemResponse struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Verdict    string     `json:"verdict,omitempty"`
	Message    string     `json:"message,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type profileResponse struct {
	LabID    string `json:"lab_id"`
	Nickname string `json:"nickname"`
	Track    string `json:"track,omitempty"`
	Pick     bool   `json:"pick"`
}

type submissionResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	ArtifactKey string `json:"artifact_key,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

type submissionDetailResponse struct {
	ID         string                `json:"id"`
	Status     string                `json:"status"`
	Verdict    string                `json:"verdict,omitempty"`
	Message    string                `json:"message,omitempty"`
	Detail     json.RawMessage       `json:"detail,omitempty"`
	Scores     []submissionScoreItem `json:"scores,omitempty"`
	CreatedAt  time.Time             `json:"created_at"`
	FinishedAt *time.Time            `json:"finished_at,omitempty"`
}

type submissionScoreItem struct {
	MetricID string  `json:"metric_id"`
	Value    float64 `json:"value"`
}

var submitInterruptContext = func(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}

type submissionFile struct {
	Name    string
	Path    string
	Content []byte
}

func NewSubmitCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	var detach bool
	var noWait bool
	cmd := &cobra.Command{
		Use:   "submit <files...>",
		Short: "Submit lab files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSubmit(cmd.Context(), deps, args, detach, noWait)
		},
	}
	cmd.Flags().BoolVar(&detach, "detach", false, "Submit and exit without waiting for the result")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "Submit and exit without waiting for the result")
	return cmd
}

func NewBoardCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	var by string
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Show the leaderboard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoard(cmd.Context(), deps, by)
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "Select a metric to rank by")
	return cmd
}

func NewHistoryCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "history [submission-id]",
		Short: "Show submission history",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			submissionID := ""
			if len(args) > 0 {
				submissionID = args[0]
			}
			return runHistory(cmd.Context(), deps, submissionID)
		},
	}
}

func NewNickCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "nick <name>",
		Short: "Update your nickname",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNick(cmd.Context(), deps, args[0])
		},
	}
}

func NewTrackCommand(deps *Dependencies) *cobra.Command {
	deps = normalizeDependencies(deps)
	return &cobra.Command{
		Use:   "track <metric_id>",
		Short: "Update your track",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrack(cmd.Context(), deps, args[0])
		},
	}
}

func runSubmit(ctx context.Context, deps *Dependencies, args []string, detach, noWait bool) error {
	labID, err := activeLabID(deps)
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(deps)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(deps)
	if err != nil {
		return err
	}

	client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
	if err != nil {
		return err
	}
	waitSpinner := newLineSpinner(deps.Out, submitOutputIsTTY(deps.Out), "Contacting server...")
	stopWaitSpinner := func() {
		if waitSpinner == nil {
			return
		}
		_ = waitSpinner.Stop()
		waitSpinner = nil
	}
	if err := waitSpinner.Start(); err != nil {
		return err
	}
	lab, err := client.getLab(ctx, labID)
	if err != nil {
		stopWaitSpinner()
		return err
	}
	files, err := validateSubmissionFiles(lab.Manifest.Submit.Files, args)
	if err != nil {
		stopWaitSpinner()
		return err
	}
	archiveHash, err := submissionArchiveHash(files)
	if err != nil {
		stopWaitSpinner()
		return err
	}

	body, contentType, err := buildMultipartSubmission(files)
	if err != nil {
		stopWaitSpinner()
		return err
	}
	if cfg.KeyID == 0 {
		stopWaitSpinner()
		return fmt.Errorf("key id is required; run auth first")
	}
	privateKey, err := readPrivateKeyWithDeps(deps, cfg.KeyPath)
	if err != nil {
		stopWaitSpinner()
		return err
	}
	nonce, err := newNonce()
	if err != nil {
		stopWaitSpinner()
		return err
	}
	payload := auth.NewPayload(labID, client.now().UTC(), nonce, submissionFileNames(files)).
		WithContentHash(archiveHash)
	if err := waitSpinner.Update("Sending submission..."); err != nil {
		stopWaitSpinner()
		return err
	}
	req, err := client.signedRequestWithPayload(ctx, http.MethodPost, "/api/labs/"+labID+"/submit", body, contentType, cfg, privateKey, payload)
	if err != nil {
		stopWaitSpinner()
		return err
	}

	var submission submissionResponse
	if err := client.doJSON(req, &submission); err != nil {
		stopWaitSpinner()
		return err
	}
	stopWaitSpinner()

	if detach || noWait {
		return renderSubmissionSummary(deps.Out, submission, "", "")
	}

	waitCtx, stop := submitInterruptContext(ctx)
	defer stop()

	current := submission
	lastStatus := ""
	statusHistory := []string{}
	var detail submissionDetailResponse
	if err := renderSubmissionTaskStart(deps.Out, labID); err != nil {
		return err
	}
	var live *submitLiveRenderer
	if submitOutputIsTTY(deps.Out) {
		live = newSubmitLiveRenderer(deps.Out, deps.Now, client.now().UTC(), current.Status)
		if err := live.Start(); err != nil {
			return err
		}
	}
	for {
		if current.Status != lastStatus {
			statusHistory = appendSubmissionStatus(statusHistory, current.Status)
			if live != nil {
				if err := live.Update(current.Status); err != nil {
					return err
				}
			}
			lastStatus = current.Status
		}
		if submissionIsTerminal(current.Status) {
			if detail.ID != "" && detail.ID == current.ID && strings.EqualFold(detail.Status, current.Status) {
				break
			}
			fetched, err := client.getSubmissionDetail(waitCtx, labID, current.ID, cfg, privateKey)
			if err != nil {
				if live != nil {
					if stopErr := live.Stop(); stopErr != nil {
						return stopErr
					}
				} else if err := renderSubmissionStatusSummary(deps.Out, statusHistory); err != nil {
					return err
				}
				return submitWaitError(deps.Out, current.ID, err)
			}
			detail = fetched
			break
		}

		select {
		case <-waitCtx.Done():
			if live != nil {
				if err := live.Stop(); err != nil {
					return err
				}
			} else if err := renderSubmissionStatusSummary(deps.Out, statusHistory); err != nil {
				return err
			}
			return submitWaitInterrupted(deps.Out, current.ID)
		case <-time.After(deps.PollInterval):
		}

		fetched, err := client.getSubmissionDetail(waitCtx, labID, current.ID, cfg, privateKey)
		if err != nil {
			if live != nil {
				if stopErr := live.Stop(); stopErr != nil {
					return stopErr
				}
			} else if err := renderSubmissionStatusSummary(deps.Out, statusHistory); err != nil {
				return err
			}
			return submitWaitError(deps.Out, current.ID, err)
		}
		current.Status = fetched.Status
		detail = fetched
	}

	if live != nil {
		if err := live.Stop(); err != nil {
			return err
		}
	} else if err := renderSubmissionStatusSummary(deps.Out, statusHistory); err != nil {
		return err
	}

	return renderSubmissionFinal(deps.Out, lab.Manifest, detail)
}

func runBoard(ctx context.Context, deps *Dependencies, by string) error {
	labID, err := activeLabID(deps)
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(deps)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(deps)
	if err != nil {
		return err
	}
	client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
	if err != nil {
		return err
	}
	lab, err := client.getLab(ctx, labID)
	if err != nil {
		return err
	}
	var board boardResponse
	if cfg.KeyID != 0 {
		privateKey, err := readPrivateKeyWithDeps(deps, cfg.KeyPath)
		if err != nil {
			return err
		}
		board, err = client.getSignedBoard(ctx, labID, by, cfg, privateKey)
	} else {
		board, err = client.getBoard(ctx, labID, by)
	}
	if err != nil {
		return err
	}
	return renderBoard(deps.Out, lab.Manifest, board)
}

func runHistory(ctx context.Context, deps *Dependencies, submissionID string) error {
	labID, err := activeLabID(deps)
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(deps)
	if err != nil {
		return err
	}
	if cfg.KeyID == 0 {
		return fmt.Errorf("key id is required; run auth first")
	}
	privateKey, err := readPrivateKeyWithDeps(deps, cfg.KeyPath)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(deps)
	if err != nil {
		return err
	}
	client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
	if err != nil {
		return err
	}
	if strings.TrimSpace(submissionID) != "" {
		lab, err := client.getLab(ctx, labID)
		if err != nil {
			return err
		}
		detail, err := client.getSubmissionDetail(ctx, labID, strings.TrimSpace(submissionID), cfg, privateKey)
		if err != nil {
			return err
		}
		return renderSubmissionDetailView(deps.Out, lab.Manifest, detail)
	}
	history, err := client.getHistory(ctx, labID, cfg, privateKey)
	if err != nil {
		return err
	}
	return renderHistory(deps.Out, history)
}

func runNick(ctx context.Context, deps *Dependencies, nickname string) error {
	labID, err := activeLabID(deps)
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(deps)
	if err != nil {
		return err
	}
	if cfg.KeyID == 0 {
		return fmt.Errorf("key id is required; run auth first")
	}
	privateKey, err := readPrivateKeyWithDeps(deps, cfg.KeyPath)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(deps)
	if err != nil {
		return err
	}
	client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
	if err != nil {
		return err
	}
	profile, err := client.updateNickname(ctx, labID, nickname, cfg, privateKey)
	if err != nil {
		return err
	}
	theme := ui.DefaultTheme()
	fmt.Fprintln(deps.Out, theme.SuccessStyle.Render("✓")+" "+theme.TitleStyle.Render("Nickname updated")+"  "+theme.ValueStyle.Render(profile.Nickname))
	return nil
}

func runTrack(ctx context.Context, deps *Dependencies, track string) error {
	labID, err := activeLabID(deps)
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(deps)
	if err != nil {
		return err
	}
	if cfg.KeyID == 0 {
		return fmt.Errorf("key id is required; run auth first")
	}
	privateKey, err := readPrivateKeyWithDeps(deps, cfg.KeyPath)
	if err != nil {
		return err
	}
	serverURL, err := resolveServerURL(deps)
	if err != nil {
		return err
	}
	client, err := newAPIClient(serverURL, deps.HTTPClient, deps.Now)
	if err != nil {
		return err
	}
	lab, err := client.getLab(ctx, labID)
	if err != nil {
		return err
	}
	if !lab.Manifest.Board.Pick {
		return fmt.Errorf("track selection is disabled")
	}
	if !manifestHasMetric(lab.Manifest.Metrics, track) {
		return fmt.Errorf("invalid track")
	}
	profile, err := client.updateTrack(ctx, labID, track, cfg, privateKey)
	if err != nil {
		return err
	}
	theme := ui.DefaultTheme()
	fmt.Fprintln(deps.Out, theme.SuccessStyle.Render("✓")+" "+theme.TitleStyle.Render("Track set")+"  "+theme.ValueStyle.Render(profile.Track))
	return nil
}

func activeLabID(deps *Dependencies) (string, error) {
	if deps == nil {
		return "", fmt.Errorf("lab id is required; pass --lab or set lab in .labkit/config.toml")
	}
	cfg, err := resolveConfig(deps)
	if err != nil {
		return "", err
	}
	labID := strings.TrimSpace(cfg.Lab)
	if labID == "" {
		return "", fmt.Errorf("lab id is required; pass --lab or set lab in .labkit/config.toml")
	}
	return labID, nil
}

func resolveServerURL(deps *Dependencies) (string, error) {
	cfg, err := resolveConfig(deps)
	if err != nil {
		return "", err
	}
	serverURL := strings.TrimSpace(cfg.ServerURL)
	if serverURL == "" {
		return "", fmt.Errorf("server URL is required")
	}
	return serverURL, nil
}

func (c *apiClient) getLab(ctx context.Context, labID string) (labResponse, error) {
	req, err := c.base.NewRequest(ctx, http.MethodGet, "/api/labs/"+labID, nil)
	if err != nil {
		return labResponse{}, err
	}
	var result labResponse
	if err := c.doJSON(req, &result); err != nil {
		return labResponse{}, err
	}
	return result, nil
}

func (c *apiClient) getBoard(ctx context.Context, labID, by string) (boardResponse, error) {
	path := "/api/labs/" + labID + "/board"
	if strings.TrimSpace(by) != "" {
		path += "?by=" + url.QueryEscape(by)
	}
	req, err := c.base.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return boardResponse{}, err
	}
	var result boardResponse
	if err := c.doJSON(req, &result); err != nil {
		return boardResponse{}, err
	}
	return result, nil
}

func (c *apiClient) getSignedBoard(ctx context.Context, labID, by string, cfg config.Config, private ed25519.PrivateKey) (boardResponse, error) {
	path := "/api/labs/" + labID + "/board"
	if strings.TrimSpace(by) != "" {
		path += "?by=" + url.QueryEscape(by)
	}
	req, err := c.signedRequest(ctx, http.MethodGet, path, nil, cfg, private)
	if err != nil {
		return boardResponse{}, err
	}
	var result boardResponse
	if err := c.doJSON(req, &result); err != nil {
		return boardResponse{}, err
	}
	return result, nil
}

func (c *apiClient) getSubmissionDetail(ctx context.Context, labID, submissionID string, cfg config.Config, private ed25519.PrivateKey) (submissionDetailResponse, error) {
	path := "/api/labs/" + labID + "/submissions/" + submissionID
	req, err := c.signedRequest(ctx, http.MethodGet, path, nil, cfg, private)
	if err != nil {
		return submissionDetailResponse{}, err
	}
	var result submissionDetailResponse
	if err := c.doJSON(req, &result); err != nil {
		return submissionDetailResponse{}, err
	}
	return result, nil
}

func (c *apiClient) getHistory(ctx context.Context, labID string, cfg config.Config, private ed25519.PrivateKey) (historyResponse, error) {
	req, err := c.signedRequest(ctx, http.MethodGet, "/api/labs/"+labID+"/history", nil, cfg, private)
	if err != nil {
		return historyResponse{}, err
	}
	var result historyResponse
	if err := c.doJSON(req, &result); err != nil {
		return historyResponse{}, err
	}
	return result, nil
}

func (c *apiClient) updateNickname(ctx context.Context, labID, nickname string, cfg config.Config, private ed25519.PrivateKey) (profileResponse, error) {
	req, err := c.signedRequest(ctx, http.MethodPut, "/api/labs/"+labID+"/nickname", map[string]string{"nickname": nickname}, cfg, private)
	if err != nil {
		return profileResponse{}, err
	}
	var result profileResponse
	if err := c.doJSON(req, &result); err != nil {
		return profileResponse{}, err
	}
	return result, nil
}

func (c *apiClient) updateTrack(ctx context.Context, labID, track string, cfg config.Config, private ed25519.PrivateKey) (profileResponse, error) {
	req, err := c.signedRequest(ctx, http.MethodPut, "/api/labs/"+labID+"/track", map[string]string{"track": track}, cfg, private)
	if err != nil {
		return profileResponse{}, err
	}
	var result profileResponse
	if err := c.doJSON(req, &result); err != nil {
		return profileResponse{}, err
	}
	return result, nil
}

func (c *apiClient) createSubmission(ctx context.Context, labID string, body []byte, contentType string, cfg config.Config, private ed25519.PrivateKey) (submissionResponse, error) {
	req, err := c.signedRawRequest(ctx, http.MethodPost, "/api/labs/"+labID+"/submit", body, contentType, cfg, private)
	if err != nil {
		return submissionResponse{}, err
	}
	var result submissionResponse
	if err := c.doJSON(req, &result); err != nil {
		return submissionResponse{}, err
	}
	return result, nil
}

func (c *apiClient) signedRequestWithPayload(ctx context.Context, method, path string, body []byte, contentType string, cfg config.Config, private ed25519.PrivateKey, payload auth.Payload) (*http.Request, error) {
	if cfg.KeyID == 0 {
		return nil, fmt.Errorf("key id is required")
	}
	req, err := c.base.NewRequestWithBytes(ctx, method, path, body, contentType)
	if err != nil {
		return nil, err
	}
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(private, signingBytes)
	fingerprint, err := keyFingerprintFromPrivateKey(private)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-LabKit-Key-Fingerprint", fingerprint)
	req.Header.Set("X-LabKit-Timestamp", payload.Timestamp.UTC().Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", payload.Nonce)
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString(sig))
	return req, nil
}

func (c *apiClient) signedRawRequest(ctx context.Context, method, path string, body []byte, contentType string, cfg config.Config, private ed25519.PrivateKey) (*http.Request, error) {
	if cfg.KeyID == 0 {
		return nil, fmt.Errorf("key id is required")
	}
	req, err := c.base.NewRequestWithBytes(ctx, method, path, body, contentType)
	if err != nil {
		return nil, err
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, err
	}
	now := c.now().UTC()
	payload := auth.NewPayload(strings.ToUpper(strings.TrimSpace(method))+" "+path, now, nonce, nil).
		WithContentHash(sha256Hex(body))
	signingBytes, err := payload.SigningBytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(private, signingBytes)
	fingerprint, err := keyFingerprintFromPrivateKey(private)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-LabKit-Key-Fingerprint", fingerprint)
	req.Header.Set("X-LabKit-Timestamp", now.Format(time.RFC3339Nano))
	req.Header.Set("X-LabKit-Nonce", nonce)
	req.Header.Set("X-LabKit-Signature", base64.StdEncoding.EncodeToString(sig))
	return req, nil
}

func validateSubmissionFiles(expected []string, paths []string) ([]submissionFile, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("missing required files")
	}
	expectedOrder := append([]string(nil), expected...)
	if len(expectedOrder) == 0 {
		return nil, fmt.Errorf("manifest does not declare submission files")
	}

	provided := make(map[string]string, len(paths))
	for _, path := range paths {
		clean := strings.TrimSpace(path)
		if clean == "" {
			return nil, fmt.Errorf("invalid submission file path")
		}
		name := filepath.Base(clean)
		if _, ok := provided[name]; ok {
			return nil, fmt.Errorf("duplicate file %q", name)
		}
		provided[name] = clean
	}

	missing := make([]string, 0)
	ordered := make([]submissionFile, 0, len(expectedOrder))
	for _, name := range expectedOrder {
		path, ok := provided[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		ordered = append(ordered, submissionFile{Name: name, Path: path, Content: content})
		delete(provided, name)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required files: %s", strings.Join(missing, ", "))
	}
	if len(provided) > 0 {
		extra := make([]string, 0, len(provided))
		for name := range provided {
			extra = append(extra, name)
		}
		sort.Strings(extra)
		return nil, fmt.Errorf("unexpected files: %s", strings.Join(extra, ", "))
	}
	return ordered, nil
}

func buildMultipartSubmission(files []submissionFile) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.Name)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(file.Content); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func submissionArchiveHash(files []submissionFile) (string, error) {
	sorted := append([]submissionFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
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
			return "", err
		}
		if _, err := tw.Write(file.Content); err != nil {
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gzw.Close(); err != nil {
		return "", err
	}

	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func submissionFileNames(files []submissionFile) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return names
}

func renderBoard(out io.Writer, lab manifest.PublicManifest, board boardResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()

	metricSuffix := ""
	if strings.TrimSpace(board.SelectedMetric) != "" {
		metricSuffix = " · sorted by " + board.SelectedMetric
	}
	count := fmt.Sprintf(" · %d participants", len(board.Rows))
	title := theme.TitleStyle.Render("Leaderboard") +
		theme.MutedStyle.Render(metricSuffix+count)

	if len(board.Metrics) > 1 {
		tabs := make([]string, len(board.Metrics))
		for i, m := range board.Metrics {
			if m.Selected || strings.EqualFold(m.ID, board.SelectedMetric) {
				tabs[i] = theme.TitleStyle.Render(m.Name)
			} else {
				tabs[i] = theme.MutedStyle.Render(m.Name)
			}
		}
		if _, err := fmt.Fprintln(out, title+"\n  "+strings.Join(tabs, " · ")); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(out, title); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}

	const (
		rankW    = 5
		nickW    = 18
		scoreW   = 8
		updatedW = 8
		gap      = "  "
	)
	rowWidth := rankW + len(gap) + nickW + len(gap) + scoreW + len(gap) + updatedW

	header := ui.PadRight(theme.MutedStyle.Render("#"), rankW) + gap +
		ui.PadRight(theme.MutedStyle.Render("NICKNAME"), nickW) + gap +
		ui.PadRight(theme.MutedStyle.Render("SCORE"), scoreW) + gap +
		theme.MutedStyle.Render("UPDATED")
	if _, err := fmt.Fprintln(out, "  "+header); err != nil {
		return err
	}
	sep := theme.SeparatorStyle.Render(strings.Repeat("─", rowWidth))
	if _, err := fmt.Fprintln(out, "  "+sep); err != nil {
		return err
	}

	unitByID := make(map[string]string, len(lab.Metrics))
	for _, metric := range lab.Metrics {
		unitByID[metric.ID] = metric.Unit
	}
	maxScore := float32(0)
	for _, row := range board.Rows {
		for _, s := range row.Scores {
			if s.MetricID == board.SelectedMetric && s.Value > maxScore {
				maxScore = s.Value
			}
		}
	}

	now := time.Now()
	for _, row := range board.Rows {
		scoreVal := float32(0)
		for _, s := range row.Scores {
			if s.MetricID == board.SelectedMetric {
				scoreVal = s.Value
			}
		}
		scoreStr := formatScore(scoreVal, unitByID[board.SelectedMetric])
		updatedStr := ui.RelativeTime(row.UpdatedAt, now)

		displayName := row.Nickname
		if row.CurrentUser {
			displayName = fmt.Sprintf("you (%s)", row.Nickname)
		}

		var fgColor, bgColor lipgloss.Color
		switch {
		case row.CurrentUser:
			fgColor = lipgloss.Color("#9ece6a")
			bgColor = lipgloss.Color("#1f2d1a")
		case row.Rank == 1:
			fgColor = lipgloss.Color("#e0af68")
			bgColor = lipgloss.Color("#2a2015")
		default:
			fgColor = lipgloss.Color("#c0caf5")
			bgColor = lipgloss.Color("#1e2030")
		}

		renderedRank := renderBoardRankBadge(theme, row.Rank)
		restRow := ui.PadRight(displayName, nickW) + gap +
			ui.PadRight(scoreStr, scoreW) + gap +
			ui.PadRight(updatedStr, updatedW)

		fillFraction := 0.0
		if maxScore > 0 {
			fillFraction = float64(scoreVal) / float64(maxScore)
		}

		rendered := "  " + renderedRank + gap + ui.BgFillRow(restRow, fillFraction, fgColor, bgColor)
		if _, err := fmt.Fprintln(out, rendered); err != nil {
			return err
		}
	}

	return nil
}

func renderHistory(out io.Writer, history historyResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	now := time.Now()

	count := fmt.Sprintf("  %s", theme.MutedStyle.Render(fmt.Sprintf("%d submissions", len(history.Submissions))))
	title := theme.TitleStyle.Render("Submission history")
	if _, err := fmt.Fprintln(out, title+"\n"); err != nil {
		return err
	}

	const (
		statusW  = 10
		verdictW = 10
		gap      = "  "
	)
	idW := len("ID")
	for _, item := range history.Submissions {
		if len(item.ID) > idW {
			idW = len(item.ID)
		}
	}
	if idW < 12 {
		idW = 12
	}
	header := ui.PadRight(theme.MutedStyle.Render("ID"), idW) + gap +
		ui.PadRight(theme.MutedStyle.Render("STATUS"), statusW) + gap +
		ui.PadRight(theme.MutedStyle.Render("VERDICT"), verdictW) + gap +
		theme.MutedStyle.Render("SUBMITTED")
	rowWidth := idW + len(gap) + statusW + len(gap) + verdictW + len(gap) + 10
	sep := theme.SeparatorStyle.Render(strings.Repeat("─", rowWidth))

	if _, err := fmt.Fprintln(out, "  "+header); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  "+sep); err != nil {
		return err
	}

	for _, item := range history.Submissions {
		var statusStr string
		switch strings.ToLower(item.Status) {
		case "completed", "done":
			statusStr = theme.SuccessStyle.Render(item.Status)
		case "failed", "error":
			statusStr = theme.ErrorStyle.Render(item.Status)
		case "queued", "pending":
			statusStr = theme.WarningStyle.Render(item.Status)
		case "running":
			statusStr = theme.InfoStyle.Render(item.Status)
		default:
			statusStr = theme.MutedStyle.Render(item.Status)
		}

		var verdictStr string
		switch strings.ToLower(item.Verdict) {
		case "scored", "passed":
			verdictStr = theme.SuccessStyle.Render(item.Verdict)
		case "failed", "error":
			verdictStr = theme.ErrorStyle.Render(item.Verdict)
		default:
			verdictStr = theme.MutedStyle.Render(item.Verdict)
		}
		if strings.TrimSpace(item.Verdict) == "" {
			verdictStr = theme.MutedStyle.Render("—")
		}

		submittedStr := ui.RelativeTime(item.CreatedAt, now)

		row := ui.PadRight(theme.ValueStyle.Render(item.ID), idW) + gap +
			ui.PadRight(statusStr, statusW) + gap +
			ui.PadRight(verdictStr, verdictW) + gap +
			theme.MutedStyle.Render(submittedStr)

		if _, err := fmt.Fprintln(out, "  "+row); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	_, err := fmt.Fprintln(out, count)
	return err
}

func renderBoardRankBadge(theme ui.Theme, rank int) string {
	switch rank {
	case 1:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0f111a")).
			Background(lipgloss.Color("#e0af68")).
			Padding(0, 1).
			Render("1ST")
	case 2:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0f111a")).
			Background(lipgloss.Color("#b7c1d3")).
			Padding(0, 1).
			Render("2ND")
	case 3:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0f111a")).
			Background(lipgloss.Color("#c8925b")).
			Padding(0, 1).
			Render("3RD")
	default:
		return ui.PadRight(theme.MutedStyle.Render(fmt.Sprintf("%d", rank)), 5)
	}
}

func formatScore(value float32, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%g", value)
	}
	return fmt.Sprintf("%g%s", value, unit)
}

func manifestHasMetric(metrics []manifest.MetricSection, wanted string) bool {
	for _, metric := range metrics {
		if metric.ID == wanted {
			return true
		}
	}
	return false
}

func renderSubmissionTaskStart(out io.Writer, labID string) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	title := theme.InfoStyle.Render("●") + " " + theme.TitleStyle.Render("Submitting") + "  " + theme.ValueStyle.Render(labID)
	_, err := fmt.Fprintln(out, title)
	return err
}

func appendSubmissionStatus(history []string, status string) []string {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		return history
	}
	if len(history) > 0 && history[len(history)-1] == normalized {
		return history
	}
	return append(history, normalized)
}

func renderSubmissionStatusSummary(out io.Writer, statuses []string) error {
	if out == nil {
		out = io.Discard
	}
	if len(statuses) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(out, "  state   %s\n", strings.Join(statuses, " -> "))
	return err
}

func renderSubmissionSummary(out io.Writer, submission submissionResponse, verdict, elapsed string) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	title := theme.InfoStyle.Render("●") + " " + theme.TitleStyle.Render("Submitted") + "  " +
		theme.MutedStyle.Render("(detached)")
	if _, err := fmt.Fprintln(out, title); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  %s  %s\n",
		theme.MutedStyle.Render("id"),
		theme.ValueStyle.Render(submission.ID)); err != nil {
		return err
	}
	return nil
}

func renderSubmissionFinal(out io.Writer, lab manifest.PublicManifest, detail submissionDetailResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()

	block, isFailure := submissionResultBlock(theme, detail, true)

	var prefix string
	if isFailure {
		prefix = theme.ErrorStyle.Render("✗")
	} else {
		prefix = theme.SuccessStyle.Render("✓")
	}
	titleLine := prefix + " " + theme.TitleStyle.Render("Submitted")

	if _, err := fmt.Fprintln(out, titleLine); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, block.Render()); err != nil {
		return err
	}

	if !isFailure && len(detail.Scores) > 0 {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		return renderSubmissionScores(out, lab, detail.Scores)
	}
	return nil
}

func renderSubmissionDetailView(out io.Writer, lab manifest.PublicManifest, detail submissionDetailResponse) error {
	if out == nil {
		out = io.Discard
	}
	theme := ui.DefaultTheme()
	block, _ := submissionResultBlock(theme, detail, false)

	if _, err := fmt.Fprintln(out, theme.TitleStyle.Render("Submission details")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, block.Render()); err != nil {
		return err
	}
	if len(detail.Scores) > 0 {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		if err := renderSubmissionScores(out, lab, detail.Scores); err != nil {
			return err
		}
	}
	detailLines := renderSubmissionDetailLines(detail.Detail)
	if len(detailLines) == 0 && strings.TrimSpace(detail.Message) != "" {
		detailLines = []string{detail.Message}
	}
	if len(detailLines) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	for _, line := range detailLines {
		if _, err := fmt.Fprintln(out, "  "+line); err != nil {
			return err
		}
	}
	return nil
}

func renderSubmissionScores(out io.Writer, lab manifest.PublicManifest, scores []submissionScoreItem) error {
	if len(scores) == 0 {
		return nil
	}
	theme := ui.DefaultTheme()
	ordered := orderedSubmissionScores(lab, scores)
	if _, err := fmt.Fprintln(out, theme.LabelStyle.Render("  Scores")); err != nil {
		return err
	}
	width := 0
	for _, score := range ordered {
		if len(score.DisplayLabel) > width {
			width = len(score.DisplayLabel)
		}
	}
	for _, score := range ordered {
		valueStr := theme.SuccessStyle.Render(score.Value)
		if _, err := fmt.Fprintf(out, "  %-*s   %s\n", width, theme.MutedStyle.Render(score.DisplayLabel), valueStr); err != nil {
			return err
		}
	}
	return nil
}

func renderSubmissionFailureDetails(out io.Writer, detail submissionDetailResponse) error {
	lines := renderSubmissionDetailLines(detail.Detail)
	if len(lines) == 0 {
		return nil
	}
	if out == nil {
		out = io.Discard
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Details"); err != nil {
		return err
	}
	for _, line := range lines {
		if line == "" {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(out, "  %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

func submissionResultBlock(theme ui.Theme, detail submissionDetailResponse, includeFailureDetails bool) (ui.ResultBlock, bool) {
	elapsed := ""
	if detail.FinishedAt != nil && !detail.CreatedAt.IsZero() {
		elapsed = "   " + detail.FinishedAt.Sub(detail.CreatedAt).Round(time.Second).String()
	}

	passed := strings.EqualFold(detail.Verdict, "scored") || strings.EqualFold(detail.Status, "completed") && detail.Verdict == ""
	failed := !passed && detail.Verdict != ""
	isFailure := failed || strings.EqualFold(detail.Verdict, "failed") || strings.EqualFold(detail.Status, "failed")

	titleStr := theme.SuccessStyle.Render("PASSED") + elapsed
	if isFailure {
		titleStr = theme.ErrorStyle.Render("FAILED") + elapsed
	}

	detailLines := []string(nil)
	if includeFailureDetails && isFailure {
		detailLines = renderSubmissionDetailLines(detail.Detail)
		if msg := strings.TrimSpace(detail.Message); msg != "" && len(detailLines) == 0 {
			detailLines = []string{msg}
		}
	}

	return ui.ResultBlock{
		Title:   titleStr,
		ID:      detail.ID,
		Details: detailLines,
		Failed:  isFailure,
	}, isFailure
}

type renderedSubmissionScore struct {
	SummaryKey   string
	DisplayLabel string
	Value        string
}

func orderedSubmissionScores(lab manifest.PublicManifest, scores []submissionScoreItem) []renderedSubmissionScore {
	scoreByID := make(map[string]submissionScoreItem, len(scores))
	for _, score := range scores {
		scoreByID[score.MetricID] = score
	}
	ordered := make([]renderedSubmissionScore, 0, len(scores))
	seen := make(map[string]struct{}, len(scores))
	for _, metric := range lab.Metrics {
		score, ok := scoreByID[metric.ID]
		if !ok {
			continue
		}
		label := metric.ID
		if strings.TrimSpace(metric.Name) != "" {
			label = metric.Name
		}
		ordered = append(ordered, renderedSubmissionScore{
			SummaryKey:   metric.ID,
			DisplayLabel: label,
			Value:        formatScoreValue(score.Value, metric.Unit),
		})
		seen[metric.ID] = struct{}{}
	}
	extraIDs := make([]string, 0, len(scores))
	for _, score := range scores {
		if _, ok := seen[score.MetricID]; ok {
			continue
		}
		extraIDs = append(extraIDs, score.MetricID)
	}
	sort.Strings(extraIDs)
	for _, metricID := range extraIDs {
		score := scoreByID[metricID]
		ordered = append(ordered, renderedSubmissionScore{
			SummaryKey:   metricID,
			DisplayLabel: metricID,
			Value:        formatScoreValue(score.Value, ""),
		})
	}
	return ordered
}

func submissionScoreSummary(lab manifest.PublicManifest, scores []submissionScoreItem) (string, string) {
	ordered := orderedSubmissionScores(lab, scores)
	if len(ordered) == 0 {
		return "", ""
	}
	if len(ordered) == 1 {
		return "score", ordered[0].Value
	}
	pairs := make([]string, 0, len(ordered))
	for _, score := range ordered {
		pairs = append(pairs, score.SummaryKey+"="+score.Value)
	}
	return "metrics", strings.Join(pairs, ", ")
}

func compactSubmissionOutcome(status, verdict string) string {
	status = strings.TrimSpace(status)
	verdict = strings.TrimSpace(verdict)
	switch {
	case status != "" && verdict != "":
		return status + "/" + verdict
	case verdict != "":
		return verdict
	default:
		return status
	}
}

func renderSubmissionKV(out io.Writer, label, value string) error {
	if out == nil {
		out = io.Discard
	}
	_, err := fmt.Fprintf(out, "  %-7s %s\n", label, value)
	return err
}

func renderSubmissionDetailLines(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var structured struct {
		Format  string `json:"format"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &structured); err == nil && strings.TrimSpace(structured.Content) != "" {
		lines := []string{}
		if strings.TrimSpace(structured.Format) != "" {
			lines = append(lines, fmt.Sprintf("detail (%s):", structured.Format))
		}
		lines = append(lines, structured.Content)
		return lines
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

func renderSubmissionInterruptHint(out io.Writer, submissionID string) error {
	theme := ui.DefaultTheme()
	if _, err := fmt.Fprintln(out, theme.WarningStyle.Render("⚠ Waiting interrupted")); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  %s is still running on the server.\n  Check results later with %s or %s.\n",
		theme.MutedStyle.Render(submissionID),
		theme.InfoStyle.Render("labkit history"),
		theme.InfoStyle.Render("labkit board"),
	)
	return err
}

func submitWaitInterrupted(out io.Writer, submissionID string) error {
	if err := renderSubmissionInterruptHint(out, submissionID); err != nil {
		return err
	}
	return fmt.Errorf("waiting interrupted")
}

func submitWaitError(out io.Writer, submissionID string, err error) error {
	if errors.Is(err, context.Canceled) {
		return submitWaitInterrupted(out, submissionID)
	}
	return err
}

func submissionIsTerminal(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "queued", "running", "":
		return false
	default:
		return true
	}
}

func formatScoreValue(value float64, unit string) string {
	if strings.TrimSpace(unit) == "" {
		return fmt.Sprintf("%g", value)
	}
	return fmt.Sprintf("%g%s", value, unit)
}
