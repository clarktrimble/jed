package scanner

import "strings"

type Config struct {
	Architecture string            `json:"architecture"`
	Os           string            `json:"os"`
	User         string            `json:"user,omitempty"`
	Env          []string          `json:"env,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

type Platform struct {
	Name   string `json:"name"`
	Config Config `json:"config"`
}

type Image struct {
	Registry   string     `json:"registry"`
	Repository string     `json:"repository"`
	Tag        string     `json:"tag"`
	Platforms  []Platform `json:"platforms"`
}

type Images []Image

func (images Images) Configs(platformName string) map[string]Config {
	configs := map[string]Config{}

	for _, image := range images {
		for _, imagePlatform := range image.Platforms {
			if imagePlatform.Name == platformName {
				configs[image.imageRef()] = imagePlatform.Config
				break
			}
		}
	}

	return configs
}

// unexported

func (image Image) imageRef() string {
	parts := []string{}
	if image.Registry != "" {
		parts = append(parts, image.Registry)
	}
	parts = append(parts, image.Repository)

	return strings.Join(parts, "/") + ":" + image.Tag
}
