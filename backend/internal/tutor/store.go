package tutor

import (
	"encoding/json"
	"os"
	"sync"
)

// Store keeps one conversation per case.
//
// Server-side rather than in the browser, for the same reason the completion
// dates are: this is a record of study that should survive clearing site data
// and be the same on a second browser. It lives beside progress-log.json and is
// gitignored — a transcript is working state, not source.
//
// Keyed by slug, so a case has one continuous thread across days rather than a
// new one per visit. Coming back to W10D4 tomorrow should resume the
// conversation, not start again from "what is a Sub".
type Store struct {
	mu      sync.Mutex
	path    string
	threads map[string][]Message
}

// OpenStore reads path if it exists, or starts empty on the first run.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, threads: map[string][]Message{}}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.threads); err != nil {
		// A corrupt transcript is not worth refusing to start over. The
		// conversation is a convenience; the plan is the product.
		s.threads = map[string][]Message{}
	}
	return s, nil
}

// Thread returns a copy of one case's conversation, oldest first.
func (s *Store) Thread(slug string) []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.threads[slug]...)
}

// Append adds turns to a case's thread and writes the file.
func (s *Store) Append(slug string, msgs ...Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.threads[slug] = append(s.threads[slug], msgs...)
	return s.save()
}

// Clear forgets one case's conversation.
func (s *Store) Clear(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.threads, slug)
	return s.save()
}

// save writes the whole file. Callers hold the lock.
//
// Written to a temporary file and renamed, so a crash mid-write leaves the
// previous transcript intact rather than a half-written one that will not parse.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.threads, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
