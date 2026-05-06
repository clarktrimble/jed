package swarm

import (
	"context"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// Deploy creates or updates a swarm service from a rendered jed.Spec.
func (d *Swarm) Deploy(ctx context.Context, spec jed.Spec) (id string, created bool, err error) {
	service := spec.Service

	// Todo: finding the right task for log file is flakey
	//       2026-03-05T16:22:08     trt14okg7c2s    running traefik:v3.6.9
	//       2026-03-05T16:22:08     ustlwf51japh    pending traefik:v3.6.9  no suitable node (host-mode port already in use on 1 node)
	//       (I think sort failed to discriminate and we looked at running instead of pending)
	// Todo: fix error: open whoami.env: no such file or directory (this is deploy.sh so maybe?)
	// Todo: is there a gid issue where transship needs docker gid?

	body, err := d.Spec(ctx, spec)
	if err != nil {
		return
	}

	svcInfo, verErr := d.GetService(ctx, service.Name)
	if verErr != nil {
		if !errors.Is(verErr, ErrServiceNotFound) {
			err = verErr
			return
		}

		id, err = d.createService(ctx, body)
		created = err == nil
		return
	}

	err = d.updateService(ctx, service.Name, svcInfo.Version.Index, body)
	if err != nil {
		return
	}

	id = svcInfo.ID
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
