// Package memo provides an in-memory implementation of jed.Store.
//
// The in-memory store is useful for testing and development.
// All data is lost when the process exits.
package memo

import (
	"context"
	"maps"
	"sync"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// Store implements jed.Store interface using in-memory storage.
// Safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	svcs map[string]jed.Service
	envs map[string]jed.Env
}

// New creates a new empty in-memory store.
func New() *Store {
	return &Store{
		svcs: make(map[string]jed.Service),
		envs: make(map[string]jed.Env),
	}
}

// GetService retrieves a service definition by name.
// Note: shallow copy!!
func (s *Store) GetService(ctx context.Context, name string) (jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.svcs[name]
	if !ok {
		return jed.Service{}, errors.Errorf("service not found: %s", name)
	}
	return svc, nil
}

// SetService persists a service definition.
func (s *Store) SetService(ctx context.Context, svc jed.Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.svcs[svc.Name] = svc
	return nil
}

// DelService removes a service definition.
func (s *Store) DelService(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.svcs, name)
	return nil
}

// Services retrieves all service definitions.
// Note: shallow copy!!
func (s *Store) Services(ctx context.Context) ([]jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]jed.Service, 0, len(s.svcs))
	for _, svc := range s.svcs {
		services = append(services, svc)
	}
	return services, nil
}

// GetEnv retrieves environment variables for a service.
func (s *Store) GetEnv(ctx context.Context, name string) (jed.Env, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, ok := s.envs[name]
	if !ok {
		// Return empty env when not found (not an error condition)
		return jed.Env{Name: name, Vars: make(map[string]string)}, nil
	}

	// Return a copy to prevent external modification
	return jed.Env{
		Name: env.Name,
		Vars: maps.Clone(env.Vars),
	}, nil
}

// SetEnv persists environment variables for a service.
func (s *Store) SetEnv(ctx context.Context, env jed.Env) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external modification
	s.envs[env.Name] = jed.Env{
		Name: env.Name,
		Vars: maps.Clone(env.Vars),
	}
	return nil
}

// DelEnv removes environment variables for a service.
func (s *Store) DelEnv(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.envs, name)
	return nil
}

// Envs retrieves all environment variable sets.
func (s *Store) Envs(ctx context.Context) ([]jed.Env, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	envs := make([]jed.Env, 0, len(s.envs))
	for _, env := range s.envs {
		// Return copies to prevent external modification
		envs = append(envs, jed.Env{
			Name: env.Name,
			Vars: maps.Clone(env.Vars),
		})
	}
	return envs, nil
}
