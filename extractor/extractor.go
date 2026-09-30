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
	"github.com/clarktrimble/jed/logger"
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
	logger logger.Logger
}

// New creates an Extractor.
func New(client Client, logger logger.Logger) *Extractor {
	return &Extractor{client: client, logger: logger}
}

// Files extracts paths from imageRef using a temporary stopped container.
func (extractor *Extractor) Files(ctx context.Context, imageRef string, paths ...string) (files map[string][]byte, err error) {
	files = map[string][]byte{}
	if len(paths) == 0 {
		return
	}

	extractor.logger.Debug(ctx, "extracting files from image", "ref", imageRef, "count", len(paths))
	if err = extractor.pull(ctx, imageRef); err != nil {
		return
	}

	return extractor.files(ctx, imageRef, paths)
}

// LabelFiles extracts files named by labels on imageRef. Each requested label's
// value is treated as a file path, and the returned map is keyed by label name.
// Missing or empty labels are omitted. For example:
//
//	files, err := extractor.LabelFiles(ctx, "registry.example.com/app:v1", "com.example.env")
func (extractor *Extractor) LabelFiles(ctx context.Context, imageRef string, labels ...string) (files map[string][]byte, err error) {
	files = map[string][]byte{}
	if len(labels) == 0 {
		return
	}

	extractor.logger.Debug(ctx, "extracting label files from image", "ref", imageRef, "count", len(labels))

	// fail fast(er?) by depending on local image
	// Todo: fix propery, prolly by storing all env on discover, sigh
	//if err = extractor.pull(ctx, imageRef); err != nil {
	//return
	//}

	image, err := extractor.inspect(ctx, imageRef)
	if err != nil {
		return nil, err
	}

	pathsByLabel := make(map[string]string, len(labels))
	for _, label := range labels {
		if path := image.Labels[label]; path != "" {
			pathsByLabel[label] = path
		}
	}
	if len(pathsByLabel) == 0 {
		return files, nil
	}

	paths := make([]string, 0, len(pathsByLabel))
	seenPaths := make(map[string]bool, len(pathsByLabel))
	for _, path := range pathsByLabel {
		if !seenPaths[path] {
			paths = append(paths, path)
			seenPaths[path] = true
		}
	}

	contents, err := extractor.files(ctx, image.ID, paths)
	if err != nil {
		return nil, err
	}
	for label, path := range pathsByLabel {
		files[label] = contents[path]
	}
	return files, nil
}

type image struct {
	ID     string
	Labels map[string]string
}

func (extractor *Extractor) files(ctx context.Context, imageRef string, paths []string) (files map[string][]byte, err error) {
	files = make(map[string][]byte, len(paths))

	id, err := extractor.create(ctx, imageRef)
	if err != nil {
		return
	}
	defer func() {
		deleteErr := extractor.delete(context.WithoutCancel(ctx), id)
		if err != nil || deleteErr != nil {
			err = errors.Errorf("%v; cleanup err: %v", err, deleteErr)
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

	extractor.logger.Debug(ctx, "pulling image", "ref", imageRef)

	path := fmt.Sprintf("/images/create?fromImage=%s&tag=%s", url.QueryEscape(fromImage), url.QueryEscape(tag))
	_, err = extractor.client.SendJson(ctx, "POST", path, nil)
	return
}

func (extractor *Extractor) inspect(ctx context.Context, imageRef string) (result image, err error) {
	var response struct {
		ID     string `json:"Id"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}

	path := fmt.Sprintf("/images/%s/json", url.PathEscape(imageRef))
	if err = extractor.client.SendObject(ctx, "GET", path, nil, &response); err != nil {
		return result, err
	}
	if response.ID == "" {
		return result, errors.Errorf("image inspection for %q returned no ID", imageRef)
	}
	return image{ID: response.ID, Labels: response.Config.Labels}, nil
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
	extractor.logger.Debug(ctx, "created extraction container", "ref", imageRef, "name", name, "id", id)
	return
}

func (extractor *Extractor) delete(ctx context.Context, id string) error {

	extractor.logger.Debug(ctx, "deleting extraction container", "id", id)
	path := fmt.Sprintf("/containers/%s", url.PathEscape(id))
	return extractor.client.SendObject(ctx, "DELETE", path, nil, nil)
}

func (extractor *Extractor) file(ctx context.Context, id, filePath string) (file []byte, err error) {

	extractor.logger.Debug(ctx, "extracting file from container", "id", id, "path", filePath)
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
