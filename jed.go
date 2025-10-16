package jed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/clarktrimble/hondo"
	"github.com/pkg/errors"
)

//go:generate moq -out mock_test.go -pkg jed_test . Logger Client

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

// Config is Svc configurables.
type Config struct{}

// Svc is the service.
// Container and Status relationship:
// - Container is input config (immutable, user-facing base names like "postgres")
// - Status is runtime state from Docker (actual deployed names like "postgres-k7m9x2n")
// - Deploy adds random suffix to Container.Name to create unique deployed names
// - Undeploy matches Container.Name to Status via prefix matching
// - Statii are filtered by managed_by=jed label
// - Caller maintains svc.statii via Statii() calls
//
// Todo: implement container loading that adds managed_by=jed label
// Todo: ensure statii are kept up-to-date by caller (refresh strategy?)
// Todo: consider regex validation of suffix format in findDeployName
type Svc struct {
	client Client
	logger Logger
	statii []Status
}

// NewSvc creates an Svc from Config.
func (cfg *Config) NewSvc(client Client, lgr Logger) *Svc {

	return &Svc{
		client: client,
		logger: lgr,
	}
}

// Status is container status returned from docker
// Todo: rename?
// Todo: think thru how we'll structure this relative to services
type Status struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

// Deploy creates and starts a container.
func (svc *Svc) Deploy(ctx context.Context, cntr *Container) (id string, err error) {

	suffix := hondo.Rand(7)
	deployName := cntr.Name + "-" + suffix

	cfg, err := cntr.config()
	if err != nil {
		return
	}

	id, err = svc.create(ctx, deployName, cfg)
	if err != nil {
		return
	}

	//cntr.Id = id

	err = svc.start(ctx, deployName)
	if err != nil {
		return
	}

	//cntr.Condition = Unchecked
	return
}

// Undeploy stops and removes a container.
func (svc *Svc) Undeploy(ctx context.Context, cntr *Container) (err error) {

	deployName, err := svc.findDeployName(cntr.Name)
	if err != nil {
		return
	}

	err = svc.stop(ctx, deployName)
	if err != nil {
		// Todo: easy to get hung up on created but not started, think thru
		//return
		svc.logger.Error(ctx, "failed to stop container", err)
		// best effort
	}

	err = svc.delete(ctx, deployName)
	if err != nil {
		return
	}

	//cntr.Id = ""
	//cntr.Condition = Undeployed
	return
}

// findDeployName finds the deployed container name from base name by matching statii.
func (svc *Svc) findDeployName(baseName string) (deployName string, err error) {

	prefix := baseName + "-"
	for _, status := range svc.statii {
		for _, name := range status.Names {
			// Docker prepends "/" to names, strip it
			cleanName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(cleanName, prefix) {
				return cleanName, nil
			}
		}
	}

	err = errors.Errorf("container with base name %s not found", baseName)
	return
}

// Statii returns all containers managed by this service.
func (svc *Svc) Statii(ctx context.Context) (statii []Status, err error) {

	err = svc.client.SendObject(ctx, "GET", "/containers/json?all=true&filters={\"label\":[\"managed_by=jed\"]}", nil, &statii)
	if err != nil {
		return
	}

	//statii = map[string]Status{}
	//for _, status := range filtered {
	//statii[status.Id] = status
	//}

	return
}

// Logs retrieves logs from a container using Docker API.
// Todo: consider additional options (timestamps, since, until, follow)
func (svc *Svc) Logs(ctx context.Context, id, tail string) (logs []byte, err error) {

	svc.logger.Info(ctx, "getting container logs", "id", id)

	// Todo: allow for stdout and/or stderr
	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%s", id, tail)

	rawLogs, err := svc.client.SendJson(ctx, "GET", path, nil)
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
