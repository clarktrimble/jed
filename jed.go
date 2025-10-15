package jed

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

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

type Svc struct {
	client Client
	logger Logger
}

// GetContainerLogs retrieves logs from a container using Docker API.
func (svc *Svc) GetContainerLogs(ctx context.Context, id, tail string) (logs []byte, err error) {

	svc.logger.Info(ctx, "getting container logs", "id", id)

	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%s", id, tail)

	rawLogs, err := svc.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to get logs for container")
		return
	}

	reader := decodeLogs(bytes.NewReader(rawLogs))
	logs, err = io.ReadAll(reader)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse docker logs for container")
		return
	}

	return
}

// decodeLogs decodes Docker's multiplexed stream format.
// Docker logs use an 8-byte header for each frame:
// [STREAM_TYPE, 0, 0, 0, SIZE_BE_UINT32]
// where STREAM_TYPE is 1 for stdout, 2 for stderr
func decodeLogsOg(src io.Reader, dst io.Writer) (err error) {

	header := make([]byte, 8)

	for {
		_, err = io.ReadFull(src, header)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			err = errors.Wrapf(err, "failed to read docker log header")
			return
		}

		frameSize := binary.BigEndian.Uint32(header[4:8])
		_, err = io.CopyN(dst, src, int64(frameSize))
		if err != nil {
			err = errors.Wrapf(err, "failed to copy docker log stream for decode")
			return
		}
	}
}
