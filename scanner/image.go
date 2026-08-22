package scanner

type Image struct {
	//Registry   string     `json:"registry"`
	Repository string     `json:"repository"`
	Tag        string     `json:"tag"`
	Platforms  []Platform `json:"platforms"`
}

type Platform struct {
	Name   string `json:"name"`
	Config Config `json:"config"`
}

type Config struct {
	Architecture string            `json:"architecture"`
	Os           string            `json:"os"`
	User         string            `json:"user,omitempty"`
	Env          []string          `json:"env,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}
