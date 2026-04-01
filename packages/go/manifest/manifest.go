package manifest

import (
	"bytes"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
)

// Manifest is the full lab manifest contract.
type Manifest struct {
	Lab      LabSection      `toml:"lab"`
	Submit   SubmitSection   `toml:"submit"`
	Eval     EvalSection     `toml:"eval"`
	Quota    QuotaSection    `toml:"quota"`
	Metrics  []MetricSection `toml:"metric"`
	Board    BoardSection    `toml:"board"`
	Schedule ScheduleSection `toml:"schedule"`
}

// LabSection describes the lab identity.
type LabSection struct {
	ID   string            `toml:"id"`
	Name string            `toml:"name"`
	Tags map[string]string `toml:"tags"`
}

// SubmitSection describes expected submission files.
type SubmitSection struct {
	Files   []string `toml:"files"`
	MaxSize string   `toml:"max_size"`
}

// EvalSection describes evaluator runtime configuration.
type EvalSection struct {
	Image   string `toml:"image"`
	Timeout int    `toml:"timeout"`
}

// QuotaSection describes daily quota rules.
type QuotaSection struct {
	Daily int      `toml:"daily"`
	Free  []string `toml:"free"`
}

// MetricSection describes one ranking metric.
type MetricSection struct {
	ID   string     `toml:"id"`
	Name string     `toml:"name"`
	Sort MetricSort `toml:"sort"`
	Unit string     `toml:"unit"`
}

// MetricSort determines whether higher or lower values rank first.
type MetricSort string

const (
	MetricSortAsc  MetricSort = "asc"
	MetricSortDesc MetricSort = "desc"
)

// BoardSection configures ranking behavior.
type BoardSection struct {
	RankBy string `toml:"rank_by"`
	Pick   bool   `toml:"pick"`
}

// ScheduleSection configures the time window.
type ScheduleSection struct {
	Visible time.Time `toml:"visible"`
	Open    time.Time `toml:"open"`
	Close   time.Time `toml:"close"`
}

// Parse decodes, normalizes, and validates a manifest.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if _, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	m.normalize()
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) normalize() {
	if m.Lab.Tags == nil {
		m.Lab.Tags = map[string]string{}
	}
	if m.Submit.MaxSize == "" {
		m.Submit.MaxSize = "1MB"
	}
	if m.Eval.Timeout == 0 {
		m.Eval.Timeout = 300
	}
	if m.Quota.Free == nil {
		m.Quota.Free = []string{}
	}
	for i := range m.Metrics {
		if m.Metrics[i].Name == "" {
			m.Metrics[i].Name = m.Metrics[i].ID
		}
	}
	if m.Board.RankBy == "" && len(m.Metrics) > 0 {
		m.Board.RankBy = m.Metrics[0].ID
	}
	if m.Schedule.Visible.IsZero() {
		m.Schedule.Visible = m.Schedule.Open
	}
}
