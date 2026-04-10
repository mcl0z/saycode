package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"saycoding/internal/config"
	"saycoding/internal/types"
)

type Store struct {
	root string
}

func NewStore() (*Store, error) {
	root, err := config.RootDir()
	if err != nil {
		return nil, err
	}
	sessionsDir := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) New(cwd string) *types.Session {
	now := time.Now().UTC()
	return &types.Session{
		ID:        fmt.Sprintf("%s-%04d", now.Format("20060102-150405"), now.Nanosecond()%10000),
		CWD:       cwd,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []types.Message{},
	}
}

func (s *Store) Save(sess *types.Session) error {
	sess.UpdatedAt = time.Now().UTC()
	path := filepath.Join(s.root, "sessions", sess.ID+".json")
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return s.saveIndex(sess)
}

func (s *Store) Load(id string) (*types.Session, error) {
	path := filepath.Join(s.root, "sessions", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sess types.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) List() ([]types.SessionIndexEntry, error) {
	path := filepath.Join(s.root, "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, nil
	}
	var idx []types.SessionIndexEntry
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, nil
	}
	sort.Slice(idx, func(i, j int) bool {
		return idx[i].UpdatedAt.After(idx[j].UpdatedAt)
	})
	return idx, nil
}

func (s *Store) Latest() (*types.SessionIndexEntry, error) {
	idx, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(idx) == 0 {
		return nil, nil
	}
	return &idx[0], nil
}

func (s *Store) saveIndex(sess *types.Session) error {
	idx, _ := s.List()
	entry := types.SessionIndexEntry{
		ID:        sess.ID,
		CWD:       sess.CWD,
		UpdatedAt: sess.UpdatedAt,
	}
	next := []types.SessionIndexEntry{entry}
	for _, item := range idx {
		if item.ID != sess.ID {
			next = append(next, item)
		}
		if len(next) >= 50 {
			break
		}
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, "index.json"), data, 0o600)
}
