package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

func MetricHash(m Metric) string {
	// Deterministic representation of all fields that affect measurement
	canonical := struct {
		ID        string         `json:"id"`
		Traversal string         `json:"traversal"`
		Paths     Paths          `json:"paths"`
		Builtin   string         `json:"builtin,omitempty"`
		Exec      []string       `json:"exec,omitempty"`
		Config    map[string]any `json:"config,omitempty"`
	}{
		ID:        m.ID,
		Traversal: m.Traversal,
		Paths:     m.Paths,
		Builtin:   m.Runner.Builtin,
		Exec:      m.Runner.Exec,
		Config:    m.Runner.Config,
	}

	data, err := json.Marshal(canonical)
	if err != nil {
		// Should never happen with these types
		panic(fmt.Sprintf("failed to marshal metric for hashing: %v", err))
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:16]) // 32 hex chars, plenty for dedup
}
