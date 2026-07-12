package swarm

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// Spec is the Docker Swarm service spec payload Jed will create or update.
type Spec struct {
	Name         string            `json:"Name"`
	Labels       map[string]string `json:"Labels"`
	TaskTemplate TaskTemplate      `json:"TaskTemplate"`
	Mode         Mode              `json:"Mode"`
	UpdateConfig UpdateConfig      `json:"UpdateConfig"`
	EndpointSpec *EndpointSpec     `json:"EndpointSpec,omitempty"`
}

// TaskTemplate is the Docker swarm task definition within a service spec.
type TaskTemplate struct {
	ContainerSpec ContainerSpec `json:"ContainerSpec"`
	LogDriver     LogDriver     `json:"LogDriver"`
	Resources     Resources     `json:"Resources"`
	RestartPolicy RestartPolicy `json:"RestartPolicy"`
	Networks      []Network     `json:"Networks"`
	// ForceUpdate is carried forward from Docker's current spec on update and
	// bumped by Restart; omitted (treated as zero) when unset.
	ForceUpdate uint64 `json:"ForceUpdate,omitempty"`
}

// LogDriver configures container log handling.
type LogDriver struct {
	Name    string            `json:"Name"`
	Options map[string]string `json:"Options"`
}

// Resources holds CPU and memory limits and reservations.
type Resources struct {
	Limits       ResourceValues `json:"Limits"`
	Reservations ResourceValues `json:"Reservations"`
}

// ResourceValues is a CPU/memory pair in Docker's nano-CPU and byte units.
type ResourceValues struct {
	NanoCPUs    int64 `json:"NanoCPUs"`
	MemoryBytes int64 `json:"MemoryBytes"`
}

// RestartPolicy controls task restart behavior. Delay and MaxAttempts are
// omitted for the "none" condition.
type RestartPolicy struct {
	Condition   string `json:"Condition"`
	Delay       int64  `json:"Delay,omitempty"`
	MaxAttempts int    `json:"MaxAttempts,omitempty"`
}

// Network attaches the task to a Docker network by name.
type Network struct {
	Target string `json:"Target"`
}

// Mode is the service replication mode.
type Mode struct {
	Replicated Replicated `json:"Replicated"`
}

// Replicated sets the desired instance count. Replicas has no omitempty so a
// legitimate scaled-to-zero (0) is still emitted.
type Replicated struct {
	Replicas int `json:"Replicas"`
}

// UpdateConfig controls rolling-update behavior.
type UpdateConfig struct {
	Order         string `json:"Order"`
	FailureAction string `json:"FailureAction"`
}

// EndpointSpec publishes service ports.
type EndpointSpec struct {
	Ports []Port `json:"Ports"`
}

// Port is a single published port mapping.
type Port struct {
	Protocol      string `json:"Protocol"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	PublishMode   string `json:"PublishMode,omitempty"`
}

// ContainerSpec is the Docker container configuration within a swarm task.
// Fields that Jed omits when empty carry omitempty; Image/Env/ReadOnly/User
// are always emitted.
type ContainerSpec struct {
	Image    string      `json:"Image"`
	Env      []string    `json:"Env"`
	ReadOnly bool        `json:"ReadOnly"`
	User     string      `json:"User"`
	Command  []string    `json:"Command,omitempty"`
	Secrets  []SecretRef `json:"Secrets,omitempty"`
	Configs  []ConfigRef `json:"Configs,omitempty"`
	Mounts   []Mount     `json:"Mounts,omitempty"`
	Hosts    []string    `json:"Hosts,omitempty"`
	Groups   []string    `json:"Groups,omitempty"`
}

// SecretRef mounts a versioned swarm secret into the container.
type SecretRef struct {
	SecretID   string  `json:"SecretID"`
	SecretName string  `json:"SecretName"`
	File       FileRef `json:"File"`
}

// ConfigRef mounts a versioned swarm config into the container.
type ConfigRef struct {
	ConfigID   string  `json:"ConfigID"`
	ConfigName string  `json:"ConfigName"`
	File       FileRef `json:"File"`
}

// FileRef is the on-disk target for a mounted secret or config.
type FileRef struct {
	Name string `json:"Name"`
	UID  string `json:"UID"`
	GID  string `json:"GID"`
	Mode int    `json:"Mode"`
}

// Mount is a volume or bind mount for the container.
type Mount struct {
	Type   string `json:"Type"`
	Source string `json:"Source"`
	Target string `json:"Target"`
}

// Spec builds the Docker Swarm service spec payload for a rendered jed.Spec.
// It validates the service and resolves latest versioned swarm secrets/configs,
// but does not create or update the service.
func (d *Swarm) Spec(ctx context.Context, jspec jed.Spec) (Spec, error) {
	service := jspec.Service
	if err := service.Validate(); err != nil {
		return Spec{}, errors.Wrapf(err, "failed to validate service %q", service.Name)
	}

	resolved, err := d.resolveSecrets(ctx, service.Secrets)
	if err != nil {
		return Spec{}, err
	}

	resolvedCfgs, err := d.resolveConfigs(ctx, service.Configs)
	if err != nil {
		return Spec{}, err
	}

	body, err := buildSpec(jspec, resolved, resolvedCfgs)
	if err != nil {
		return Spec{}, errors.Wrapf(err, "failed to build spec for %q", service.Name)
	}

	return body, nil
}

func buildSpec(jspec jed.Spec, secrets []resolvedSecret, configs []resolvedConfig) (Spec, error) {
	service := jspec.Service
	env := jspec.Env

	ports, err := swarmPorts(service.Ports, service.PublishMode)
	if err != nil {
		return Spec{}, err
	}

	resources, err := resourceSpec(service.Resources.WithDefaults())
	if err != nil {
		return Spec{}, err
	}

	uid := service.User
	gid := uid
	if i := strings.IndexByte(uid, ':'); i >= 0 {
		gid = uid[i+1:]
		uid = uid[:i]
	}

	containerSpec := ContainerSpec{
		Image:    service.Image,
		Env:      envLines(env.Vars),
		ReadOnly: true,
		User:     uid + ":" + gid,
		Command:  service.Command,
		Hosts:    service.Hosts,
		Groups:   service.Groups,
	}

	if len(secrets) > 0 {
		containerSpec.Secrets = secretRefs(secrets, uid, gid)
	}

	if len(configs) > 0 {
		containerSpec.Configs = configRefs(configs, uid, gid)
	}

	if len(service.Volumes) > 0 {
		containerSpec.Mounts = swarmMounts(service.Volumes)
	}

	labels := map[string]string{}
	if service.Traefik != nil {
		labels = traefikLabels(service.Name, service.Traefik)
	}
	maps.Copy(labels, service.Labels) // explicit labels win

	spec := Spec{
		Name:   service.Name,
		Labels: labels,
		TaskTemplate: TaskTemplate{
			ContainerSpec: containerSpec,
			LogDriver: LogDriver{
				Name: "json-file",
				Options: map[string]string{
					"max-size": "10m",
					"max-file": "3",
				},
			},
			Resources:     resources,
			RestartPolicy: restartPolicy(service),
			Networks: []Network{
				{Target: service.Network},
			},
		},
		Mode: Mode{
			Replicated: Replicated{
				Replicas: service.Replicas,
			},
		},
		UpdateConfig: UpdateConfig{
			Order:         "stop-first",
			FailureAction: "pause",
			//FailureAction: "rollback",
		},
	}

	if len(ports) > 0 {
		spec.EndpointSpec = &EndpointSpec{
			Ports: ports,
		}
	}

	return spec, nil
}

func secretRefs(secrets []resolvedSecret, uid, gid string) []SecretRef {

	refs := make([]SecretRef, 0, len(secrets))
	for _, s := range secrets {
		refs = append(refs, SecretRef{
			SecretID:   s.id,
			SecretName: s.name,
			File: FileRef{
				Name: s.file,
				UID:  uid,
				GID:  gid,
				Mode: 0o400,
			},
		})
	}
	return refs
}

func configRefs(configs []resolvedConfig, uid, gid string) []ConfigRef {

	refs := make([]ConfigRef, 0, len(configs))
	for _, c := range configs {
		refs = append(refs, ConfigRef{
			ConfigID:   c.id,
			ConfigName: c.name,
			File: FileRef{
				Name: c.target,
				UID:  uid,
				GID:  gid,
				Mode: 0o400,
			},
		})
	}
	return refs
}

func swarmPorts(ports map[string]string, publishMode string) ([]Port, error) {

	// Todo: consider fully specified format: src, dst, proto, mode

	result := make([]Port, 0, len(ports))
	for containerPort, hostPort := range ports {
		parts := strings.SplitN(containerPort, "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid port format %q, expected port/protocol", containerPort)
		}

		target, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid target port %q: %w", parts[0], err)
		}

		published, err := strconv.Atoi(hostPort)
		if err != nil {
			return nil, fmt.Errorf("invalid published port %q: %w", hostPort, err)
		}

		portSpec := Port{
			Protocol:      parts[1],
			TargetPort:    target,
			PublishedPort: published,
		}
		if publishMode == "host" {
			portSpec.PublishMode = "host"
		}

		result = append(result, portSpec)
	}
	return result, nil
}

func swarmMounts(volumes map[string]string) []Mount {

	mounts := make([]Mount, 0, len(volumes))
	for source, target := range volumes {
		mountType := "volume"
		if strings.HasPrefix(source, "/") {
			mountType = "bind"
		}
		mounts = append(mounts, Mount{
			Type:   mountType,
			Source: source,
			Target: target,
		})
	}
	return mounts
}

func envLines(vars map[string]string) []string {

	lines := make([]string, 0, len(vars))
	for key, value := range vars {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	return lines
}

func resourceSpec(r jed.Resources) (Resources, error) {
	cpuLimit, err := parseCPU(r.CPULimit)
	if err != nil {
		return Resources{}, errors.Wrap(err, "cpu_limit")
	}
	cpuReserve, err := parseCPU(r.CPUReserve)
	if err != nil {
		return Resources{}, errors.Wrap(err, "cpu_reserve")
	}
	memLimit, err := parseMem(r.MemLimit)
	if err != nil {
		return Resources{}, errors.Wrap(err, "mem_limit")
	}
	memReserve, err := parseMem(r.MemReserve)
	if err != nil {
		return Resources{}, errors.Wrap(err, "mem_reserve")
	}

	return Resources{
		Limits: ResourceValues{
			NanoCPUs:    cpuLimit,
			MemoryBytes: memLimit,
		},
		Reservations: ResourceValues{
			NanoCPUs:    cpuReserve,
			MemoryBytes: memReserve,
		},
	}, nil
}

func parseCPU(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errors.Errorf("invalid cpu value %q", s)
	}
	return int64(f * 1_000_000_000), nil
}

func parseMem(s string) (int64, error) {
	if !strings.HasSuffix(s, "M") {
		return 0, errors.Errorf("memory value %q must have M suffix", s)
	}
	n, err := strconv.ParseInt(strings.TrimSuffix(s, "M"), 10, 64)
	if err != nil {
		return 0, errors.Errorf("invalid memory value %q", s)
	}
	return n * 1024 * 1024, nil
}

func restartPolicy(service jed.Service) RestartPolicy {
	condition := service.Restart
	if condition == "" {
		condition = jed.RestartNone
	}

	if condition == jed.RestartNone {
		return RestartPolicy{
			Condition: "none",
		}
	}

	restart := RestartPolicy{
		Condition: condition,
		Delay:     5000000000,
	}
	if condition == jed.RestartOnFailure {
		restart.MaxAttempts = 1
	}
	return restart
}
