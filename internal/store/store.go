package store

import (
	"fmt"
	"log/slog"
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
	log     *slog.Logger
}

// New creates (or opens) a Store backed by dataDir.
func New(dataDir string, logger *slog.Logger) (*Store, error) {
	logger.Debug("initializing store", "dir", dataDir)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		logger.Error("failed to create data directory", "dir", dataDir, "err", err)
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	logger.Debug("store directory ready", "dir", dataDir)
	return &Store{dataDir: dataDir, log: logger}, nil
}

// ListProjects returns all project names.
func (s *Store) ListProjects() ([]string, error) {
	s.log.Debug("listing projects", "dir", s.dataDir)
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		s.log.Error("failed to read data directory", "dir", s.dataDir, "err", err)
		return nil, err
	}
	var projects []string
	for _, e := range entries {
		if e.IsDir() {
			projects = append(projects, e.Name())
		}
	}
	s.log.Debug("projects listed", "count", len(projects))
	return projects, nil
}

// ListFiles returns metadata for all files under a project.
func (s *Store) ListFiles(project string) ([]FileInfo, error) {
	dir := filepath.Join(s.dataDir, project)
	s.log.Debug("listing files", "project", project, "dir", dir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		s.log.Warn("project directory not found", "project", project, "dir", dir)
		return nil, fmt.Errorf("project %q not found", project)
	}
	if err != nil {
		s.log.Error("failed to read project directory", "project", project, "err", err)
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
	s.log.Debug("files listed", "project", project, "count", len(files))
	return files, nil
}

// GetFile returns the content of a stored file.
func (s *Store) GetFile(project, filename string) ([]byte, error) {
	path := filepath.Join(s.dataDir, project, filename)
	s.log.Debug("reading file", "project", project, "file", filename)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		s.log.Warn("file not found", "project", project, "file", filename, "path", path)
		return nil, fmt.Errorf("file %q not found in project %q", filename, project)
	}
	if err != nil {
		s.log.Error("failed to read file", "project", project, "file", filename, "path", path, "err", err)
		return nil, err
	}
	s.log.Debug("file read", "project", project, "file", filename, "bytes", len(data))
	return data, nil
}

// PutFile writes content to a file, creating the project directory if needed.
func (s *Store) PutFile(project, filename string, content []byte) error {
	s.log.Debug("storing file", "project", project, "file", filename, "bytes", len(content))
	if err := s.validateName(project); err != nil {
		s.log.Warn("invalid project name", "project", project, "err", err)
		return err
	}
	if err := s.validateName(filename); err != nil {
		s.log.Warn("invalid filename", "file", filename, "err", err)
		return err
	}
	dir := filepath.Join(s.dataDir, project)
	if err := os.MkdirAll(dir, 0700); err != nil {
		s.log.Error("failed to create project directory", "project", project, "dir", dir, "err", err)
		return err
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, content, 0600); err != nil {
		s.log.Error("failed to write file", "project", project, "file", filename, "path", path, "err", err)
		return err
	}
	s.log.Info("file stored", "project", project, "file", filename, "bytes", len(content))
	return nil
}

// DeleteFile removes a single file.
func (s *Store) DeleteFile(project, filename string) error {
	path := filepath.Join(s.dataDir, project, filename)
	s.log.Debug("deleting file", "project", project, "file", filename, "path", path)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			s.log.Warn("file not found on delete", "project", project, "file", filename)
		} else {
			s.log.Error("failed to delete file", "project", project, "file", filename, "err", err)
		}
		return err
	}
	s.log.Info("file deleted", "project", project, "file", filename)
	return nil
}

// DeleteProject removes a project and all its files.
func (s *Store) DeleteProject(project string) error {
	dir := filepath.Join(s.dataDir, project)
	s.log.Debug("deleting project", "project", project, "dir", dir)
	if err := os.RemoveAll(dir); err != nil {
		s.log.Error("failed to delete project", "project", project, "dir", dir, "err", err)
		return err
	}
	s.log.Info("project deleted", "project", project)
	return nil
}

// validateName ensures names don't escape the data directory.
func (s *Store) validateName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return fmt.Errorf("invalid name %q", name)
	}
	return nil
}
