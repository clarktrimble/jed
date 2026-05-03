// Package container deploys jed services as standalone Docker containers.
package container

//go:generate moq -out mock_test.go -pkg container_test . Logger Client

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/clarktrimble/hondo"
	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/internal/dockerlog"
	"github.com/pkg/errors"
)

// Client is an HTTP client for communicating with the Docker API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) error
	SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
}

// Logger specifies a contextual, structured logger.
type Logger interface {
	Info(ctx context.Context, msg string, kv ...any)
	Debug(ctx context.Context, msg string, kv ...any)
	Error(ctx context.Context, msg string, err error, kv ...any)
}

// Runtime deploys services as standalone Docker containers.
type Runtime struct {
	client Client
	logger Logger
}

// New creates a standalone container runtime.
func New(client Client, logger Logger) *Runtime {
	return &Runtime{client: client, logger: logger}
}

// Deploy creates and starts a container for service using env.
func (rt *Runtime) Deploy(ctx context.Context, service jed.Service, env jed.Env) (id string, err error) {
	containers, err := rt.Containers(ctx)
	if err != nil {
		return
	}
	if containers.deployed(service.Name) {
		err = errors.Errorf("service %s already deployed", service.Name)
		return
	}

	cfg, err := buildConfig(service, env)
	if err != nil {
		return
	}

	deployName := service.Name + "-" + hondo.Rand(7)
	id, err = rt.create(ctx, deployName, cfg)
	if err != nil {
		return
	}

	err = rt.start(ctx, deployName)
	return
}

// Undeploy stops and removes a container by service name.
func (rt *Runtime) Undeploy(ctx context.Context, serviceName string) (err error) {
	containers, err := rt.Containers(ctx)
	if err != nil {
		return
	}

	ctr, err := containers.Find(serviceName)
	if err != nil {
		return
	}

	err = rt.stop(ctx, ctr.DeployName())
	if err != nil {
		// best effort, we could check for "304 already stopped"
		rt.logger.Error(ctx, "failed to stop container", err)
	}

	err = rt.delete(ctx, ctr.DeployName())
	return
}

// Redeploy undeploys and deploys a container.
func (rt *Runtime) Redeploy(ctx context.Context, service jed.Service, env jed.Env) (err error) {
	err = rt.Undeploy(ctx, service.Name)
	if err != nil {
		return
	}

	_, err = rt.Deploy(ctx, service, env)
	return
}

// Containers returns all containers managed by this runtime.
func (rt *Runtime) Containers(ctx context.Context) (mgd Containers, err error) {
	ctrs, err := rt.containers(ctx)
	if err != nil {
		return
	}

	mgd, noMatch := ctrs.managed()
	if len(noMatch) != 0 {
		rt.logger.Error(ctx, "ignoring managed_by=jed containers",
			errors.Errorf("unexpected containers"), "names", noMatch)
	}
	return
}

// Logs retrieves logs from a container using Docker API.
func (rt *Runtime) Logs(ctx context.Context, id, tail string) (logs []byte, err error) {
	rt.logger.Info(ctx, "getting container logs", "id", id)

	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%s", id, tail)
	rawLogs, err := rt.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		err = errors.Wrap(err, "failed to get logs for container")
		return
	}

	reader := dockerlog.Decode(bytes.NewReader(rawLogs))
	logs, err = io.ReadAll(reader)
	if err != nil {
		err = errors.Wrap(err, "failed to parse docker logs for container")
		return
	}

	return
}
