package jed

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

// decodeLogs returns an io.Reader that decodes Docker's multiplexed stream format.
func decodeLogs(src io.Reader) io.Reader {
	return &logDecoder{
		src:    src,
		header: make([]byte, 8),
	}
}

type logDecoder struct {
	src       io.Reader
	header    []byte
	buf       []byte // buffered decoded data
	bufOffset int    // current position in buf
}

func (d *logDecoder) Read(p []byte) (n int, err error) {
	// If we have buffered data, return it first
	if d.bufOffset < len(d.buf) {
		n = copy(p, d.buf[d.bufOffset:])
		d.bufOffset += n
		if d.bufOffset >= len(d.buf) {
			d.buf = nil
			d.bufOffset = 0
		}
		return n, nil
	}

	// Read next frame header
	_, err = io.ReadFull(d.src, d.header)
	if err == io.EOF {
		return 0, io.EOF
	}
	if err != nil {
		return 0, errors.Wrap(err, "failed to read docker log header")
	}

	// Extract frame size
	frameSize := binary.BigEndian.Uint32(d.header[4:8])

	// Read frame into buffer
	d.buf = make([]byte, frameSize)
	_, err = io.ReadFull(d.src, d.buf)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read docker log frame")
	}

	// Copy what we can to p
	n = copy(p, d.buf)
	d.bufOffset = n

	return n, nil
}
