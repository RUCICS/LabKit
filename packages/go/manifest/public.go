package manifest

// PublicManifest is the unauthenticated projection of a manifest.
type PublicManifest struct {
	Lab      LabSection      `toml:"lab"`
	Submit   SubmitSection   `toml:"submit"`
	Eval     EvalSection     `toml:"eval"`
	Quota    QuotaSection    `toml:"quota"`
	Metrics  []MetricSection `toml:"metric"`
	Board    BoardSection    `toml:"board"`
	Schedule ScheduleSection `toml:"schedule"`
}

// Public returns the client-facing manifest projection.
func (m *Manifest) Public() PublicManifest {
	pub := PublicManifest{
		Lab:      m.Lab,
		Submit:   m.Submit,
		Eval:     m.Eval,
		Quota:    m.Quota,
		Metrics:  make([]MetricSection, len(m.Metrics)),
		Board:    m.Board,
		Schedule: m.Schedule,
	}
	pub.Eval.Image = ""
	if m.Lab.Tags != nil {
		pub.Lab.Tags = map[string]string{}
		for k, v := range m.Lab.Tags {
			pub.Lab.Tags[k] = v
		}
	}
	if m.Submit.Files != nil {
		pub.Submit.Files = append([]string(nil), m.Submit.Files...)
	}
	if m.Quota.Free != nil {
		pub.Quota.Free = append([]string(nil), m.Quota.Free...)
	}
	copy(pub.Metrics, m.Metrics)
	return pub
}
