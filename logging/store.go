package logging

import (
	"sync"
)

const firstAlloc = 4096

type store struct {
	data map[string]*Logger
	mu   sync.RWMutex
}

func newStore() *store {
	return &store{
		data: make(map[string]*Logger, firstAlloc),
		mu:   sync.RWMutex{},
	}
}

func (s *store) getOrCreate(id string) *Logger {
	s.mu.RLock()
	logger, ok := s.data[id]
	s.mu.RUnlock()

	if !ok {
		s.mu.Lock()
		defer s.mu.Unlock()

		logger, ok = s.data[id]
		if !ok {
			logger = newInstance(id)
			s.data[id] = logger
		}
	}

	return logger
}
