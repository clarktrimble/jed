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

// Deploy creates or updates a swarm service from a jed.Service and jed.Env.
func (d *Swarm) Deploy(ctx context.Context, service jed.Service, env jed.Env) (id string, err error) {

	// Todo: validate service rather than crashing around
	// Todo: honor restart from service yaml, we're ignoring it
	// Todo: finding the right task for log file is flakey
	//       2026-03-05T16:22:08     trt14okg7c2s    running traefik:v3.6.9
	//       2026-03-05T16:22:08     ustlwf51japh    pending traefik:v3.6.9  no suitable node (host-mode port already in use on 1 node)
	//       (I think sort failed to discriminate and we looked at running instead of pending)
	// Todo: fix error: open whoami.env: no such file or directory (this is deploy.sh so maybe?)
	// Todo: is there a gid issue where transship needs docker gid?

	// Resolve secrets to latest versions
	resolved, err := d.resolveSecrets(ctx, service.Secrets)
	if err != nil {
		return
	}

	// Build spec
	spec, err := buildSpec(service, env, resolved)
	if err != nil {
		err = errors.Wrapf(err, "failed to build spec for %q", service.Name)
		return
	}

	// Check if service exists
	svcInfo, verErr := d.GetService(ctx, service.Name)
	if verErr != nil {
		// Todo: more explicit / less fragile detection
		if !strings.Contains(verErr.Error(), "not found") {
			err = verErr
			return
		}

		// Service doesn't exist, create it
		id, err = d.createService(ctx, spec)
		return
	}

	// Service exists, update it
	err = d.updateService(ctx, service.Name, svcInfo.Version.Index, spec)
	return
}

type resolvedSecret struct {
	id   string
	name string // versioned name (e.g., "s3_secret_key_v3")
	file string // base name / mount filename (e.g., "s3_secret_key")
}

func (d *Swarm) resolveSecrets(ctx context.Context, secrets []string) ([]resolvedSecret, error) {

	resolved := make([]resolvedSecret, 0, len(secrets))
	for _, name := range secrets {
		id, versionedName, err := d.SecretLatest(ctx, name)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedSecret{
			id:   id,
			name: versionedName,
			file: name,
		})
	}
	return resolved, nil
}

func buildSpec(service jed.Service, env jed.Env, secrets []resolvedSecret) (map[string]any, error) {

	ports, err := swarmPorts(service.Ports, service.PublishMode)
	if err != nil {
		return nil, err
	}

	resources, err := resourceSpec(service.Resources.WithDefaults())
	if err != nil {
		return nil, err
	}

	user := service.User
	if user == "" {
		user = "1001"
	}

	containerSpec := map[string]any{
		"Image":    service.Image,
		"Env":      envLines(env.Vars),
		"ReadOnly": true,
		"User":     user,
	}

	if len(service.Command) > 0 {
		containerSpec["Command"] = service.Command
	}

	if len(secrets) > 0 {
		containerSpec["Secrets"] = secretRefs(secrets)
	}

	if len(service.Volumes) > 0 {
		containerSpec["Mounts"] = swarmMounts(service.Volumes)
	}

	if len(service.Hosts) > 0 {
		containerSpec["Hosts"] = service.Hosts
	}

	labels := service.Labels
	if service.Traefik != nil {
		labels = traefikLabels(service.Name, service.Traefik)
		maps.Copy(labels, service.Labels) // explicit labels win
	}

	spec := map[string]any{
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
			"Resources": resources,
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

func secretRefs(secrets []resolvedSecret) []map[string]any {

	refs := make([]map[string]any, 0, len(secrets))
	for _, s := range secrets {
		refs = append(refs, map[string]any{
			"SecretID":   s.id,
			"SecretName": s.name,
			"File": map[string]any{
				"Name": s.file,
				"UID":  "1001",
				"GID":  "1001",
				"Mode": 292,
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
	if !service.RestartService {
		return map[string]any{
			"Condition": "none",
		}
	}

	n := 1
	if service.RestartAttempts != nil {
		n = *service.RestartAttempts
	}
	return map[string]any{
		"Condition":   "on-failure",
		"Delay":       5000000000,
		"MaxAttempts": n,
	}
}

// traefikLabels generates traefik routing labels for a service.
// Stripped PathPrefix routing at /{name} with TLS on websecure entrypoint,
func traefikLabels(name string, t *jed.Traefik) map[string]string {
	return map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", name):                           fmt.Sprintf("PathPrefix(`/%s`)", name),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", name):                    "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls", name):                            "true",
		fmt.Sprintf("traefik.http.routers.%s.middlewares", name):                    fmt.Sprintf("%s-strip", name),
		fmt.Sprintf("traefik.http.middlewares.%s-strip.stripprefix.prefixes", name): fmt.Sprintf("/%s", name),
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", name):      t.Port,
	}
}
