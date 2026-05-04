package jed

import (
	"context"
	"maps"

	"github.com/pkg/errors"
)

// Jed loads stored service state and renders runtime-neutral specs.
type Jed struct {
	store Store
	vars  map[string]string
}

// New creates a Jed using render vars loaded from the env named name.
func New(ctx context.Context, store Store, name string) (*Jed, error) {
	if store == nil {
		return nil, errors.New("jed has nil store")
	}

	env, err := store.GetEnv(ctx, name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vars env %q from store", name)
	}

	return &Jed{store: store, vars: maps.Clone(env.Vars)}, nil
}

// Spec loads service and env by name from the store and returns a rendered spec.
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

	env, err := j.store.GetEnv(ctx, name)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to get env %q from store", name)
	}

	spec, err := NewSpec(svc, env, j.vars)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to render spec for service %q", name)
	}

	return spec, nil
}
