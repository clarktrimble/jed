package swarm

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// Deploy creates or updates a swarm service from a jed.Service and jed.Env.
func (d *Deployer) Deploy(ctx context.Context, service jed.Service, env jed.Env) (id string, err error) {

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
		if !strings.Contains(verErr.Error(), "404") {
			err = verErr
			return
		}

		// Service doesn't exist, create it
		id, err = d.CreateService(ctx, spec)
		return
	}

	// Service exists, update it
	err = d.UpdateService(ctx, service.Name, svcInfo.Version.Index, spec)
	return
}

type resolvedSecret struct {
	id   string
	name string // versioned name (e.g., "s3_secret_key_v3")
	file string // base name / mount filename (e.g., "s3_secret_key")
}

func (d *Deployer) resolveSecrets(ctx context.Context, secrets []string) ([]resolvedSecret, error) {

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

	ports, err := swarmPorts(service.Ports)
	if err != nil {
		return nil, err
	}

	containerSpec := map[string]any{
		"Image":    service.Image,
		"Env":      envLines(env.Vars),
		"ReadOnly": true,
		"User":     "1001",
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

	spec := map[string]any{
		"Name":   service.Name,
		"Labels": service.Labels,
		"TaskTemplate": map[string]any{
			"ContainerSpec": containerSpec,
			"LogDriver": map[string]any{
				"Name": "json-file",
				"Options": map[string]string{
					"max-size": "10m",
					"max-file": "3",
				},
			},
			"Resources": map[string]any{
				"Limits": map[string]any{
					"NanoCPUs":    500000000,
					"MemoryBytes": 134217728,
				},
				"Reservations": map[string]any{
					"NanoCPUs":    100000000,
					"MemoryBytes": 67108864,
				},
			},
			"RestartPolicy": map[string]any{
				"Condition":   "on-failure",
				"Delay":       5000000000,
				"MaxAttempts": 3,
			},
			"Networks": []map[string]string{
				{"Target": service.Network},
			},
		},
		"Mode": map[string]any{
			"Replicated": map[string]any{
				"Replicas": 1,
			},
		},
		"UpdateConfig": map[string]any{
			"Order":         "stop-first",
			"FailureAction": "rollback",
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

func swarmPorts(ports map[string]string) ([]map[string]any, error) {

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

		result = append(result, map[string]any{
			"Protocol":      parts[1],
			"TargetPort":    target,
			"PublishedPort": published,
		})
	}
	return result, nil
}

func swarmMounts(volumes map[string]string) []map[string]string {

	mounts := make([]map[string]string, 0, len(volumes))
	for source, target := range volumes {
		mounts = append(mounts, map[string]string{
			"Type":   "bind",
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
