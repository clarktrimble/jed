package jed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"regexp"

	"github.com/clarktrimble/hondo"
	"github.com/pkg/errors"
)

//go:generate moq -out mock_test.go -pkg jed_test . Logger Client

const (
	configFile string = "services.yaml"
	envSuffix  string = "env"
)

var (
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

// Config is Jed configurables.
type Config struct{}

// Jed is the service.
// Service and Container relationship:
// - Service is input config (immutable, user-facing base names like "postgres")
// - Container is runtime state from Docker (actual deployed names like "postgres-k7m9x2n")
// - Deploy adds random suffix to Service.Name to create unique deployed names
// - Containers() returns map keyed by service name for easy lookup
// - Containers are filtered by managed_by=jed label
type Jed struct {
	client Client
	logger Logger
	svcs   []Service
}

// New creates a Jed from Config.
func (cfg *Config) New(ctx context.Context, client Client, lgr Logger, cfs fs.FS) (jed *Jed, err error) {

	services, err := loadServices(cfs)
	if err != nil {
		return
	}

	jed = &Jed{
		client: client,
		logger: lgr,
		svcs:   services,
	}

	for _, service := range jed.svcs {
		err = jed.checkImage(ctx, service.Image)
		if err != nil {
			return
		}
	}

	return
}

// Services returns a copy of services mapped by name.
func (jed *Jed) Services() map[string]Service {

	services := make(map[string]Service, len(jed.svcs))
	for _, svc := range jed.svcs {
		services[svc.Name] = Service{
			Name:    svc.Name,
			Image:   svc.Image,
			Network: svc.Network,
			Restart: svc.Restart,
			Env:     maps.Clone(svc.Env),
			Ports:   maps.Clone(svc.Ports),
			Labels:  maps.Clone(svc.Labels),
			Volumes: maps.Clone(svc.Volumes),
		}
	}
	return services
}

// Deploy creates and starts a container.
func (jed *Jed) Deploy(ctx context.Context, service *Service) (id string, err error) {

	suffix := hondo.Rand(7)
	deployName := service.Name + "-" + suffix

	cfg, err := service.config()
	if err != nil {
		return
	}

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
func (jed *Jed) Undeploy(ctx context.Context, service *Service) (err error) {

	containers, err := jed.Containers(ctx)
	if err != nil {
		return
	}

	deployName, err := containers.DeployName(service.Name)
	if err != nil {
		return
	}

	err = jed.stop(ctx, deployName)
	if err != nil {
		// best effort, we could check for "304 already stopped"
		jed.logger.Error(ctx, "failed to stop container", err)
	}

	err = jed.delete(ctx, deployName)
	return
}

// Containers returns all containers managed by this service.
func (jed *Jed) Containers(ctx context.Context) (Containers, error) {

	containers, err := jed.containers(ctx)
	if err != nil {
		return Containers{}, err
	}

	byName, noMatch := newContainers(containers)
	if len(noMatch) != 0 {
		err = errors.Errorf("unexpected container names")
		jed.logger.Error(ctx, "ignoring managed_by=jed containers", err, "names", noMatch)
	}
	return byName, nil
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
