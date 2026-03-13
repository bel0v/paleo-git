package engine

import "time"

// Status values for Result.
const (
	StatusOK    = "ok"
	StatusError = "error"
)

// Result holds the outcome of running a single metric at a single commit.
// Consumers should check Status before using Value — when Status is
// StatusError, the Error field describes the failure.
type Result struct {
	MetricID   string    `json:"metric_id"`
	MetricHash string    `json:"metric_hash"`
	Commit     string    `json:"commit"`
	AuthorDate time.Time `json:"author_date"`
	Value      int       `json:"value"`
	Files      []string  `json:"files,omitempty"`
	Status     string    `json:"status"`
	DurationMs int       `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
}
