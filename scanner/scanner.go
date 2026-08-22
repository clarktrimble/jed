package scanner

import (
	"context"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) (err error)
}

type Scanner struct {
	client client
	limit  int
}

func New(client client) *Scanner {
	return &Scanner{
		client: client,
		limit:  8,
	}
}

func (scanner *Scanner) platforms(ctx context.Context, repository, tag string) (platforms []Platform, err error) {

	references, err := scanner.getReferences(ctx, repository, tag)
	if err != nil {
		return
	}
	/*
		if len(references) == 0 {
			err = errors.New("registry tag has no manifest references")
			return
		}
	*/

	for _, reference := range references {
		cfg, err := scanner.getConfig(ctx, repository, reference)
		if err != nil {
			return platforms, err
		}

		platforms = append(platforms, Platform{
			Name:   reference.Platform,
			Config: cfg,
		})
	}
	return
}

func (scanner *Scanner) Images(ctx context.Context) (images []Image, err error) {

	repositories, err := scanner.getRepositories(ctx)
	if err != nil {
		return
	}

	/*
		type repositoryImages struct {
			Repository string
			Images     []Image
		}
	*/

	//tagses := [][]string{}
	tagses := make([][]string, len(repositories))

	// Todo: decide whether errgroup.WithContext cancellation is useful here or just hides root errors.
	var eg errgroup.Group
	eg.SetLimit(scanner.limit)

	for i, repository := range repositories {
		eg.Go(func() error {
			tags, err := scanner.getTags(ctx, repository)
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

	/*
		for _, repository := range found {
			for _, tag := range repository.Tags {
				images = append(images, Image{
					Repository: repository.Repository,
					Tag:        tag,
				})
			}
		}
	*/

	/*
		offsets := make([]int, len(repositories))
		for i, repo := range repositories {
			if i > 0 {
				offsets[i] = offsets[i-1] + len(tagsByRepo[repositories[i-1]])
			}
			images = append(images, make([]Image, len(tagsByRepo[repo]))...)
		}
	*/

	for _, repo := range repositories {
		for _, tag := range tagsByRepo[repo] {
			images = append(images, Image{
				Repository: repo,
				Tag:        tag,
			})
		}
	}

	eg = errgroup.Group{}
	eg.SetLimit(scanner.limit)

	for i, image := range images {
		eg.Go(func() error {
			platforms, err := scanner.platforms(ctx, image.Repository, image.Tag)
			if err != nil {
				// only wrap once mother fucker
				//return errors.Wrapf(err, "scan %s:%s", image.Repository, image.Tag)
				return err
			}
			images[i].Platforms = platforms
			return nil
		})
	}

	err = eg.Wait()
	return
}

/*
func (scanner *Scanner) tags(ctx context.Context, repository string) (images []Image, err error) {
	tags, err := scanner.getTags(ctx, repository)
	if err != nil {
		return
	}

	images = make([]Image, len(tags))
	for i, tag := range tags {
		images[i] = Image{
			Repository: repository,
			Tag:        tag,
		}
	}
	return
}
*/
