// Package jed manages Docker containers as services with persistent configuration.
//
// # Abstractions
//
//   - Service:   Persistent configuration defining what to deploy (image, ports, volumes, etc.)
//   - Env:       A container's environment variables
//   - Container: Selected fields from Docker API's container data, augmented with service name
//
// # Lifecycle
//
//   - Deploy() creates and starts a Container from a Service
//   - Undeploy() stops and removes a Container
//   - Redeploy() stops, removes, creates, and starts a Container with fresh cfg and env
//
// # Identification
//
//   - Containers managed by Jed have the "managed_by=jed" label
//   - Containers are named by appending a random suffix to the service name (e.g., "postgres-k7m9x2n")
//
// # Stateless
//
// Jed relies on the Docker API and Service/Env Store for all state, maintaining no internal cache.
package jed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"

	"github.com/clarktrimble/hondo"
	"github.com/pkg/errors"
)

//go:generate moq -out mock_test.go -pkg jed_test . Logger Client Store

var (
	deployNamePattern = regexp.MustCompile(`^/(.+)-[a-zA-Z0-9]{7}$`)
)

// Client is an HTTP client for communicating with the Docker API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) (err error)
	SendJson(ctx context.Context, method, path string, body io.Reader) (data []byte, err error)
}

// Logger specifies a contextual, structured logger.
type Logger interface {
	Info(ctx context.Context, msg string, kv ...any)
	Debug(ctx context.Context, msg string, kv ...any)
	Error(ctx context.Context, msg string, err error, kv ...any)
}

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

// Config holds configuration for creating a Jed instance.
type Config struct{}

// Jed manages Docker containers as services.
type Jed struct {
	client Client
	logger Logger
	store  Store
}

// New creates a Jed from Config.
func (cfg *Config) New(ctx context.Context, client Client, lgr Logger, store Store) (jed *Jed, err error) {

	jed = &Jed{
		client: client,
		logger: lgr,
		store:  store,
	}

	// Validate images exist for all services
	services, err := store.Services(ctx)
	if err != nil {
		return
	}

	for _, service := range services {
		err = jed.checkImage(ctx, service.Image)
		if err != nil {
			return
		}
	}

	return
}

// Deploy creates and starts a container.
func (jed *Jed) Deploy(ctx context.Context, service Service) (id string, err error) {

	// Todo: just take name and lookup?

	containers, err := jed.Containers(ctx)
	if err != nil {
		return
	}
	if containers.deployed(service.Name) {
		err = errors.Errorf("service %s already deployed", service.Name)
		return
	}

	env, err := jed.store.GetEnv(ctx, service.Name)
	if err != nil {
		return
	}
	cfg, err := service.config(env)
	if err != nil {
		return
	}

	deployName := service.Name + "-" + hondo.Rand(7)
	id, err = jed.create(ctx, deployName, cfg)
	if err != nil {
		return
	}

	err = jed.start(ctx, deployName)
	if err != nil {
		return
	}

	return
}

// Undeploy stops and removes a container.
func (jed *Jed) Undeploy(ctx context.Context, service Service) (err error) {

	containers, err := jed.Containers(ctx)
	if err != nil {
		return
	}

	ctr, err := containers.Find(service.Name)
	if err != nil {
		return
	}

	err = jed.stop(ctx, ctr.DeployName())
	if err != nil {
		// best effort, we could check for "304 already stopped"
		jed.logger.Error(ctx, "failed to stop container", err)
	}

	err = jed.delete(ctx, ctr.DeployName())
	return
}

// Redeploy undeploys and deploys a container.
func (jed *Jed) Redeploy(ctx context.Context, service Service) (err error) {

	err = jed.Undeploy(ctx, service)
	if err != nil {
		return
	}

	_, err = jed.Deploy(ctx, service)
	return
}

// Containers returns all containers managed by this service.
func (jed *Jed) Containers(ctx context.Context) (mgd Containers, err error) {

	ctrs, err := jed.containers(ctx)
	if err != nil {
		return
	}

	mgd, noMatch := managed(ctrs)
	if len(noMatch) != 0 {
		jed.logger.Error(ctx, "ignoring managed_by=jed containers",
			errors.Errorf("unexpected containers"), "names", noMatch)
	}
	return
}

// Logs retrieves logs from a container using Docker API.
func (jed *Jed) Logs(ctx context.Context, id, tail string) (logs []byte, err error) {

	jed.logger.Info(ctx, "getting container logs", "id", id)

	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%s", id, tail)
	rawLogs, err := jed.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to get logs for container")
		return
	}

	// deocdeLogs is written to support streaming
	// to expose that from here we'd need an io.ReadCloser from Client
	// and to return one as well.  Could be a cool feature :)
	reader := decodeLogs(bytes.NewReader(rawLogs))
	logs, err = io.ReadAll(reader)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse docker logs for container")
		return
	}

	return
}
