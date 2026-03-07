package engine

import "time"

const (
	StatusOK    = "ok"
	StatusError = "error"
)

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
