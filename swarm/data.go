package swarm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Secret represents a Docker Swarm secret.
type Secret struct {
	// ID is the Docker secret ID.
	ID string

	// Name is the Docker secret name. Secrets created by CreateSecret are named {base}_v{N}.
	Name string
}

// Config represents a Docker Swarm config.
type Config struct {
	// ID is the Docker config ID.
	ID string

	// Name is the Docker config name. Configs created by CreateConfig are named {base}_v{N}.
	Name string
}

// SecretLatest returns the ID and versioned name of the latest secret by base name.
// Looks for secrets matching {name}_v{N} and returns the highest version.
func (d *Swarm) SecretLatest(ctx context.Context, name string) (id, versionedName string, err error) {

	secrets, err := d.ListSecrets(ctx)
	if err != nil {
		return "", "", err
	}

	return findLatest(secretsToItems(secrets), name, "secret")
}

// ConfigLatest returns the ID and versioned name of the latest config by base name.
// Looks for configs matching {name}_v{N} and returns the highest version.
func (d *Swarm) ConfigLatest(ctx context.Context, name string) (id, versionedName string, err error) {
	// Todo: add test data with configs and test this.

	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return "", "", err
	}

	return findLatest(configsToItems(configs), name, "config")
}

// ListSecrets returns all secrets.
func (d *Swarm) ListSecrets(ctx context.Context) ([]Secret, error) {

	var secrets []namedResource
	err := d.client.SendObject(ctx, "GET", "/v1.52/secrets", nil, &secrets)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list secrets")
	}

	result := make([]Secret, len(secrets))
	for i, s := range secrets {
		result[i] = Secret{
			ID:   s.ID,
			Name: s.Spec.Name,
		}
	}

	return result, nil
}

// CreateSecret creates a versioned secret and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Swarm) CreateSecret(ctx context.Context, name string, value []byte) (string, error) {

	secrets, err := d.ListSecrets(ctx)
	if err != nil {
		return "", err
	}

	versionedName := fmt.Sprintf("%s_v%d", name, findNextVersion(secretsToItems(secrets), name))
	req := dataCreate{
		Name: versionedName,
		Data: base64.StdEncoding.EncodeToString(value),
	}

	var resp IDResponse
	err = d.client.SendObject(ctx, "POST", "/v1.52/secrets/create", req, &resp)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create secret %q", versionedName)
	}

	return resp.ID, nil
}

// ListConfigs returns all configs.
func (d *Swarm) ListConfigs(ctx context.Context) ([]Config, error) {

	var configs []namedResource
	err := d.client.SendObject(ctx, "GET", "/v1.52/configs", nil, &configs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list configs")
	}

	result := make([]Config, len(configs))
	for i, c := range configs {
		result[i] = Config{
			ID:   c.ID,
			Name: c.Spec.Name,
		}
	}

	return result, nil
}

// CreateConfig creates a versioned config and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Swarm) CreateConfig(ctx context.Context, name string, value []byte) (string, error) {

	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return "", err
	}

	versionedName := fmt.Sprintf("%s_v%d", name, findNextVersion(configsToItems(configs), name))
	req := dataCreate{
		Name: versionedName,
		Data: base64.StdEncoding.EncodeToString(value),
	}

	var resp IDResponse
	err = d.client.SendObject(ctx, "POST", "/v1.52/configs/create", req, &resp)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create config %q", versionedName)
	}

	return resp.ID, nil
}

// DeleteConfig deletes a config by ID.
func (d *Swarm) DeleteConfig(ctx context.Context, id string) error {

	err := d.client.SendObject(ctx, "DELETE", "/v1.52/configs/"+id, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete config %q", id)
	}

	return nil
}

type namedResource struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
}

type dataCreate struct {
	Name string `json:"Name"`
	Data string `json:"Data"`
}

// namedItem is used by version helpers.
type namedItem struct {
	ID   string
	Name string
}

func secretsToItems(secrets []Secret) []namedItem {
	items := make([]namedItem, len(secrets))
	for i, s := range secrets {
		items[i] = namedItem(s)
	}
	return items
}

func configsToItems(configs []Config) []namedItem {
	items := make([]namedItem, len(configs))
	for i, c := range configs {
		items[i] = namedItem(c)
	}
	return items
}

func findLatest(items []namedItem, baseName, resourceType string) (id, name string, err error) {
	var bestID, bestName string
	var bestVersion int

	prefix := baseName + "_v"
	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) {
			vStr := strings.TrimPrefix(item.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v > bestVersion {
				bestVersion = v
				bestID = item.ID
				bestName = item.Name
			}
		}
	}

	if bestID == "" {
		return "", "", errors.Errorf("%s %q not found (no %s_v* versions)", resourceType, baseName, baseName)
	}

	return bestID, bestName, nil
}

func findNextVersion(items []namedItem, baseName string) int {
	nextVersion := 1
	prefix := baseName + "_v"

	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) {
			vStr := strings.TrimPrefix(item.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v >= nextVersion {
				nextVersion = v + 1
			}
		}
	}

	return nextVersion
}
