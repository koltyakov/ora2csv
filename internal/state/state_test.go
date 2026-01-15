package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("valid state file", func(t *testing.T) {
		tmpDir := t.TempDir()
		statePath := filepath.Join(tmpDir, "state.json")

		testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true}
]`
		os.WriteFile(statePath, []byte(testState), 0644)

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
		os.WriteFile(statePath, []byte("invalid json"), 0644)

		_, err := Load(statePath, nil, "")
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})
}

func TestGetEntities(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	testState := `[
  {"entity":"test.entity1","lastRunTime":"2025-01-01T00:00:00","active":true},
  {"entity":"test.entity2","lastRunTime":"","active":true},
  {"entity":"test.entity3","lastRunTime":"2025-01-01T00:00:00","active":false}
]`
	os.WriteFile(statePath, []byte(testState), 0644)

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
	os.WriteFile(statePath, []byte(testState), 0644)

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
	os.WriteFile(statePath, []byte(testState), 0644)

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
	os.WriteFile(statePath, []byte(testState), 0644)

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
	os.WriteFile(statePath, []byte(testState), 0644)

	st, err := Load(statePath, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = st.UpdateEntityTimestamp("nonexistent", "2025-01-15T12:00:00")
	if err == nil {
		t.Error("expected error for nonexistent entity, got nil")
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
		os.WriteFile(statePath, []byte(testState), 0644)
		os.MkdirAll(sqlDir, 0755)
		os.WriteFile(filepath.Join(sqlDir, "test.entity1.sql"), []byte("SELECT 1"), 0644)
		os.WriteFile(filepath.Join(sqlDir, "test.entity2.sql"), []byte("SELECT 2"), 0644)

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
		os.WriteFile(statePath, []byte(testState), 0644)
		os.MkdirAll(sqlDir, 0755)
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
		os.WriteFile(statePath, []byte(testState), 0644)
		os.MkdirAll(sqlDir, 0755)
		os.WriteFile(filepath.Join(sqlDir, "test.active1.sql"), []byte("SELECT 1"), 0644)

		st, err := Load(statePath, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = st.ValidateSQLFiles(sqlDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestGetSQLPath(t *testing.T) {
	st := &File{}
	path := st.GetSQLPath("/app/sql", "test.entity")
	expected := "/app/sql/test.entity.sql"
	if path != expected {
		t.Errorf("got %q, want %q", path, expected)
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
	os.WriteFile(statePath, []byte(testState), 0644)

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
	os.WriteFile(statePath, []byte(testState), 0644)

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
	os.WriteFile(statePath, []byte(unsortedState), 0644)

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
	os.WriteFile(statePath, []byte(testState), 0644)

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
