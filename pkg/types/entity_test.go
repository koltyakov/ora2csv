package types

import (
	"testing"
	"time"
)

func TestEntityState_GetLastRunTime(t *testing.T) {
	tests := []struct {
		name        string
		lastRunTime string
		want        time.Time
		wantErr     bool
	}{
		{
			name:        "valid timestamp",
			lastRunTime: "2025-01-14T10:30:00",
			want:        time.Date(2025, 1, 14, 10, 30, 0, 0, time.UTC),
			wantErr:     false,
		},
		{
			name:        "empty string",
			lastRunTime: "",
			want:        time.Time{},
			wantErr:     false,
		},
		{
			name:        "null string",
			lastRunTime: "null",
			want:        time.Time{},
			wantErr:     false,
		},
		{
			name:        "invalid format",
			lastRunTime: "invalid",
			want:        time.Time{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := EntityState{
				LastRunTime: tt.lastRunTime,
			}
			got, err := e.GetLastRunTime()
			if (err != nil) != tt.wantErr {
				t.Errorf("EntityState.GetLastRunTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("EntityState.GetLastRunTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntityState_SetLastRunTime(t *testing.T) {
	e := EntityState{}
	testTime := time.Date(2025, 1, 14, 10, 30, 0, 0, time.UTC)

	e.SetLastRunTime(testTime)

	if e.LastRunTime != "2025-01-14T10:30:00" {
		t.Errorf("SetLastRunTime() = %s, want 2025-01-14T10:30:00", e.LastRunTime)
	}
}

func TestEntityState_RoundTrip(t *testing.T) {
	e := EntityState{
		Entity:      "test.entity",
		LastRunTime: "",
		Active:      true,
	}

	// Set time
	originalTime := time.Date(2025, 1, 14, 10, 30, 0, 0, time.UTC)
	e.SetLastRunTime(originalTime)

	// Get time back
	retrievedTime, err := e.GetLastRunTime()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !originalTime.Equal(retrievedTime) {
		t.Errorf("got %v, want %v", retrievedTime, originalTime)
	}
}

func TestEntityResult_SuccessProperty(t *testing.T) {
	result := EntityResult{
		Entity:   "test.entity",
		Success:  true,
		RowCount: 100,
		FilePath: "/export/test.entity__2025-01-14.csv",
		Error:    nil,
	}

	if result.Success != true {
		t.Errorf("got Success %v, want true", result.Success)
	}
	if result.Entity != "test.entity" {
		t.Errorf("got Entity %q, want test.entity", result.Entity)
	}
	if result.RowCount != 100 {
		t.Errorf("got RowCount %d, want 100", result.RowCount)
	}
}

func TestEntityResult_FailureProperty(t *testing.T) {
	err := testErr("test error")
	result := EntityResult{
		Entity:   "test.entity",
		Success:  false,
		RowCount: 0,
		Error:    err,
	}

	if result.Success != false {
		t.Errorf("got Success %v, want false", result.Success)
	}
	if result.Error.Error() != "test error" {
		t.Errorf("got error %q, want test error", result.Error.Error())
	}
}

func TestExportResult_Aggregation(t *testing.T) {
	result := ExportResult{
		TotalEntities:  4,
		ProcessedCount: 3,
		SuccessCount:   2,
		FailedCount:    1,
		SkippedCount:   1,
		Results: []EntityResult{
			{Entity: "entity1", Success: true, RowCount: 100},
			{Entity: "entity2", Success: true, RowCount: 200},
			{Entity: "entity3", Success: false, Error: testErr("error")},
		},
	}

	if result.ProcessedCount != 3 {
		t.Errorf("got ProcessedCount %d, want 3", result.ProcessedCount)
	}
	if result.SuccessCount != 2 {
		t.Errorf("got SuccessCount %d, want 2", result.SuccessCount)
	}
	if result.FailedCount != 1 {
		t.Errorf("got FailedCount %d, want 1", result.FailedCount)
	}
	if len(result.Results) != 3 {
		t.Errorf("got %d results, want 3", len(result.Results))
	}
}

// testErr is a simple error for testing
type testErr string

func (e testErr) Error() string {
	return string(e)
}
