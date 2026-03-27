package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileInfo describes a stored env file.
type FileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// Store manages env files on disk.
// Layout: <dataDir>/<project>/<filename>
type Store struct {
	dataDir string
}

// New creates (or opens) a Store backed by dataDir.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Store{dataDir: dataDir}, nil
}

// ListProjects returns all project names.
func (s *Store) ListProjects() ([]string, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, err
	}
	var projects []string
	for _, e := range entries {
		if e.IsDir() {
			projects = append(projects, e.Name())
		}
	}
	return projects, nil
}

// ListFiles returns all files for a project.
func (s *Store) ListFiles(project string) ([]FileInfo, error) {
	dir := filepath.Join(s.dataDir, project)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("project %q not found", project)
	}
	if err != nil {
		return nil, err
	}
	var files []FileInfo
	for _, e := range entries {
		if !e.IsDir() {
			info, _ := e.Info()
			files = append(files, FileInfo{
				Name:    e.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
	}
	return files, nil
}

// GetFile returns the content of a file.
func (s *Store) GetFile(project, filename string) ([]byte, error) {
	path := filepath.Join(s.dataDir, project, filename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("file %q not found in project %q", filename, project)
	}
	return data, err
}

// PutFile writes content to a file, creating the project directory if needed.
func (s *Store) PutFile(project, filename string, content []byte) error {
	if err := s.validateName(project); err != nil {
		return err
	}
	if err := s.validateName(filename); err != nil {
		return err
	}
	dir := filepath.Join(s.dataDir, project)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), content, 0600)
}

// DeleteFile removes a single file.
func (s *Store) DeleteFile(project, filename string) error {
	path := filepath.Join(s.dataDir, project, filename)
	return os.Remove(path)
}

// DeleteProject removes a project and all its files.
func (s *Store) DeleteProject(project string) error {
	dir := filepath.Join(s.dataDir, project)
	return os.RemoveAll(dir)
}

// validateName ensures names don't escape the data directory.
func (s *Store) validateName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}
