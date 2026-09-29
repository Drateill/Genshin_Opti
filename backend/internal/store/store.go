// Package store holds the current GOOD import in memory for the running
// process, per the spec ("GOOD JSON gardé en mémoire au runtime pour le
// solver"). Postgres persistence is explicitly optional in the spec and is
// not implemented here.
package store

import (
	"sync"

	"artifact-optimizer/internal/model"
)

type Store struct {
	mu     sync.RWMutex
	export model.GoodExport
}

func New() *Store {
	return &Store{}
}

func (s *Store) Set(export model.GoodExport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.export = export
}

func (s *Store) Get() model.GoodExport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.export
}

func (s *Store) HasData() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.export.Artifacts) > 0
}
