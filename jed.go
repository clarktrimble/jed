package jed

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

//go:generate moq -out mock_test.go -pkg jed_test . Logger Client

// Client specifies an http client by which stuff can be sent and received.
type Client interface {
	//SendObject(ctx context.Context, method, path string, snd, rcv any) (err error)
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

// NewSvc creates an Svc from Config.
func (cfg *Config) NewSvc(client Client, lgr Logger) *Svc {

	return &Svc{
		client: client,
		logger: lgr,
	}
}

// Svc is the service.
type Svc struct {
	client Client
	logger Logger
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
