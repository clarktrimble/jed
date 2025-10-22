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
	// Todo: round these up and rename
	suffixPattern = regexp.MustCompile(`^/(.+)-[a-zA-Z0-9]{7}$`)
)

// Client specifies an http client by which stuff can be sent and received.
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
// Implementations must be safe for concurrent use.
type Store interface {
	// Service operations
	GetService(ctx context.Context, name string) (Service, error)
	SetService(ctx context.Context, svc Service) error
	DelService(ctx context.Context, name string) error
	Services(ctx context.Context) ([]Service, error) // Todo: deep copy?

	// Env operations
	// GetEnv returns environment variables for a service.
	// Returns empty Env with initialized Vars map when service has no env (not an error).
	// Todo: godoc properly
	GetEnv(ctx context.Context, name string) (Env, error)
	SetEnv(ctx context.Context, env Env) error
	DelEnv(ctx context.Context, name string) error
	Envs(ctx context.Context) ([]Env, error)
}

// Config is Jed configurables.
type Config struct{}

// Jed is the service.
// Service and Container relationship:
// - Service is input config (immutable, user-facing base names like "postgres")
// - Container is runtime state from Docker (actual deployed names like "postgres-k7m9x2n")
// - Deploy adds random suffix to Service.Name to create unique deployed names
// - Containers() returns map keyed by service name for easy lookup
// - Containers are filtered by managed_by=jed label
// - Store is the source of truth for service definitions and environment variables (no caching)
type Jed struct {
	client Client
	logger Logger
	store  Store // Required: source of truth for services and environment variables
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

// SetEnv sets environment variables for a service.
func (jed *Jed) SetEnv(ctx context.Context, serviceName string, env map[string]string) (err error) {

	services, err := jed.Services(ctx)
	if err != nil {
		return
	}

	_, err = services.Find(serviceName)
	if err != nil {
		return
	}

	err = jed.store.SetEnv(ctx, Env{Name: serviceName, Vars: env})
	return
}

// GetEnv retrieves environment variables for a service.
func (jed *Jed) GetEnv(ctx context.Context, serviceName string) (env map[string]string, err error) {

	e, err := jed.store.GetEnv(ctx, serviceName)
	if err != nil {
		return
	}
	env = e.Vars
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
