// Package jed defines service configuration and persistence contracts for Just Enough Docker.
//
// The root package is intentionally runtime-neutral. It contains the shared
// service model used by runtime packages such as swarm and container, the
// Store interface used to persist desired service state, and Jed helpers for
// loading rendered specs from a store.
package jed

import (
	"context"
	"fmt"

	"github.com/clarktrimble/jed/logger"
	"github.com/pkg/errors"
)

// DBSchemaVersion is the current persistent Jed store schema version.
// Bump this when changing the store schema in a backwards-incompatible way.
const DBSchemaVersion = "1"

// Store persists services and environment variables.
//
// Implementations must be safe for concurrent use. The Store is the source
// of truth for all service definitions and environment variables.
type Store interface {
	// GetService retrieves a service definition by name.
	// Returns NotFoundError if the service does not exist.
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

// NotFoundError reports a missing stored object.
type NotFoundError struct {
	Kind string
	Name string
}

func (err NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", err.Kind, err.Name)
}

// Env holds named environment variables for a service.
//
// Env entries are stored separately from Service definitions so sensitive or
// deployment-specific values can be updated without changing the service model.
type Env struct {
	// Name is the service or variable-set name this environment belongs to.
	Name string `json:"name"`
	// Vars maps environment variable names to their values.
	Vars map[string]string `json:"vars"`
}

// Jed loads stored service state and renders runtime-neutral specs.
type Jed struct {
	store       Store
	varsEnvName string
	logger      logger.Logger
}

// New creates a Jed that uses render vars from varsEnvName.
func New(ctx context.Context, store Store, varsEnvName string, lgr logger.Logger) (*Jed, error) {
	if store == nil {
		return nil, errors.New("jed has nil store")
	}
	if lgr == nil {
		return nil, errors.New("jed has nil logger")
	}

	if _, err := store.GetEnv(ctx, varsEnvName); err != nil {
		return nil, errors.Wrapf(err, "failed to get vars env %q from store", varsEnvName)
	}

	return &Jed{store: store, varsEnvName: varsEnvName, logger: lgr}, nil
}

// Store returns the underlying store.
func (j *Jed) Store() Store {
	if j == nil {
		return nil
	}
	return j.store
}

// Scale updates the stored replica count for service name.
func (j *Jed) Scale(ctx context.Context, name string, count int) error {
	if j == nil {
		return errors.New("nil jed")
	}
	if j.store == nil {
		return errors.New("jed has nil store")
	}

	svc, err := j.store.GetService(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get service %q from store", name)
	}

	svc.Replicas = count
	if err := svc.Validate(); err != nil {
		return errors.Wrapf(err, "failed to validate service %q", name)
	}

	if err := j.store.SetService(ctx, svc); err != nil {
		return errors.Wrapf(err, "failed to set service %q in store", name)
	}

	return nil
}

// Spec loads service, env, and current render vars from the store and returns a rendered spec.
func (j *Jed) Spec(ctx context.Context, name string) (Spec, error) {
	if j == nil {
		return Spec{}, errors.New("nil jed")
	}
	if j.store == nil {
		return Spec{}, errors.New("jed has nil store")
	}

	svc, err := j.store.GetService(ctx, name)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get service %q from store", name)
	}

	if err := svc.Validate(); err != nil {
		return Spec{}, errors.Wrapf(err, "failed to validate service %q", name)
	}

	varsEnv, err := j.store.GetEnv(ctx, j.varsEnvName)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get vars env %q from store", j.varsEnvName)
	}

	env, err := j.store.GetEnv(ctx, name)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get env %q from store", name)
	}

	spec, err := Render(svc, env, varsEnv.Vars)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to render spec for service %q", name)
	}

	return spec, nil
}
