package store_test

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/arashthr/envault/internal/store"
)

// newStore creates a Store backed by a temp directory that is cleaned up
// automatically when the test ends.
func newStore(t *testing.T) *store.Store {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.New(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

// ── ListProjects ──────────────────────────────────────────────────────────────

func TestListProjects_empty(t *testing.T) {
	s := newStore(t)
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestListProjects(t *testing.T) {
	s := newStore(t)
	mustPut(t, s, "alpha", ".env", "A=1")
	mustPut(t, s, "beta", ".env", "B=2")
	mustPut(t, s, "gamma", ".env", "C=3")

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d: %v", len(projects), projects)
	}
	want := map[string]bool{"alpha": true, "beta": true, "gamma": true}
	for _, p := range projects {
		if !want[p] {
			t.Errorf("unexpected project %q", p)
		}
	}
}

// ── PutFile / GetFile ─────────────────────────────────────────────────────────

func TestPutAndGetFile_roundTrip(t *testing.T) {
	s := newStore(t)
	content := "DATABASE_URL=postgres://localhost/test\nSECRET=abc123\n"
	if err := s.PutFile("myapp", ".env", []byte(content)); err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	got, err := s.GetFile("myapp", ".env")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if string(got) != content {
		t.Errorf("content mismatch\n got: %q\nwant: %q", got, content)
	}
}

func TestPutFile_overwrite(t *testing.T) {
	s := newStore(t)
	mustPut(t, s, "proj", ".env", "V=1")
	mustPut(t, s, "proj", ".env", "V=2") // overwrite

	got, err := s.GetFile("proj", ".env")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "V=2" {
		t.Errorf("expected overwritten content, got %q", got)
	}
}

func TestPutFile_emptyContent(t *testing.T) {
	s := newStore(t)
	if err := s.PutFile("proj", ".env", []byte{}); err != nil {
		t.Fatalf("PutFile with empty content: %v", err)
	}
	got, err := s.GetFile("proj", ".env")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty content, got %q", got)
	}
}

func TestGetFile_notFound(t *testing.T) {
	s := newStore(t)
	_, err := s.GetFile("noproject", ".env")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// ── PutFile name validation ───────────────────────────────────────────────────

func TestPutFile_invalidProjectName(t *testing.T) {
	s := newStore(t)
	cases := []string{"", ".", "..", "a/b", "../evil"}
	for _, name := range cases {
		if err := s.PutFile(name, ".env", []byte("x")); err == nil {
			t.Errorf("PutFile with project=%q: expected error, got nil", name)
		}
	}
}

func TestPutFile_invalidFileName(t *testing.T) {
	s := newStore(t)
	cases := []string{"", ".", "..", "a/b", "../evil"}
	for _, name := range cases {
		if err := s.PutFile("proj", name, []byte("x")); err == nil {
			t.Errorf("PutFile with file=%q: expected error, got nil", name)
		}
	}
}

// ── ListFiles ─────────────────────────────────────────────────────────────────

func TestListFiles(t *testing.T) {
	s := newStore(t)
	mustPut(t, s, "proj", ".env", "A=1")
	mustPut(t, s, "proj", ".env.staging", "A=staging")

	files, err := s.ListFiles("proj")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	names := map[string]bool{}
	for _, f := range files {
		names[f.Name] = true
		if f.Size == 0 && f.Name == ".env" {
			t.Errorf("expected non-zero size for .env")
		}
	}
	if !names[".env"] || !names[".env.staging"] {
		t.Errorf("unexpected file list: %v", names)
	}
}

func TestListFiles_projectNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.ListFiles("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent project, got nil")
	}
}

func TestListFiles_empty(t *testing.T) {
	s := newStore(t)
	// Create a project directory explicitly by writing and then deleting a file.
	mustPut(t, s, "proj", ".env", "X=1")
	if err := s.DeleteFile("proj", ".env"); err != nil {
		t.Fatal(err)
	}
	files, err := s.ListFiles("proj")
	if err != nil {
		t.Fatalf("ListFiles on empty project: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

// ── DeleteFile ────────────────────────────────────────────────────────────────

func TestDeleteFile(t *testing.T) {
	s := newStore(t)
	mustPut(t, s, "proj", ".env", "A=1")

	if err := s.DeleteFile("proj", ".env"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, err := s.GetFile("proj", ".env"); err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestDeleteFile_notFound(t *testing.T) {
	s := newStore(t)
	err := s.DeleteFile("noproj", ".env")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// ── DeleteProject ─────────────────────────────────────────────────────────────

func TestDeleteProject(t *testing.T) {
	s := newStore(t)
	mustPut(t, s, "proj", ".env", "A=1")
	mustPut(t, s, "proj", ".env.staging", "A=2")

	if err := s.DeleteProject("proj"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	projects, _ := s.ListProjects()
	for _, p := range projects {
		if p == "proj" {
			t.Error("project still present after delete")
		}
	}
}

func TestDeleteProject_nonexistent(t *testing.T) {
	s := newStore(t)
	// RemoveAll on a missing path is not an error.
	if err := s.DeleteProject("ghost"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── FileInfo fields ───────────────────────────────────────────────────────────

func TestFileInfo_fields(t *testing.T) {
	s := newStore(t)
	content := "SECRET=hello"
	mustPut(t, s, "proj", ".env", content)

	files, err := s.ListFiles("proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Name != ".env" {
		t.Errorf("name = %q, want .env", f.Name)
	}
	if f.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", f.Size, len(content))
	}
	if f.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
}

// ── New: creates dir if missing ───────────────────────────────────────────────

func TestNew_createsDir(t *testing.T) {
	dir := t.TempDir() + "/new/nested/dir"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := store.New(dir, logger)
	if err != nil {
		t.Fatalf("store.New with nested path: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("expected directory %q to be created", dir)
	}
}

// ── helper ────────────────────────────────────────────────────────────────────

func mustPut(t *testing.T, s *store.Store, project, file, content string) {
	t.Helper()
	if err := s.PutFile(project, file, []byte(content)); err != nil {
		t.Fatalf("mustPut(%q, %q): %v", project, file, err)
	}
}
