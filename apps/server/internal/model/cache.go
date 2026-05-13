package model

type CacheOperationRecord struct {
	ID           string   `json:"id"`
	Operation    string   `json:"operation"`
	Scope        string   `json:"scope"`
	Status       string   `json:"status"`
	Message      string   `json:"message"`
	Patterns     []string `json:"patterns"`
	AffectedKeys int64    `json:"affectedKeys"`
	StartedAt    string   `json:"startedAt"`
	FinishedAt   string   `json:"finishedAt"`
	DurationMs   int64    `json:"durationMs"`
}
