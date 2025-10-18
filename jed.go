package jed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"slices"

	"github.com/clarktrimble/hondo"
	"github.com/pkg/errors"
)

//go:generate moq -out mock_test.go -pkg jed_test . Logger Client

var (
	suffixPattern = regexp.MustCompile(`-[^-]+$`) // Todo: make specific
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

// Config is Svc configurables.
type Config struct{}

// Svc is the service.
// Container and Status relationship:
// - Container is input config (immutable, user-facing base names like "postgres")
// - Status is runtime state from Docker (actual deployed names like "postgres-k7m9x2n")
// - Deploy adds random suffix to Container.Name to create unique deployed names
// - Statii() returns map keyed by base name for easy lookup
// - Statii are filtered by managed_by=jed label
//
// Todo: implement container loading that adds managed_by=jed label
type Svc struct {
	client Client
	logger Logger
	cntrs  []Container
}

// NewSvc creates an Svc from Config.
func (cfg *Config) NewSvc(ctx context.Context, client Client, lgr Logger, cfs fs.FS) (svc *Svc, err error) {

	containers, err := loadContainers(cfs)
	if err != nil {
		return
	}

	svc = &Svc{
		client: client,
		logger: lgr,
		cntrs:  containers,
	}

	for _, cntr := range svc.cntrs {
		err = svc.checkImage(ctx, cntr.Image)
		if err != nil {
			return
		}
	}

	return
}

// Containers returns a copy of the loaded containers.
func (svc *Svc) Containers() []Container {
	return slices.Clone(svc.cntrs)
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

	err = svc.start(ctx, deployName)
	if err != nil {
		return
	}

	return
}

// Undeploy stops and removes a container.
func (svc *Svc) Undeploy(ctx context.Context, cntr *Container) (err error) {

	statii, err := svc.Statii(ctx)
	if err != nil {
		return
	}

	deployName, err := statii.DeployName(cntr.Name)
	if err != nil {
		return
	}

	err = svc.stop(ctx, deployName)
	if err != nil {
		// best effort, we could check for "304 already stopped"
		svc.logger.Error(ctx, "failed to stop container", err)
	}

	err = svc.delete(ctx, deployName)
	return
}

// Statii returns all containers managed by this service.
func (svc *Svc) Statii(ctx context.Context) (statii Statii, err error) {

	statuses, err := svc.containers(ctx)
	if err != nil {
		return
		// Todo: think about best effort here (with logses of course!)
	}

	statii = newStatii(statuses)
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
