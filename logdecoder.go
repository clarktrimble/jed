package jed

import (
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

const (
	maxFrameSize uint32 = 65536
)

func decodeLogs(src io.Reader) io.Reader {
	return &logDecoder{
		src:    src,
		header: make([]byte, 8),
		buf:    make([]byte, 0, maxFrameSize),
	}
}

type logDecoder struct {
	src    io.Reader
	header []byte
	buf    []byte
	offset int
}

// Read implements io.Reader .
func (dec *logDecoder) Read(dst []byte) (n int, err error) {

	// If we have buffered data, return it first
	if dec.offset < len(dec.buf) {
		n = copy(dst, dec.buf[dec.offset:])
		dec.offset += n
		if dec.offset >= len(dec.buf) {
			dec.buf = dec.buf[:0]
			dec.offset = 0
		}
		return n, nil
	}

	// Read next frame header
	_, err = io.ReadFull(dec.src, dec.header)
	if err == io.EOF {
		return 0, io.EOF
	}
	if err != nil {
		return 0, errors.Wrap(err, "failed to read docker log header")
	}

	// Read frame into buffer
	frameSize := binary.BigEndian.Uint32(dec.header[4:8])
	if frameSize > maxFrameSize {
		err = errors.Errorf("docker frame exceeds decoder buffer size: %d", maxFrameSize)
		return 0, err
	}

	dec.buf = dec.buf[:frameSize]
	_, err = io.ReadFull(dec.src, dec.buf)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read docker log frame")
	}

	// Copy what we can to dst
	n = copy(dst, dec.buf)
	dec.offset = n

	return n, nil
}
