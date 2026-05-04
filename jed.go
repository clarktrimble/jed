// Package jed defines service configuration and persistence contracts for Just Enough Docker.
//
// The root package is intentionally runtime-neutral. It contains the shared
// service model used by runtime packages such as swarm and container, the
// Store interface used to persist desired service state, and Jed helpers for
// loading rendered specs from a store.
package jed

import "context"

// Store persists services and environment variables.
//
// Implementations must be safe for concurrent use. The Store is the source
// of truth for all service definitions and environment variables.
type Store interface {
	// GetService retrieves a service definition by name.
	// Returns an error if the service does not exist.
	GetService(ctx context.Context, name string) (Service, error)

	// SetService creates or updates a service definition.
	SetService(ctx context.Context, svc Service) error

	// DelService removes a service definition by name.
	DelService(ctx context.Context, name string) error

	// Services returns all service definitions.
	Services(ctx context.Context) ([]Service, error)

	// GetEnv retrieves environment variables for a service.
	// Returns an empty Env with initialized Vars map when the service has no
	// environment variables (not an error). Returns an error only on storage failures.
	GetEnv(ctx context.Context, name string) (Env, error)

	// SetEnv creates or updates environment variables for a service.
	SetEnv(ctx context.Context, env Env) error

	// DelEnv removes environment variables for a service by name.
	DelEnv(ctx context.Context, name string) error

	// Envs returns all environment variable sets for all services.
	Envs(ctx context.Context) ([]Env, error)
}

// Env holds environment variables for a service.
type Env struct {
	Name string
	Vars map[string]string
}
