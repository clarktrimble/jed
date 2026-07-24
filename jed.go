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
const DBSchemaVersion = "3"

// Store persists services and environment variables.
//
// Implementations must be safe for concurrent use. The Store is the source
// of truth for all service definitions and environment variables.
type Store interface {
	// GetService retrieves a service definition by name and image.
	// Returns NotFoundError if the service does not exist.
	GetService(ctx context.Context, name, image string) (Service, error)

	// SetService creates or updates a service definition.
	SetService(ctx context.Context, svc Service) error

	// DelService removes a service definition by name and image.
	DelService(ctx context.Context, name, image string) error

	// Services returns all service definitions for name.
	Services(ctx context.Context, name string) ([]Service, error)

	// AllServices returns all service definitions.
	AllServices(ctx context.Context) ([]Service, error)

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

	// GetIntent retrieves the desired active image and replica count by service name.
	// Returns NotFoundError if the service has no intent and is therefore disabled.
	GetIntent(ctx context.Context, name string) (Intent, error)

	// SetIntent creates or updates the desired active image and replica count.
	SetIntent(ctx context.Context, intent Intent) error

	// DelIntent removes the desired active image and replica count by service name.
	DelIntent(ctx context.Context, name string) error

	// Intents returns all desired active images and replica counts.
	Intents(ctx context.Context) ([]Intent, error)
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

// Config controls Jed creation. Fields are populated by launch (envconfig)
// from their default tags; New does not apply defaults.
type Config struct {
	// VarsEnvName is the store env whose vars are available to all specs during render.
	VarsEnvName string `json:"vars_env_name" default:"_global"`
	// DefaultUid is the container user (uid or uid:gid) applied when a service omits user.
	DefaultUid string `json:"default_uid" default:"1000"`
}

// Jed loads stored service state and renders runtime-neutral specs.
type Jed struct {
	store       Store
	varsEnvName string
	defaultUid  string
	logger      logger.Logger
}

// New creates Jed from Config.
func (cfg *Config) New(store Store, lgr logger.Logger) *Jed {

	return &Jed{
		store:       store,
		varsEnvName: cfg.VarsEnvName,
		defaultUid:  cfg.DefaultUid,
		logger:      lgr,
	}
}

// Store returns the underlying store.
func (j *Jed) Store() Store {
	return j.store
}

// Enable marks service name as enabled in the store.
func (j *Jed) Enable(ctx context.Context, name string) error {
	svc, err := j.singleService(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get service %q from store", name)
	}

	svc.Enabled = true

	err = svc.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate service %q", name)
	}

	err = j.store.SetService(ctx, svc)
	if err != nil {
		return errors.Wrapf(err, "failed to set service %q in store", name)
	}

	return nil
}

// Disable marks service name as disabled in the store.
func (j *Jed) Disable(ctx context.Context, name string) error {
	svc, err := j.singleService(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get service %q from store", name)
	}

	svc.Enabled = false

	err = svc.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate service %q", name)
	}

	err = j.store.SetService(ctx, svc)
	if err != nil {
		return errors.Wrapf(err, "failed to set service %q in store", name)
	}

	return nil
}

// Scale updates the stored replica count for service name.
func (j *Jed) Scale(ctx context.Context, name string, count int) error {
	svc, err := j.singleService(ctx, name)
	if err != nil {
		return errors.Wrapf(err, "failed to get service %q from store", name)
	}

	svc.Replicas = count

	err = svc.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate service %q", name)
	}

	err = j.store.SetService(ctx, svc)
	if err != nil {
		return errors.Wrapf(err, "failed to set service %q in store", name)
	}

	return nil
}

// Todo: make sure this gets cleaned up
func (j *Jed) singleService(ctx context.Context, name string) (Service, error) {
	services, err := j.store.Services(ctx, name)
	if err != nil {
		return Service{}, err
	}
	if len(services) == 0 {
		return Service{}, NotFoundError{Kind: "service", Name: name}
	}
	if len(services) > 1 {
		return Service{}, errors.Errorf("service %q has multiple images", name)
	}
	return services[0], nil
}

// Spec loads the service, its env, and the current render vars from the store
// and returns a rendered spec.
func (j *Jed) Spec(ctx context.Context, name string) (Spec, error) {
	env, err := j.store.GetEnv(ctx, name)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get env %q from store", name)
	}

	return j.spec(ctx, name, env)
}

// SpecWithEnv renders name's spec using the provided service env instead of the
// stored one. The env is not merged with the stored env and is not saved; it is
// used only to render this spec, e.g. to preflight a submitted env before saving.
func (j *Jed) SpecWithEnv(ctx context.Context, name string, env Env) (Spec, error) {
	return j.spec(ctx, name, env)
}

// spec renders name's service using env as the service env and the stored render vars.
func (j *Jed) spec(ctx context.Context, name string, env Env) (Spec, error) {
	svc, err := j.singleService(ctx, name)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get service %q from store", name)
	}

	if svc.User == "" {
		svc.User = j.defaultUid
	}

	err = svc.Validate()
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to validate service %q", name)
	}

	varsEnv, err := j.store.GetEnv(ctx, j.varsEnvName)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get vars env %q from store", j.varsEnvName)
	}

	spec, err := render(svc, env, varsEnv.Vars)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to render spec for service %q", name)
	}

	return spec, nil
}
