package types

import "time"

// EntityState represents the state of a single entity from state.json
type EntityState struct {
	Entity      string `json:"entity"`
	LastRunTime string `json:"lastRunTime"` // ISO 8601 format
	Active      bool   `json:"active"`
}

// GetLastRunTime parses the LastRunTime string into a time.Time
// Returns zero time if LastRunTime is empty or "null"
func (e *EntityState) GetLastRunTime() (time.Time, error) {
	if e.LastRunTime == "" || e.LastRunTime == "null" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02T15:04:05", e.LastRunTime)
}

// SetLastRunTime sets the LastRunTime from a time.Time
func (e *EntityState) SetLastRunTime(t time.Time) {
	e.LastRunTime = t.Format("2006-01-02T15:04:05")
}

// EntityResult represents the result of processing a single entity
type EntityResult struct {
	Entity   string
	Success  bool
	RowCount int
	FilePath string
	Error    error
	Duration time.Duration
}

// ExportResult represents the overall result of an export run
type ExportResult struct {
	TotalEntities  int
	ProcessedCount int
	SuccessCount   int
	FailedCount    int
	SkippedCount   int
	Results        []EntityResult
	Duration       time.Duration
}
