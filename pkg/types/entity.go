package types

import "time"

// EntityState represents the state of a single entity from state.json
type EntityState struct {
	Entity      string `json:"entity"`
	LastRunTime string `json:"lastRunTime"` // ISO 8601 format
	Active      bool   `json:"active"`
}

// GetLastRunTime parses the LastRunTime string into a time.Time (UTC)
// Returns zero time if LastRunTime is empty or "null"
func (e *EntityState) GetLastRunTime() (time.Time, error) {
	if e.LastRunTime == "" || e.LastRunTime == "null" {
		return time.Time{}, nil
	}
	return time.ParseInLocation("2006-01-02T15:04:05", e.LastRunTime, time.UTC)
}

// SetLastRunTime sets the LastRunTime from a time.Time (formats as UTC)
func (e *EntityState) SetLastRunTime(t time.Time) {
	e.LastRunTime = t.UTC().Format("2006-01-02T15:04:05")
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
