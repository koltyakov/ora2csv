package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koltyakov/ora2csv/internal/storage"
)

type fakeRemoteStore struct {
	exists        bool
	existsErr     error
	data          []byte
	downloadErr   error
	uploadErr     error
	uploaded      []byte
	etag          *string
	nextETag      string
	expectedETags []*string
}

func (f *fakeRemoteStore) Exists(ctx context.Context, key string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeRemoteStore) DownloadBytes(ctx context.Context, key string) ([]byte, error) {
	return f.data, f.downloadErr
}

func (f *fakeRemoteStore) UploadBytes(ctx context.Context, key string, data []byte) error {
	f.uploaded = append([]byte(nil), data...)
	return f.uploadErr
}

func (f *fakeRemoteStore) DownloadBytesVersion(ctx context.Context, key string) ([]byte, *string, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if f.existsErr != nil {
		return nil, nil, f.existsErr
	}
	if f.downloadErr != nil {
		return nil, nil, f.downloadErr
	}
	if !f.exists {
		return nil, nil, nil
	}
	etag := f.etag
	if etag == nil {
		value := `"etag-1"`
		etag = &value
	}
	return f.data, etag, nil
}

func (f *fakeRemoteStore) UploadBytesCAS(ctx context.Context, key string, data []byte, expectedETag *string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var expectedCopy *string
	if expectedETag != nil {
		value := *expectedETag
		expectedCopy = &value
	}
	f.expectedETags = append(f.expectedETags, expectedCopy)
	f.uploaded = append([]byte(nil), data...)
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	if f.nextETag == "" {
		f.nextETag = `"etag-next"`
	}
	return f.nextETag, nil
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s) error: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error: %v", path, err)
	}
}

func TestLoad(t *testing.T) {
	t.Run("valid state file", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")

		testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true}
]`
		mustWriteFile(t, statePath, testState)

		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if st.TotalCount() != 2 {
			t.Errorf("got %d entities, want 2", st.TotalCount())
		}
		if st.ActiveCount() != 2 {
			t.Errorf("got %d active entities, want 2", st.ActiveCount())
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := Load("/nonexistent/state.json", nil, "")
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		mustWriteFile(t, statePath, "invalid json")

		_, err := Load(statePath, nil, "")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("S3 exists error returns error", func(t *testing.T) {
		remote := &fakeRemoteStore{existsErr: errors.New("access denied")}

		_, err := Load("/nonexistent/state.json", remote, "state.json")
		if err == nil {
			t.Fatal("expected error for S3 exists failure, got nil")
		}
		if !strings.Contains(err.Error(), "failed to download S3 state file") {
			t.Fatalf("error = %q, want S3 state context", err.Error())
		}
	})

	t.Run("S3 download error returns error", func(t *testing.T) {
		remote := &fakeRemoteStore{
			exists:      true,
			downloadErr: errors.New("download failed"),
		}

		_, err := Load("/nonexistent/state.json", remote, "state.json")
		if err == nil {
			t.Fatal("expected error for S3 download failure, got nil")
		}
		if !strings.Contains(err.Error(), "failed to download S3 state file") {
			t.Fatalf("error = %q, want S3 download context", err.Error())
		}
	})

	t.Run("S3 missing and local missing starts empty", func(t *testing.T) {
		remote := &fakeRemoteStore{exists: false}

		st, err := Load("/nonexistent/state.json", remote, "state.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.TotalCount() != 0 {
			t.Fatalf("got %d entities, want 0", st.TotalCount())
		}
	})

	t.Run("S3 state downloads and writes local copy", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		remote := &fakeRemoteStore{
			exists: true,
			data:   []byte(`[{"entity":"test.entity1","lastRunTime":"","active":true}]`),
		}

		st, err := Load(statePath, remote, "state.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.TotalCount() != 1 {
			t.Fatalf("got %d entities, want 1", st.TotalCount())
		}
		data, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if string(data) != string(remote.data) {
			t.Fatalf("local state = %q, want %q", string(data), string(remote.data))
		}
	})
}

func TestLoad_StrictValidation(t *testing.T) {
	validEntity := `{"entity":"crm.orders","lastRunTime":"","active":true}`
	tests := []struct {
		name    string
		content string
	}{
		{name: "top-level null", content: `null`},
		{name: "trailing document", content: `[] []`},
		{name: "unknown field", content: `[{"entity":"crm.orders","lastRunTime":"","active":true,"actve":true}]`},
		{name: "missing entity", content: `[{"lastRunTime":"","active":true}]`},
		{name: "missing timestamp", content: `[{"entity":"crm.orders","active":true}]`},
		{name: "missing active", content: `[{"entity":"crm.orders","lastRunTime":""}]`},
		{name: "null field", content: `[{"entity":null,"lastRunTime":"","active":true}]`},
		{name: "empty entity", content: `[{"entity":"","lastRunTime":"","active":true}]`},
		{name: "surrounding whitespace", content: `[{"entity":" crm.orders ","lastRunTime":"","active":true}]`},
		{name: "parent traversal", content: `[{"entity":"../outside","lastRunTime":"","active":true}]`},
		{name: "windows traversal", content: `[{"entity":"..\\outside","lastRunTime":"","active":true}]`},
		{name: "nested entity", content: `[{"entity":"crm/orders","lastRunTime":"","active":true}]`},
		{name: "dot entity", content: `[{"entity":".","lastRunTime":"","active":true}]`},
		{name: "invalid timestamp", content: `[{"entity":"crm.orders","lastRunTime":"yesterday","active":true}]`},
		{name: "duplicate entity", content: `[` + validEntity + `,` + validEntity + `]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath := filepath.Join(tmpDir, "state.json")
			mustWriteFile(t, statePath, tt.content)

			if _, err := Load(statePath, nil, ""); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}

	t.Run("empty array is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		mustWriteFile(t, statePath, `[]`)
		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if st.TotalCount() != 0 {
			t.Fatalf("TotalCount() = %d, want 0", st.TotalCount())
		}
	})
}

func TestLoad_InvalidRemoteDoesNotReplaceLocal(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	localState := `[{"entity":"local.entity","lastRunTime":"","active":true}]`
	mustWriteFile(t, statePath, localState)
	remote := &fakeRemoteStore{exists: true, data: []byte(`[{"entity":"../outside","lastRunTime":"","active":true}]`)}

	if _, err := Load(statePath, remote, "state.json"); err == nil {
		t.Fatal("Load() error = nil, want invalid remote state error")
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != localState {
		t.Fatalf("local state was replaced: got %q, want %q", data, localState)
	}
}

func TestLoadContext_PropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	remote := &fakeRemoteStore{exists: true, data: []byte(`[]`)}
	if _, err := LoadContext(ctx, filepath.Join(t.TempDir(), "state.json"), remote, "state.json"); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadContext() error = %v, want context.Canceled", err)
	}
}

func TestGetEntities(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true},
  {"entity":"test.entity3","lastRunTime":"2025-01-01T00:00:00","active":false}
]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entities := st.GetEntities()
	if len(entities) != 3 {
		t.Errorf("got %d entities, want 3", len(entities))
	}

	// Verify returned copy is independent
	entities[0].Entity = "modified"
	original := st.GetEntities()
	if original[0].Entity == "modified" {
		t.Error("GetEntities() returned same slice, not a copy")
	}
}

func TestGetActiveEntities(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true},
  {"entity":"test.entity3","lastRunTime":"2025-01-01T00:00:00","active":false}
]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	active := st.GetActiveEntities()
	if len(active) != 2 {
		t.Errorf("got %d active entities, want 2", len(active))
	}

	for _, e := range active {
		if !e.Active {
			t.Errorf("expected all active entities to have Active=true, got %v", e)
		}
	}
}

func TestFindEntity(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true},
  {"entity":"test.entity3","lastRunTime":"2025-01-01T00:00:00","active":false}
]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("entity exists", func(t *testing.T) {
		entity, found := st.FindEntity("test.entity1")
		if !found {
			t.Error("expected entity to be found")
		}
		if entity.Entity != "test.entity1" {
			t.Errorf("got entity %q, want test.entity1", entity.Entity)
		}
		if !entity.Active {
			t.Error("expected entity to be active")
		}
	})

	t.Run("returned entity is a copy", func(t *testing.T) {
		entity, found := st.FindEntity("test.entity1")
		if !found {
			t.Fatal("expected entity to be found")
		}
		entity.Entity = "modified"
		stored, found := st.FindEntity("test.entity1")
		if !found || stored.Entity != "test.entity1" {
			t.Fatal("FindEntity() exposed mutable internal state")
		}
	})

	t.Run("entity not found", func(t *testing.T) {
		_, found := st.FindEntity("nonexistent")
		if found {
			t.Error("expected entity not to be found")
		}
	})
}

func TestUpdateEntityTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true}
]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = st.UpdateEntityTimestamp("test.entity1", "2025-01-15T12:00:00")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify state was persisted
	st2, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entity, found := st2.FindEntity("test.entity1")
	if !found {
		t.Fatal("entity not found")
	}
	if entity.LastRunTime != "2025-01-15T12:00:00" {
		t.Errorf("got lastRunTime %q, want 2025-01-15T12:00:00", entity.LastRunTime)
	}
}

func TestUpdateEntityTimestamp_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[{"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true}]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = st.UpdateEntityTimestamp("nonexistent", "2025-01-15T12:00:00")
	if err == nil {
		t.Error("expected error for nonexistent entity, got nil")
	}
}

func TestUpdateEntityTimestamp_RemoteFailureDoesNotLeak(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"2025-01-01T00:00:00","active":true}
]`
	mustWriteFile(t, statePath, testState)
	remote := &fakeRemoteStore{uploadErr: errors.New("upload failed")}
	st, err := Load(statePath, remote, "state.json")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := st.UpdateEntityTimestamp("test.entity1", "2025-01-02T00:00:00"); err == nil {
		t.Fatal("UpdateEntityTimestamp() error = nil, want upload failure")
	}
	entity, _ := st.FindEntity("test.entity1")
	if entity.LastRunTime != "2025-01-01T00:00:00" {
		t.Fatalf("failed update changed memory to %q", entity.LastRunTime)
	}

	remote.uploadErr = nil
	if err := st.UpdateEntityTimestamp("test.entity2", "2025-01-03T00:00:00"); err != nil {
		t.Fatalf("second UpdateEntityTimestamp() error: %v", err)
	}
	var uploaded []struct {
		Entity      string `json:"entity"`
		LastRunTime string `json:"lastRunTime"`
	}
	if err := json.Unmarshal(remote.uploaded, &uploaded); err != nil {
		t.Fatalf("Unmarshal(uploaded) error: %v", err)
	}
	if uploaded[0].LastRunTime != "2025-01-01T00:00:00" || uploaded[1].LastRunTime != "2025-01-03T00:00:00" {
		t.Fatalf("uploaded state leaked failed update: %+v", uploaded)
	}
}

func TestUpdateEntityTimestamp_LocalFailureDoesNotMutateMemory(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	mustWriteFile(t, statePath, `[{"entity":"test.entity","lastRunTime":"2025-01-01T00:00:00","active":true}]`)
	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	st.path = filepath.Join(tmpDir, "missing", "state.json")

	if err := st.UpdateEntityTimestamp("test.entity", "2025-01-02T00:00:00"); err == nil {
		t.Fatal("UpdateEntityTimestamp() error = nil, want local save failure")
	}
	entity, _ := st.FindEntity("test.entity")
	if entity.LastRunTime != "2025-01-01T00:00:00" {
		t.Fatalf("failed update changed memory to %q", entity.LastRunTime)
	}
}

func TestAdvanceEntityTimestamp_IsMonotonic(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	mustWriteFile(t, statePath, `[{"entity":"test.entity","lastRunTime":"2025-01-02T00:00:00","active":true}]`)
	remote := &fakeRemoteStore{}
	st, err := Load(statePath, remote, "state.json")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	for _, candidate := range []time.Time{
		time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		advanced, err := st.AdvanceEntityTimestamp("test.entity", candidate)
		if err != nil {
			t.Fatalf("AdvanceEntityTimestamp() error: %v", err)
		}
		if advanced {
			t.Fatalf("AdvanceEntityTimestamp(%v) advanced state", candidate)
		}
	}
	if remote.uploaded != nil {
		t.Fatal("non-forward updates uploaded state")
	}
	entity, _ := st.FindEntity("test.entity")
	if entity.LastRunTime != "2025-01-02T00:00:00" {
		t.Fatalf("lastRunTime = %q", entity.LastRunTime)
	}
}

func TestAdvanceEntityTimestamp_ConcurrentUpdatesKeepMaximum(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	mustWriteFile(t, statePath, `[{"entity":"test.entity","lastRunTime":"2025-01-01T00:00:00","active":true}]`)
	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	candidates := []time.Time{
		time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
	}
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.AdvanceEntityTimestamp("test.entity", candidate); err != nil {
				t.Errorf("AdvanceEntityTimestamp() error: %v", err)
			}
		}()
	}
	wg.Wait()
	entity, _ := st.FindEntity("test.entity")
	if entity.LastRunTime != "2025-01-03T00:00:00" {
		t.Fatalf("lastRunTime = %q, want maximum candidate", entity.LastRunTime)
	}
}

func TestAdvanceEntityTimestamp_UsesRemoteETagCAS(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	initialETag := `"etag-1"`
	remote := &fakeRemoteStore{
		exists:   true,
		data:     []byte(`[{"entity":"test.entity","lastRunTime":"2025-01-01T00:00:00","active":true}]`),
		etag:     &initialETag,
		nextETag: `"etag-2"`,
	}
	st, err := Load(statePath, remote, "state.json")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	advanced, err := st.AdvanceEntityTimestamp("test.entity", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil || !advanced {
		t.Fatalf("first AdvanceEntityTimestamp() = (%t, %v)", advanced, err)
	}
	remote.nextETag = `"etag-3"`
	advanced, err = st.AdvanceEntityTimestamp("test.entity", time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC))
	if err != nil || !advanced {
		t.Fatalf("second AdvanceEntityTimestamp() = (%t, %v)", advanced, err)
	}
	if len(remote.expectedETags) != 2 || remote.expectedETags[0] == nil || *remote.expectedETags[0] != `"etag-1"` || remote.expectedETags[1] == nil || *remote.expectedETags[1] != `"etag-2"` {
		t.Fatalf("expected ETags = %v", remote.expectedETags)
	}
}

func TestAdvanceEntityTimestamp_CASConflictPreservesState(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	initial := `[{"entity":"test.entity","lastRunTime":"2025-01-01T00:00:00","active":true}]`
	mustWriteFile(t, statePath, initial)
	remote := &fakeRemoteStore{uploadErr: storage.ErrCASConflict}
	st, err := Load(statePath, remote, "state.json")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	advanced, err := st.AdvanceEntityTimestamp("test.entity", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
	if advanced || !errors.Is(err, storage.ErrCASConflict) {
		t.Fatalf("AdvanceEntityTimestamp() = (%t, %v), want CAS conflict", advanced, err)
	}
	entity, _ := st.FindEntity("test.entity")
	if entity.LastRunTime != "2025-01-01T00:00:00" {
		t.Fatalf("memory advanced to %q", entity.LastRunTime)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != initial {
		t.Fatalf("local state changed to %q", data)
	}
	if len(remote.expectedETags) != 1 || remote.expectedETags[0] != nil {
		t.Fatalf("create CAS expected ETag = %v, want nil", remote.expectedETags)
	}
}

func TestAdvanceEntityTimestampContext_PropagatesCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	mustWriteFile(t, statePath, `[{"entity":"test.entity","lastRunTime":"2025-01-01T00:00:00","active":true}]`)
	remote := &fakeRemoteStore{}
	st, err := Load(statePath, remote, "state.json")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	advanced, err := st.AdvanceEntityTimestampContext(ctx, "test.entity", time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC))
	if advanced || !errors.Is(err, context.Canceled) {
		t.Fatalf("AdvanceEntityTimestampContext() = (%t, %v)", advanced, err)
	}
	if len(remote.expectedETags) != 0 {
		t.Fatal("cancelled update contacted remote storage")
	}
}

func TestValidateSQLFiles(t *testing.T) {
	t.Run("all files exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		sqlDir := filepath.Join(tmpDir, "sql")

		testState := `[
  {"entity":"test.entity1","lastRunTime":"","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true}
]`
		mustWriteFile(t, statePath, testState)
		mustMkdirAll(t, sqlDir)
		mustWriteFile(t, filepath.Join(sqlDir, "test.entity1.sql"), "SELECT 1")
		mustWriteFile(t, filepath.Join(sqlDir, "test.entity2.sql"), "SELECT 2")

		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = st.ValidateSQLFiles(sqlDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing files", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		sqlDir := filepath.Join(tmpDir, "sql")

		testState := `[{"entity":"test.entity1","lastRunTime":"","active":true}]`
		mustWriteFile(t, statePath, testState)
		mustMkdirAll(t, sqlDir)
		// Don't create SQL files

		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = st.ValidateSQLFiles(sqlDir)
		if err == nil {
			t.Error("expected error for missing SQL files, got nil")
		}
	})

	t.Run("inactive entity missing file is ok", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		sqlDir := filepath.Join(tmpDir, "sql")

		// Only create SQL for active entities
		testState := `[
  {"entity":"test.active1","lastRunTime":"","active":true},
  {"entity":"test.inactive1","lastRunTime":"","active":false}
]`
		mustWriteFile(t, statePath, testState)
		mustMkdirAll(t, sqlDir)
		mustWriteFile(t, filepath.Join(sqlDir, "test.active1.sql"), "SELECT 1")

		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = st.ValidateSQLFiles(sqlDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("directory is rejected as SQL file", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")
		sqlDir := filepath.Join(tmpDir, "sql")
		mustWriteFile(t, statePath, `[{"entity":"test.entity","lastRunTime":"","active":true}]`)
		mustMkdirAll(t, filepath.Join(sqlDir, "test.entity.sql"))
		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if err := st.ValidateSQLFiles(sqlDir); err == nil {
			t.Fatal("ValidateSQLFiles() error = nil, want non-regular file error")
		}
	})
}

func TestGetSQLPath(t *testing.T) {
	st := &File{}
	path, err := st.GetSQLPath("/app/sql", "test.entity")
	if err != nil {
		t.Fatalf("GetSQLPath() error: %v", err)
	}
	expected := "/app/sql/test.entity.sql"
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
	}
	if _, err := st.GetSQLPath("/app/sql", "../outside"); err == nil {
		t.Fatal("GetSQLPath() error = nil, want traversal error")
	}
}

func TestTotalCount(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true},
  {"entity":"test.entity3","lastRunTime":"","active":false}
]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if st.TotalCount() != 3 {
		t.Errorf("got %d, want 3", st.TotalCount())
	}
}

func TestActiveCount(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true},
  {"entity":"test.entity3","lastRunTime":"","active":false}
]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if st.ActiveCount() != 2 {
		t.Errorf("got %d, want 2", st.ActiveCount())
	}
}

func TestSave_SortsEntities(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create state with unsorted entities
	unsortedState := `[
  {"entity":"zebra","lastRunTime":"","active":true},
  {"entity":"alpha","lastRunTime":"","active":true},
  {"entity":"beta","lastRunTime":"","active":true}
]`
	mustWriteFile(t, statePath, unsortedState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Update should sort
	err = st.UpdateEntityTimestamp("alpha", "2025-01-01T00:00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reload and verify entities are still there and sorted
	st2, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entities := st2.GetEntities()
	if len(entities) != 3 {
		t.Fatalf("got %d entities, want 3", len(entities))
	}

	// Check they are sorted alphabetically
	if entities[0].Entity != "alpha" {
		t.Errorf("first entity is %q, want alpha", entities[0].Entity)
	}
	if entities[1].Entity != "beta" {
		t.Errorf("second entity is %q, want beta", entities[1].Entity)
	}
	if entities[2].Entity != "zebra" {
		t.Errorf("third entity is %q, want zebra", entities[2].Entity)
	}
}

func TestSave_Atomic(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[{"entity":"test.entity1","lastRunTime":"","active":true}]`
	mustWriteFile(t, statePath, testState)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Update multiple times - should not lose data
	for i := 0; i < 10; i++ {
		err = st.UpdateEntityTimestamp("test.entity1", "2025-01-15T12:00:00")
		if err != nil {
			t.Errorf("unexpected error on iteration %d: %v", i, err)
		}
	}

	// Final state should be valid
	st2, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st2.TotalCount() != 1 {
		t.Errorf("got %d entities, want 1", st2.TotalCount())
	}
}

func TestSave_UsesUniqueTemporaryFileAndPreservesMode(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	mustWriteFile(t, statePath, `[{"entity":"test.entity","lastRunTime":"","active":true}]`)
	if err := os.Chmod(statePath, 0600); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}
	legacyTempPath := statePath + ".tmp"
	mustMkdirAll(t, legacyTempPath)
	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := st.UpdateEntityTimestamp("test.entity", "2025-01-01T00:00:00"); err != nil {
		t.Fatalf("UpdateEntityTimestamp() error: %v", err)
	}
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("state mode = %o, want 600", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(tmpDir, ".state.json.tmp-*"))
	if err != nil {
		t.Fatalf("Glob() error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
