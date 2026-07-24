// Package memo provides an in-memory implementation of jed.Store.
//
// The in-memory store is useful for testing and development.
// All data is lost when the process exits.
package memo

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/clarktrimble/jed"
)

// Store implements jed.Store interface using in-memory storage.
// Safe for concurrent use.
type Store struct {
	mu      sync.RWMutex
	svcs    map[string]jed.Service
	envs    map[string]jed.Env
	intents map[string]jed.Intent
}

// New creates a new empty in-memory store.
func New() *Store {
	return &Store{
		svcs:    make(map[string]jed.Service),
		envs:    make(map[string]jed.Env),
		intents: make(map[string]jed.Intent),
	}
}

// GetService retrieves a service definition by name and image.
// Note: shallow copy!!
func (s *Store) GetService(ctx context.Context, name, image string) (jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.svcs[serviceKey(name, image)]
	if !ok {
		return jed.Service{}, jed.NotFoundError{Kind: "service", Name: serviceName(name, image)}
	}
	return svc, nil
}

// SetService persists a service definition.
func (s *Store) SetService(ctx context.Context, svc jed.Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.svcs[serviceKey(svc.Name, svc.Image)] = svc
	return nil
}

// DelService removes a service definition by name and image.
func (s *Store) DelService(ctx context.Context, name, image string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.svcs, serviceKey(name, image))
	return nil
}

// Services retrieves all service definitions for name.
// Note: shallow copy!!
func (s *Store) Services(ctx context.Context, name string) ([]jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var services []jed.Service
	for _, svc := range s.svcs {
		if svc.Name == name {
			services = append(services, svc)
		}
	}
	return services, nil
}

// AllServices retrieves all service definitions.
// Note: shallow copy!!
func (s *Store) AllServices(ctx context.Context) ([]jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Collect(maps.Values(s.svcs)), nil
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

// GetIntent retrieves the desired active image and replica count by service name.
func (s *Store) GetIntent(ctx context.Context, name string) (jed.Intent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	intent, ok := s.intents[name]
	if !ok {
		return jed.Intent{}, jed.NotFoundError{Kind: "intent", Name: name}
	}
	return intent, nil
}

// SetIntent persists the desired active image and replica count.
func (s *Store) SetIntent(ctx context.Context, intent jed.Intent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.intents[intent.Name] = intent
	return nil
}

// DelIntent removes the desired active image and replica count by service name.
func (s *Store) DelIntent(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.intents, name)
	return nil
}

// Intents retrieves all desired active images and replica counts.
func (s *Store) Intents(ctx context.Context) ([]jed.Intent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Collect(maps.Values(s.intents)), nil
}

func serviceKey(name, image string) string {
	return name + "\x00" + image
}

func serviceName(name, image string) string {
	return name + "@" + image
}
