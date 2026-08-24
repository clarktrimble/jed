package jed

import "strings"

// ImageConfig is an image config.
type ImageConfig struct {
	Architecture string            `json:"architecture"`
	Os           string            `json:"os"`
	User         string            `json:"user,omitempty"`
	Env          []string          `json:"env,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// Platform is an image platform and its config.
type Platform struct {
	Name   string      `json:"name"`
	Config ImageConfig `json:"config"`
}

// Image is a scanned repository tag.
type Image struct {
	Registry   string     `json:"registry"`
	Repository string     `json:"repository"`
	Tag        string     `json:"tag"`
	Platforms  []Platform `json:"platforms"`
}

// Images is a scanned image list.
type Images []Image

// Configs returns image refs and configs for a given platform.
func (images Images) Configs(platformName string) map[string]ImageConfig {
	configs := map[string]ImageConfig{}

	for _, image := range images {
		for _, imagePlatform := range image.Platforms {
			if imagePlatform.Name == platformName {
				configs[image.ImageRef()] = imagePlatform.Config
				break
			}
		}
	}

	return configs
}

// ImageRef returns the Docker image ref.
func (image Image) ImageRef() string {
	parts := []string{}
	if image.Registry != "" {
		parts = append(parts, image.Registry)
	}
	parts = append(parts, image.Repository)

	return strings.Join(parts, "/") + ":" + image.Tag
}
