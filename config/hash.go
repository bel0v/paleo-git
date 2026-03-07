package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// MetricHash returns a deterministic hash of the metric definition.
// It changes when any field that affects measurement results changes:
// id, traversal, paths, runner identity, or runner config.
// Consumers can use this to detect config drift and invalidate cached results.
func MetricHash(m Metric) string {
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
		panic(fmt.Sprintf("failed to marshal metric for hashing: %v", err))
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:16])
}
