package todo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"saycoding/internal/config"
)

type Item struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	root, err := config.RootDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(root, "todo.json")}, nil
}

func (s *Store) List() ([]Item, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) Add(text string) ([]Item, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	nextID := 1
	for _, item := range items {
		if item.ID >= nextID {
			nextID = item.ID + 1
		}
	}
	items = append(items, Item{
		ID:        nextID,
		Text:      text,
		UpdatedAt: time.Now().UTC(),
	})
	return items, s.save(items)
}

func (s *Store) SetDone(id int, done bool) ([]Item, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			items[i].Done = done
			items[i].UpdatedAt = time.Now().UTC()
			return items, s.save(items)
		}
	}
	return nil, fmt.Errorf("todo %d not found", id)
}

func (s *Store) Remove(id int) ([]Item, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(items))
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			continue
		}
		out = append(out, item)
	}
	if !found {
		return nil, fmt.Errorf("todo %d not found", id)
	}
	return out, s.save(out)
}

func (s *Store) Clear() error {
	return s.save(nil)
}

func (s *Store) save(items []Item) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
