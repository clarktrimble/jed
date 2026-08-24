// Package extractor extracts files from Docker images.
package extractor

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/clarktrimble/hondo"
	"github.com/pkg/errors"
)

// Client is an HTTP client for the Docker API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) error
	SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
}

// Extractor extracts files from Docker images.
type Extractor struct {
	client Client
}

// New creates an Extractor.
func New(client Client) *Extractor {
	return &Extractor{client: client}
}

// Files extracts paths from imageRef using a temporary stopped container.
func (extractor *Extractor) Files(ctx context.Context, imageRef string, paths ...string) (files map[string][]byte, err error) {

	err = extractor.pull(ctx, imageRef)
	if err != nil {
		return
	}

	id, err := extractor.create(ctx, imageRef)
	if err != nil {
		return
	}
	defer func() {
		deleteErr := extractor.delete(ctx, id)
		if err == nil {
			err = deleteErr
		}
	}()

	files = map[string][]byte{}
	for _, path := range paths {
		files[path], err = extractor.file(ctx, id, path)
		if err != nil {
			return
		}
	}

	return
}

func (extractor *Extractor) pull(ctx context.Context, imageRef string) (err error) {

	fromImage, tag, err := splitImageRef(imageRef)
	if err != nil {
		return
	}

	path := fmt.Sprintf("/images/create?fromImage=%s&tag=%s", url.QueryEscape(fromImage), url.QueryEscape(tag))
	_, err = extractor.client.SendJson(ctx, "POST", path, nil)
	return
}

func (extractor *Extractor) create(ctx context.Context, imageRef string) (id string, err error) {

	var response struct {
		Id string `json:"Id"`
	}

	name := "jed-extract-" + hondo.Rand(7)
	path := fmt.Sprintf("/containers/create?name=%s", url.QueryEscape(name))
	err = extractor.client.SendObject(ctx, "POST", path, map[string]string{"Image": imageRef}, &response)
	if err != nil {
		return
	}

	id = response.Id
	return
}

func (extractor *Extractor) delete(ctx context.Context, id string) error {

	path := fmt.Sprintf("/containers/%s", url.PathEscape(id))
	return extractor.client.SendObject(ctx, "DELETE", path, nil, nil)
}

func (extractor *Extractor) file(ctx context.Context, id, filePath string) ([]byte, error) {

	path := fmt.Sprintf("/containers/%s/archive?path=%s", url.PathEscape(id), url.QueryEscape(filePath))
	archive, err := extractor.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	file, err := readArchiveFile(archive)
	if err != nil {
		return nil, errors.Wrapf(err, "read archive for %s", filePath)
	}
	return file, nil
}

func splitImageRef(imageRef string) (fromImage, tag string, err error) {

	// Todo: look at just getting this value from Image yeah
	// Only a colon after the last slash separates the tag; earlier colons
	// may belong to the registry host, e.g. localhost:5000/app:v1.
	lastSlash := strings.LastIndex(imageRef, "/")
	lastColon := strings.LastIndex(imageRef, ":")
	if lastColon <= lastSlash {
		err = errors.Errorf("image ref %q has no tag", imageRef)
		return
	}

	fromImage = imageRef[:lastColon]
	tag = imageRef[lastColon+1:]
	return
}

func readArchiveFile(data []byte) ([]byte, error) {

	tr := tar.NewReader(bytes.NewReader(data))

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("archive contains no regular file")
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		file, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}
