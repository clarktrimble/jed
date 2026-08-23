package scanner

import (
	"context"
	"net/url"

	"github.com/clarktrimble/jed/logger"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) (err error)
	Uri() string
}

type Scanner struct {
	client   Client
	logger   logger.Logger
	limit    int
	registry string
}

func New(client Client, logger logger.Logger) (scanner *Scanner, err error) {

	uri := client.Uri()
	parsed, err := url.Parse(uri)
	if err != nil {
		err = errors.Wrapf(err, "parse registry uri %q", uri)
		return
	}

	scanner = &Scanner{
		client:   client,
		logger:   logger,
		limit:    8,
		registry: parsed.Host,
	}

	return
}

// Scan scans a registry for image infos.
func (scanner *Scanner) Scan(ctx context.Context) (images []Image, err error) {

	repositories, err := scanner.getRepositories(ctx)
	if err != nil {
		return
	}

	tagses := make([][]string, len(repositories))

	eg, groupCtx := errgroup.WithContext(ctx)
	eg.SetLimit(scanner.limit)

	for i, repository := range repositories {
		eg.Go(func() error {
			tags, err := scanner.getTags(groupCtx, repository)
			if err != nil {
				return errors.Wrapf(err, "list tags for %s", repository)
			}

			tagses[i] = tags
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return
	}

	tagsByRepo := map[string][]string{}
	for i, repo := range repositories {
		tagsByRepo[repo] = tagses[i]
	}

	for _, repo := range repositories {
		for _, tag := range tagsByRepo[repo] {
			images = append(images, Image{
				Registry:   scanner.registry,
				Repository: repo,
				Tag:        tag,
			})
		}
	}

	eg, groupCtx = errgroup.WithContext(ctx)
	eg.SetLimit(scanner.limit)

	for i, image := range images {
		eg.Go(func() error {
			platforms, err := scanner.platforms(groupCtx, image.Repository, image.Tag)
			if err != nil {
				return err
			}
			images[i].Platforms = platforms
			return nil
		})
	}

	err = eg.Wait()
	return
}

func (scanner *Scanner) platforms(ctx context.Context, repository, tag string) (platforms []Platform, err error) {

	references, err := scanner.getReferences(ctx, repository, tag)
	if err != nil {
		return
	}

	for _, reference := range references {
		cfg, err := scanner.getConfig(ctx, repository, reference)
		if err != nil {
			return platforms, err
		}

		platform := reference.Platform
		if platform == "" {
			// Direct manifest
			platform = platformFromConfig(cfg)
		}
		if platform == "" {
			scanner.logger.Error(ctx, "failed to determine platform for direct manifest",
				errors.New("missing platform"),
				"repository", repository,
				"tag", tag,
				"manifest", reference.Digest,
			)
			continue
		}

		platforms = append(platforms, Platform{
			Name:   platform,
			Config: cfg,
		})
	}
	return
}
