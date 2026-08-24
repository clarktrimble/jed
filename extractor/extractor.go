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

	// Todo: revisit when we have real use from upstream
	files = map[string][]byte{}
	if len(paths) == 0 {
		return
	}

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
		if err != nil || deleteErr != nil {
			// somewhat awkward, but quite workable
			err = errors.Errorf("%v; cleanup err: %v", err, deleteErr)
			return
		}
	}()

	for _, path := range paths {
		files[path], err = extractor.file(ctx, id, path)
		if err != nil {
			return
		}
	}

	return
}

// unexported

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

func (extractor *Extractor) file(ctx context.Context, id, filePath string) (file []byte, err error) {

	path := fmt.Sprintf("/containers/%s/archive?path=%s", url.PathEscape(id), url.QueryEscape(filePath))
	archive, err := extractor.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		return
	}

	// Todo: ask giant for a way to get a reader in the first place
	file, err = readArchiveFile(bytes.NewReader(archive))
	return
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

func readArchiveFile(reader io.Reader) (file []byte, err error) {

	// currently we return the first file if we were handed a dir
	// Todo: consider validating, probably by examining X-Docker-Container-Path-Stat header in response
	// Note: passing in reader here in case that helps bad archive

	tr := tar.NewReader(reader)
	var hdr *tar.Header

	for {
		hdr, err = tr.Next()
		if errors.Is(err, io.EOF) {
			err = errors.New("archive contains no regular file")
			return
		}
		if err != nil {
			err = errors.Wrapf(err, "failed to read tar header")
			return
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		file, err = io.ReadAll(tr)
		if err != nil {
			err = errors.Wrapf(err, "failed to read file from archive")
			return
		}

		return
	}
}
