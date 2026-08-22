package scanner

type ociCatalog struct {
	Repositories []string `json:"repositories"`
}

type ociTags struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type ociIndex struct {
	MediaType string                  `json:"mediaType"`
	Manifests []ociManifestDescriptor `json:"manifests"`
}

type ociManifestDescriptor struct {
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

type ociManifest struct {
	MediaType string          `json:"mediaType"`
	Config    ociDescriptor   `json:"config"`
	Layers    []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ociConfig struct {
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
