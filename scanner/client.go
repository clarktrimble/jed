package scanner

import (
	"context"
	"fmt"
	"net/http"
)

type manifestReference struct {
	Platform string
	Digest   string
}

func (scanner *Scanner) getRepositories(ctx context.Context) (repositories []string, err error) {
	var catalog ociCatalog

	err = scanner.client.SendObject(ctx, http.MethodGet, "/v2/_catalog", nil, &catalog)
	if err != nil {
		return
	}

	repositories = catalog.Repositories
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
	return
}

func (scanner *Scanner) getReferences(ctx context.Context, repository, tag string) (references []manifestReference, err error) {

	var idx ociIndex
	path := fmt.Sprintf("/v2/%s/manifests/%s", repository, tag)

	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &idx)
	if err != nil {
		return
	}

	for _, m := range idx.Manifests {

		platform := platformFromOCI(m.Platform)
		if platform == "" {
			continue
		}
		references = append(references, manifestReference{
			Platform: platform,
			Digest:   m.Digest,
		})
	}

	return
}

func (scanner *Scanner) getConfig(ctx context.Context, repository string, reference manifestReference) (cfg Config, err error) {
	var mfst ociManifest
	path := fmt.Sprintf("/v2/%s/manifests/%s", repository, reference.Digest)
	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &mfst)
	if err != nil {
		return
	}

	var ociCfg ociConfig
	path = fmt.Sprintf("/v2/%s/blobs/%s", repository, mfst.Config.Digest)
	err = scanner.client.SendObject(ctx, http.MethodGet, path, nil, &ociCfg)
	if err != nil {
		return
	}

	cfg = Config{
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

func platformFromOCI(spec ociPlatform) string {
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
