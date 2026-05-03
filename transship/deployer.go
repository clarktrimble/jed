// Package transship coordinates deploying stored jed services to Docker Swarm.
package transship

import (
	"context"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// Swarm deploys a rendered service spec to Docker Swarm.
//
// *swarm.Swarm implements this interface. The interface lives here so tests and
// downstream callers can provide narrow deploy targets without wrapping the full
// swarm client API.
type Swarm interface {
	Deploy(ctx context.Context, spec jed.Spec) (id string, created bool, err error)
}

// Deployer loads desired service state from a store and deploys it to Swarm.
type Deployer struct {
	Store jed.Store
	Swarm Swarm
	Vars  map[string]string
}

// Deploy loads service and env from the store, renders a spec using Vars,
// then creates or updates the swarm service named name.
func (d *Deployer) Deploy(ctx context.Context, name string) (spec jed.Spec, id string, created bool, err error) {
	if d.Store == nil {
		err = errors.New("transship deployer has nil store")
		return
	}
	if d.Swarm == nil {
		err = errors.New("transship deployer has nil swarm")
		return
	}

	svc, err := d.Store.GetService(ctx, name)
	if err != nil {
		err = errors.Wrapf(err, "failed to get service %q from store", name)
		return
	}

	if err = svc.Validate(); err != nil {
		err = errors.Wrapf(err, "failed to validate service %q", name)
		return
	}

	env, err := d.Store.GetEnv(ctx, name)
	if err != nil {
		err = errors.Wrapf(err, "failed to get env %q from store", name)
		return
	}

	spec, err = jed.NewSpec(svc, env, d.Vars)
	if err != nil {
		return
	}

	id, created, err = d.Swarm.Deploy(ctx, spec)
	return
}
