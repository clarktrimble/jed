// Package transship coordinates deploying stored jed services to Docker Swarm.
package transship

import (
	"context"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

const defaultGlobalEnvName = "_global"

// Swarm deploys a service to Docker Swarm.
//
// *swarm.Swarm implements this interface. The interface lives here so tests and
// downstream callers can provide narrow deploy targets without wrapping the full
// swarm client API.
type Swarm interface {
	Deploy(ctx context.Context, service jed.Service, env jed.Env, templateVars map[string]string) (string, error)
}

// Deployer loads desired service state from a store and applies it to Swarm.
type Deployer struct {
	Store         jed.Store
	Swarm         Swarm
	GlobalEnvName string
}

// DeployResult describes a completed deploy.
type DeployResult struct {
	Service jed.Service
	Env     jed.Env
	Global  jed.Env
	ID      string
}

// Created reports whether Deploy created a new swarm service.
func (r DeployResult) Created() bool {
	return r.ID != ""
}

// Deploy loads service, service env, and global template vars from the store,
// then creates or updates the swarm service named name.
func (d *Deployer) Deploy(ctx context.Context, name string) (DeployResult, error) {
	var result DeployResult

	if d.Store == nil {
		return result, errors.New("transship deployer has nil store")
	}
	if d.Swarm == nil {
		return result, errors.New("transship deployer has nil swarm")
	}

	svc, err := d.Store.GetService(ctx, name)
	if err != nil {
		return result, errors.Wrapf(err, "failed to get service %q from store", name)
	}
	result.Service = svc

	if err := svc.Validate(); err != nil {
		return result, errors.Wrapf(err, "failed to validate service %q", name)
	}

	env, err := d.Store.GetEnv(ctx, name)
	if err != nil {
		return result, errors.Wrapf(err, "failed to get env %q from store", name)
	}
	result.Env = env

	globalName := d.GlobalEnvName
	if globalName == "" {
		globalName = defaultGlobalEnvName
	}
	globalEnv, err := d.Store.GetEnv(ctx, globalName)
	if err != nil {
		return result, errors.Wrapf(err, "failed to get global env %q from store", globalName)
	}
	result.Global = globalEnv

	id, err := d.Swarm.Deploy(ctx, svc, env, globalEnv.Vars)
	if err != nil {
		return result, err
	}
	result.ID = id

	return result, nil
}
