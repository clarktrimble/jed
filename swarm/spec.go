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
type Spec map[string]any

// Spec builds the Docker Swarm service spec payload for a rendered jed.Spec.
// It validates the service and resolves latest versioned swarm secrets/configs,
// but does not create or update the service.
func (d *Swarm) Spec(ctx context.Context, jspec jed.Spec) (Spec, error) {
	service := jspec.Service
	if err := service.Validate(); err != nil {
		return nil, errors.Wrapf(err, "failed to validate service %q", service.Name)
	}

	resolved, err := d.resolveSecrets(ctx, service.Secrets)
	if err != nil {
		return nil, err
	}

	resolvedCfgs, err := d.resolveConfigs(ctx, service.Configs)
	if err != nil {
		return nil, err
	}

	body, err := buildSpec(jspec, resolved, resolvedCfgs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to build spec for %q", service.Name)
	}

	return body, nil
}

func buildSpec(jspec jed.Spec, secrets []resolvedSecret, configs []resolvedConfig) (Spec, error) {
	service := jspec.Service
	env := jspec.Env

	ports, err := swarmPorts(service.Ports, service.PublishMode)
	if err != nil {
		return nil, err
	}

	resources, err := resourceSpec(service.Resources.WithDefaults())
	if err != nil {
		return nil, err
	}

	uid := service.User
	if uid == "" {
		uid = "1001"
	}
	gid := uid
	if i := strings.IndexByte(uid, ':'); i >= 0 {
		gid = uid[i+1:]
		uid = uid[:i]
	}

	containerSpec := map[string]any{
		"Image":    service.Image,
		"Env":      envLines(env.Vars),
		"ReadOnly": true,
		"User":     uid + ":" + gid,
	}

	if len(service.Command) > 0 {
		containerSpec["Command"] = service.Command
	}

	if len(secrets) > 0 {
		containerSpec["Secrets"] = secretRefs(secrets, uid, gid)
	}

	if len(configs) > 0 {
		containerSpec["Configs"] = configRefs(configs, uid, gid)
	}

	if len(service.Volumes) > 0 {
		containerSpec["Mounts"] = swarmMounts(service.Volumes)
	}

	if len(service.Hosts) > 0 {
		containerSpec["Hosts"] = service.Hosts
	}

	labels := map[string]string{}
	if service.Traefik != nil {
		labels = traefikLabels(service.Name, service.Traefik)
	}
	maps.Copy(labels, service.Labels) // explicit labels win

	spec := Spec{
		"Name":   service.Name,
		"Labels": labels,
		"TaskTemplate": map[string]any{
			"ContainerSpec": containerSpec,
			"LogDriver": map[string]any{
				"Name": "json-file",
				"Options": map[string]string{
					"max-size": "10m",
					"max-file": "3",
				},
			},
			"Resources":     resources,
			"RestartPolicy": restartPolicy(service),
			"Networks": []map[string]string{
				{"Target": service.Network},
			},
		},
		"Mode": map[string]any{
			"Replicated": map[string]any{
				"Replicas": service.Replicas,
			},
		},
		"UpdateConfig": map[string]any{
			"Order":         "stop-first",
			"FailureAction": "pause",
			//"FailureAction": "rollback",
		},
	}

	if len(ports) > 0 {
		spec["EndpointSpec"] = map[string]any{
			"Ports": ports,
		}
	}

	return spec, nil
}

func secretRefs(secrets []resolvedSecret, uid, gid string) []map[string]any {

	refs := make([]map[string]any, 0, len(secrets))
	for _, s := range secrets {
		refs = append(refs, map[string]any{
			"SecretID":   s.id,
			"SecretName": s.name,
			"File": map[string]any{
				"Name": s.file,
				"UID":  uid,
				"GID":  gid,
				"Mode": 0o400,
			},
		})
	}
	return refs
}

func configRefs(configs []resolvedConfig, uid, gid string) []map[string]any {

	refs := make([]map[string]any, 0, len(configs))
	for _, c := range configs {
		refs = append(refs, map[string]any{
			"ConfigID":   c.id,
			"ConfigName": c.name,
			"File": map[string]any{
				"Name": c.target,
				"UID":  uid,
				"GID":  gid,
				"Mode": 0o400,
			},
		})
	}
	return refs
}

func swarmPorts(ports map[string]string, publishMode string) ([]map[string]any, error) {

	// Todo: consider fully specified format: src, dst, proto, mode

	result := make([]map[string]any, 0, len(ports))
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

		portSpec := map[string]any{
			"Protocol":      parts[1],
			"TargetPort":    target,
			"PublishedPort": published,
		}
		if publishMode == "host" {
			portSpec["PublishMode"] = "host"
		}

		result = append(result, portSpec)
	}
	return result, nil
}

func swarmMounts(volumes map[string]string) []map[string]string {

	mounts := make([]map[string]string, 0, len(volumes))
	for source, target := range volumes {
		mountType := "volume"
		if strings.HasPrefix(source, "/") {
			mountType = "bind"
		}
		mounts = append(mounts, map[string]string{
			"Type":   mountType,
			"Source": source,
			"Target": target,
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

func resourceSpec(r jed.Resources) (map[string]any, error) {
	cpuLimit, err := parseCPU(r.CPULimit)
	if err != nil {
		return nil, errors.Wrap(err, "cpu_limit")
	}
	cpuReserve, err := parseCPU(r.CPUReserve)
	if err != nil {
		return nil, errors.Wrap(err, "cpu_reserve")
	}
	memLimit, err := parseMem(r.MemLimit)
	if err != nil {
		return nil, errors.Wrap(err, "mem_limit")
	}
	memReserve, err := parseMem(r.MemReserve)
	if err != nil {
		return nil, errors.Wrap(err, "mem_reserve")
	}

	return map[string]any{
		"Limits": map[string]any{
			"NanoCPUs":    cpuLimit,
			"MemoryBytes": memLimit,
		},
		"Reservations": map[string]any{
			"NanoCPUs":    cpuReserve,
			"MemoryBytes": memReserve,
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

func restartPolicy(service jed.Service) map[string]any {
	condition := service.Restart.Condition
	if condition == "" {
		condition = jed.RestartNone
	}

	if condition == jed.RestartNone {
		return map[string]any{
			"Condition": "none",
		}
	}

	restart := map[string]any{
		"Condition": condition,
		"Delay":     5000000000,
	}
	if service.Restart.MaxAttempts != nil {
		restart["MaxAttempts"] = *service.Restart.MaxAttempts
	} else if condition == jed.RestartOnFailure {
		restart["MaxAttempts"] = 1
	}
	return restart
}
