package v2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"labkit.local/apps/api/internal/http/middleware"
	labsvc "labkit.local/apps/api/internal/service/labs"
	"labkit.local/packages/go/manifest"
)

type LabsService interface {
	ListPublicLabs(context.Context) ([]labsvc.Lab, error)
	GetPublicLab(context.Context, string) (labsvc.Lab, error)
}

type LabsHandler struct {
	Service LabsService
}

type labResponse struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Manifest          v2Manifest   `json:"manifest"`
	ManifestUpdatedAt time.Time    `json:"manifest_updated_at,omitempty"`
}

type listLabsResponse struct {
	Labs []labResponse `json:"labs"`
}

type v2Manifest struct {
	Lab      v2LabSection      `json:"lab"`
	Submit   v2SubmitSection   `json:"submit"`
	Eval     v2EvalSection     `json:"eval"`
	Quota    v2QuotaSection    `json:"quota"`
	Metrics  []v2MetricSection `json:"metrics"`
	Board    v2BoardSection    `json:"board"`
	Schedule v2ScheduleSection `json:"schedule"`
}

type v2LabSection struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Tags map[string]string `json:"tags,omitempty"`
}

type v2SubmitSection struct {
	Files   []string `json:"files"`
	MaxSize string   `json:"max_size"`
}

type v2EvalSection struct {
	Timeout int `json:"timeout"`
}

type v2QuotaSection struct {
	Daily int      `json:"daily"`
	Free  []string `json:"free"`
}

type v2MetricSection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Sort string `json:"sort"`
	Unit string `json:"unit,omitempty"`
}

type v2BoardSection struct {
	RankBy string `json:"rank_by"`
	Pick   bool   `json:"pick"`
}

type v2ScheduleSection struct {
	Visible time.Time `json:"visible,omitempty"`
	Open    time.Time `json:"open,omitempty"`
	Close   time.Time `json:"close,omitempty"`
}

func (h *LabsHandler) ListLabs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	labs, err := h.Service.ListPublicLabs(r.Context())
	if err != nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	out := make([]labResponse, 0, len(labs))
	for _, lab := range labs {
		out = append(out, toV2LabResponse(lab))
	}
	writeJSON(w, http.StatusOK, listLabsResponse{Labs: out})
}

func (h *LabsHandler) GetLab(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil {
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
		return
	}
	lab, err := h.Service.GetPublicLab(r.Context(), r.PathValue("labID"))
	if err != nil {
		h.writeLabError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toV2LabResponse(lab))
}

func (h *LabsHandler) writeLabError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, labsvc.ErrLabNotFound):
		middleware.WriteError(w, r, http.StatusNotFound, "lab_not_found", "Lab not found")
	default:
		middleware.WriteError(w, r, http.StatusInternalServerError, "internal_server_error", http.StatusText(http.StatusInternalServerError))
	}
}

func toV2LabResponse(lab labsvc.Lab) labResponse {
	return labResponse{
		ID:                lab.ID,
		Name:              lab.Name,
		Manifest:          toV2Manifest(lab.Manifest),
		ManifestUpdatedAt: lab.ManifestUpdatedAt,
	}
}

func toV2Manifest(pub manifest.PublicManifest) v2Manifest {
	metrics := make([]v2MetricSection, 0, len(pub.Metrics))
	for _, metric := range pub.Metrics {
		metrics = append(metrics, v2MetricSection{
			ID:   metric.ID,
			Name: metric.Name,
			Sort: string(metric.Sort),
			Unit: metric.Unit,
		})
	}
	tags := map[string]string(nil)
	if len(pub.Lab.Tags) > 0 {
		tags = map[string]string{}
		for k, v := range pub.Lab.Tags {
			tags[k] = v
		}
	}
	return v2Manifest{
		Lab: v2LabSection{
			ID:   pub.Lab.ID,
			Name: pub.Lab.Name,
			Tags: tags,
		},
		Submit: v2SubmitSection{
			Files:   append([]string(nil), pub.Submit.Files...),
			MaxSize: pub.Submit.MaxSize,
		},
		Eval: v2EvalSection{
			Timeout: pub.Eval.Timeout,
		},
		Quota: v2QuotaSection{
			Daily: pub.Quota.Daily,
			Free:  append([]string(nil), pub.Quota.Free...),
		},
		Metrics: metrics,
		Board: v2BoardSection{
			RankBy: pub.Board.RankBy,
			Pick:   pub.Board.Pick,
		},
		Schedule: v2ScheduleSection{
			Visible: pub.Schedule.Visible,
			Open:    pub.Schedule.Open,
			Close:   pub.Schedule.Close,
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

