package auth

import (
	"encoding/json"
	"sort"
	"time"
)

// Payload is the canonical request body that gets signed by the CLI.
type Payload struct {
	LabID       string
	Timestamp   time.Time
	Nonce       string
	Files       []string
	ContentHash string
}

// NewPayload copies mutable inputs so later caller mutations do not affect signing.
func NewPayload(labID string, timestamp time.Time, nonce string, files []string) Payload {
	copied := append([]string(nil), files...)
	return Payload{
		LabID:     labID,
		Timestamp: timestamp.UTC(),
		Nonce:     nonce,
		Files:     copied,
	}
}

// WithContentHash returns a copy of the payload bound to a submission content hash.
func (p Payload) WithContentHash(contentHash string) Payload {
	p.ContentHash = contentHash
	return p
}

type canonicalPayload struct {
	ContentHash string   `json:"content_hash"`
	Files       []string `json:"files"`
	LabID       string   `json:"lab_id"`
	Nonce       string   `json:"nonce"`
	TimestampMS int64    `json:"timestamp_ms"`
}

var jsonMarshal = json.Marshal

// SigningBytes returns deterministic bytes suitable for Ed25519 signing.
func (p Payload) SigningBytes() ([]byte, error) {
	view := canonicalPayload{
		ContentHash: p.ContentHash,
		Files:       append([]string(nil), p.Files...),
		LabID:       p.LabID,
		Nonce:       p.Nonce,
		TimestampMS: p.Timestamp.UTC().UnixMilli(),
	}
	sort.Strings(view.Files)
	data, err := jsonMarshal(view)
	if err != nil {
		return nil, err
	}
	return data, nil
}
