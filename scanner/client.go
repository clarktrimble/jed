package scanner

import (
	"context"
	"fmt"
	"net/http"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

const (
	ociIndexMt       = "application/vnd.oci.image.index.v1+json"
	ociManifestMt    = "application/vnd.oci.image.manifest.v1+json"
	dockerListMt     = "application/vnd.docker.distribution.manifest.list.v2+json"
	dockerManifestMt = "application/vnd.docker.distribution.manifest.v2+json"
)

type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) (err error)
	Uri() string
}

type manifestReference struct {
	Platform string
	Digest   string
}

type ociCatalog struct {
	Repositories []string `json:"repositories"`
}

type ociTags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type ociTagManifest struct {
	MediaType string               `json:"mediaType"`
	Manifests []ociIndexedManifest `json:"manifests"`
	Config    ociDescriptor        `json:"config"`
}

type ociIndexedManifest struct {
	MediaType string      `json:"mediaType"`
	Digest    string      `json:"digest"`
	Size      int64       `json:"size"`
	Platform  ociPlatform `json:"platform"`
}

type ociPlatform struct {
	Architecture string `json:"architecture"`
	Os           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type ociImageManifest struct {
	MediaType string          `json:"mediaType"`
	Config    ociDescriptor   `json:"config"`
	Layers    []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociImageConfig struct {
	Architecture string           `json:"architecture"`
	Os           string           `json:"os"`
	Config       ociRuntimeConfig `json:"config"`
}

type ociRuntimeConfig struct {
	User       string            `json:"User,omitempty"`
	Env        []string          `json:"Env,omitempty"`
	Entrypoint []string          `json:"Entrypoint,omitempty"`
	WorkingDir string            `json:"WorkingDir,omitempty"`
	Labels     map[string]string `json:"Labels,omitempty"`
}

func (scanner *Scanner) getRepositories(ctx context.Context) (repositories []string, err error) {
	var catalog ociCatalog

	err = scanner.client.SendObject(ctx, http.MethodGet, "/v2/_catalog", nil, &catalog)
	if err != nil {
		return
	}

	repositories = catalog.Repositories
	scanner.logger.Debug(ctx, "listed repositories", "count", len(repositories))
	return
}

func (scanner *Scanner) getTags(ctx context.Context, repository string) (tags []string, err error) {
	var rsp ociTags
	path := fmt.Sprintf("/v2/%s/tags/list", repository)

	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &rsp)
	if err != nil {
		return
	}

	tags = rsp.Tags
	scanner.logger.Debug(ctx, "listed repository tags", "repository", repository, "count", len(tags))
	return
}

func (scanner *Scanner) getReferences(ctx context.Context, repository, tag string) (references []manifestReference, err error) {

	var idx ociTagManifest
	path := fmt.Sprintf("/v2/%s/manifests/%s", repository, tag)

	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &idx)
	if err != nil {
		return
	}

	switch idx.MediaType {
	case ociIndexMt, dockerListMt:
		// indexed manifests
		for _, m := range idx.Manifests {

			platform := platformFromSpec(m.Platform)
			if platform == "" {
				continue
			}
			references = append(references, manifestReference{
				Platform: platform,
				Digest:   m.Digest,
			})
		}
	case ociManifestMt, dockerManifestMt:
		// direct manifest
		references = append(references, manifestReference{
			Digest: tag,
		})
	}

	if len(references) == 0 {
		scanner.logger.Error(ctx, "failed to get references",
			errors.New("no manifest references found"),
			"repository", repository,
			"tag", tag,
			"media_type", idx.MediaType,
		)
	}

	scanner.logger.Debug(ctx, "resolved manifest references", "repository", repository, "tag", tag, "count", len(references))
	return
}

func (scanner *Scanner) getConfig(ctx context.Context, repository string, reference manifestReference) (cfg jed.ImageConfig, err error) {
	var mfst ociImageManifest
	path := fmt.Sprintf("/v2/%s/manifests/%s", repository, reference.Digest)
	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &mfst)
	if err != nil {
		return
	}

	var ociCfg ociImageConfig
	path = fmt.Sprintf("/v2/%s/blobs/%s", repository, mfst.Config.Digest)
	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &ociCfg)
	if err != nil {
		return
	}

	cfg = jed.ImageConfig{
		Architecture: ociCfg.Architecture,
		Os:           ociCfg.Os,
		User:         ociCfg.Config.User,
		Env:          ociCfg.Config.Env,
		Entrypoint:   ociCfg.Config.Entrypoint,
		WorkingDir:   ociCfg.Config.WorkingDir,
		Labels:       ociCfg.Config.Labels,
	}
	return
}

func platformFromConfig(cfg jed.ImageConfig) string {
	return platformFromSpec(ociPlatform{
		Os:           cfg.Os,
		Architecture: cfg.Architecture,
	})
}

func platformFromSpec(spec ociPlatform) string {
	if spec.Os == "" || spec.Architecture == "" {
		return ""
	}
	if spec.Os == "unknown" && spec.Architecture == "unknown" {
		return ""
	}

	platform := spec.Os + "/" + spec.Architecture
	if spec.Variant != "" {
		platform += "/" + spec.Variant
	}
	return platform
}
