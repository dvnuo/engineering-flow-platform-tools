package automation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Store struct {
	RootDir string
}

func NewStore(root string) *Store {
	return &Store{RootDir: filepath.Clean(expandHome(root))}
}

func DefaultStore() (*Store, error) {
	root, err := DefaultBrowserHome()
	if err != nil {
		return nil, err
	}
	return NewStore(root), nil
}

func (s *Store) SessionsDir() string {
	return filepath.Join(s.RootDir, "sessions")
}

func (s *Store) ProfilesDir() string {
	return filepath.Join(s.RootDir, "profiles")
}

func (s *Store) LocksDir() string {
	return filepath.Join(s.RootDir, "locks")
}

func (s *Store) MetadataPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := ValidateSessionName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.SessionsDir(), name+".json"), nil
}

func (s *Store) SessionLockPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if err := ValidateSessionName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.LocksDir(), name+".lock"), nil
}

func (s *Store) SessionPath(name string) (string, error) {
	return s.MetadataPath(name)
}

func (s *Store) Save(session Session) error {
	if err := ValidateSessionName(session.Name); err != nil {
		return err
	}
	path, err := s.MetadataPath(session.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Check permissions for ~/.efp/browser/sessions.", 500)
	}
	session.MetadataPath = path
	b, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return NewError("automation_failed", err.Error(), "Session metadata could not be encoded.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return NewError("artifact_write_failed", err.Error(), "Check permissions for ~/.efp/browser/sessions.", 500)
	}
	return nil
}

func (s *Store) Load(name string) (Session, error) {
	path, err := s.MetadataPath(name)
	if err != nil {
		return Session{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, NewError("session_not_found", "Browser session was not found.", "Run browser session list --json or browser session start --json.", 404)
	}
	if err != nil {
		return Session{}, NewError("automation_failed", err.Error(), "Session metadata could not be read.", 500)
	}
	var session Session
	if err := json.Unmarshal(b, &session); err != nil {
		return Session{}, NewError("automation_failed", err.Error(), "Session metadata is not valid JSON.", 500)
	}
	if session.Name == "" {
		session.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	session.MetadataPath = path
	return session, nil
}

func (s *Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.SessionsDir())
	if errors.Is(err, os.ErrNotExist) {
		return []Session{}, nil
	}
	if err != nil {
		return nil, NewError("automation_failed", err.Error(), "Session metadata directory could not be read.", 500)
	}
	out := make([]Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		session, err := s.Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *Store) Remove(name string) error {
	path, err := s.MetadataPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return NewError("artifact_write_failed", err.Error(), "Session metadata could not be removed.", 500)
	}
	return nil
}

func (s *Store) Delete(name string) error {
	return s.Remove(name)
}
