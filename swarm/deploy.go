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
// TemplateVars are merged with env vars for {{VAR}} expansion in command args
// and labels, but are not passed to the container. Service env wins on collision.
func (d *Swarm) Deploy(ctx context.Context, service jed.Service, env jed.Env, templateVars map[string]string) (id string, err error) {

	// Todo: templateVars is a stretch here, look for a better pattern

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

	// Resolve configs to latest versions
	resolvedCfgs, err := d.resolveConfigs(ctx, service.Configs)
	if err != nil {
		return
	}

	// Build spec
	spec, err := buildSpec(service, env, templateVars, resolved, resolvedCfgs)
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

type resolvedConfig struct {
	id     string
	name   string // versioned name (e.g., "myapp_config_v2")
	target string // mount path (e.g., "/etc/myapp/app.conf")
}

func (d *Swarm) resolveConfigs(ctx context.Context, configs map[string]string) ([]resolvedConfig, error) {

	resolved := make([]resolvedConfig, 0, len(configs))
	for baseName, target := range configs {
		id, versionedName, err := d.ConfigLatest(ctx, baseName)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedConfig{
			id:     id,
			name:   versionedName,
			target: target,
		})
	}
	return resolved, nil
}

func buildSpec(service jed.Service, env jed.Env, templateVars map[string]string, secrets []resolvedSecret, configs []resolvedConfig) (map[string]any, error) {

	// Merge template vars: global first, service env wins on collision
	tplVars := make(map[string]string, len(templateVars)+len(env.Vars))
	maps.Copy(tplVars, templateVars)
	maps.Copy(tplVars, env.Vars)

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
		expanded, err := expandVars(service.Command, tplVars)
		if err != nil {
			return nil, err
		}
		containerSpec["Command"] = expanded
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

	labels := service.Labels
	if service.Traefik != nil {
		labels = traefikLabels(service.Name, service.Traefik)
		maps.Copy(labels, service.Labels) // explicit labels win
	}
	labels, err = expandMapVars(labels, tplVars)
	if err != nil {
		return nil, err
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

// traefikLabels generates traefik routing labels for a service.
// traefikLabels generates traefik routing labels for a service.
// PathPrefix routing at /{name} with TLS on websecure entrypoint.
// When PathPrefixStrip is true, adds stripprefix middleware.
func traefikLabels(name string, t *jed.Traefik) map[string]string {
	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", name):                      fmt.Sprintf("PathPrefix(`/%s`)", name),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", name):               "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls", name):                       "true",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", name): t.Port,
	}

	if t.PathPrefixStrip {
		labels[fmt.Sprintf("traefik.http.routers.%s.middlewares", name)] = fmt.Sprintf("%s-strip", name)
		labels[fmt.Sprintf("traefik.http.middlewares.%s-strip.stripprefix.prefixes", name)] = fmt.Sprintf("/%s", name)
	}

	return labels
}

func expandVars(args []string, vars map[string]string) ([]string, error) {
	result := make([]string, len(args))
	for i, arg := range args {
		expanded, err := expandString(arg, vars)
		if err != nil {
			return nil, err
		}
		result[i] = expanded
	}
	return result, nil
}

func expandMapVars(m map[string]string, vars map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(m))
	for k, v := range m {
		expanded, err := expandString(v, vars)
		if err != nil {
			return nil, err
		}
		result[k] = expanded
	}
	return result, nil
}

func expandString(s string, vars map[string]string) (string, error) {
	var result strings.Builder
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			result.WriteString(s)
			return result.String(), nil
		}
		end := strings.Index(s[start:], "}}")
		if end < 0 {
			result.WriteString(s)
			return result.String(), nil
		}
		name := s[start+2 : start+end]
		val, ok := vars[name]
		if !ok {
			return "", fmt.Errorf("template variable {{%s}} not found in env", name)
		}
		result.WriteString(s[:start])
		result.WriteString(val)
		s = s[start+end+2:]
	}
}
