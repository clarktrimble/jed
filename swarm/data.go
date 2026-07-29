package swarm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ErrNotFound marks swarm resources that could not be found.
var ErrNotFound = errors.New("not found")

// SecretResource represents a Docker Swarm secret resource.
type SecretResource struct {
	// ID is the Docker secret ID.
	ID string

	// Name is the Docker secret name. Secrets created by CreateSecret are named {base}_v{N}.
	Name string
}

// ConfigResource represents a Docker Swarm config resource.
type ConfigResource struct {
	// ID is the Docker config ID.
	ID string

	// Name is the Docker config name. Configs created by CreateConfig are named {base}_v{N}.
	Name string

	// Data is the Docker config data.
	Data []byte
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

// GetLatestConfig returns the latest config resource by base name.
// Looks for configs matching {name}_v{N} and returns the highest version.
func (d *Swarm) GetLatestConfig(ctx context.Context, name string) (ConfigResource, error) {
	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return ConfigResource{}, err
	}

	latest, ok := findLatestConfig(configs, name)
	if !ok {
		return ConfigResource{}, notFoundError("config", name)
	}

	return latest, nil
}

// ListSecrets returns all secrets.
func (d *Swarm) ListSecrets(ctx context.Context) ([]SecretResource, error) {

	var secrets []namedResource
	err := d.client.SendObject(ctx, "GET", "/v1.52/secrets", nil, &secrets)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list secrets")
	}

	result := make([]SecretResource, len(secrets))
	for i, s := range secrets {
		result[i] = SecretResource{
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
func (d *Swarm) ListConfigs(ctx context.Context) ([]ConfigResource, error) {

	var configs []namedResource
	err := d.client.SendObject(ctx, "GET", "/v1.52/configs", nil, &configs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list configs")
	}

	result := make([]ConfigResource, len(configs))
	for i, c := range configs {
		result[i] = ConfigResource{
			ID:   c.ID,
			Name: c.Spec.Name,
			Data: c.Spec.Data,
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

	encodedData := base64.StdEncoding.EncodeToString(value)
	latest, ok := findLatestConfig(configs, name)
	if ok && bytes.Equal(latest.Data, value) {
		d.logger.Info(ctx, "skipping creation of identical config", "name", name, "version", latest.Name, "id", latest.ID)
		return latest.ID, nil
	}

	versionedName := fmt.Sprintf("%s_v%d", name, findNextVersion(configsToItems(configs), name))
	req := dataCreate{
		Name: versionedName,
		Data: encodedData,
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
		Data []byte `json:"Data"`
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

func secretsToItems(secrets []SecretResource) []namedItem {
	items := make([]namedItem, len(secrets))
	for i, s := range secrets {
		items[i] = namedItem(s)
	}
	return items
}

func configsToItems(configs []ConfigResource) []namedItem {
	items := make([]namedItem, len(configs))
	for i, c := range configs {
		items[i] = namedItem{ID: c.ID, Name: c.Name}
	}
	return items
}

func findLatest(items []namedItem, baseName, resourceType string) (id, name string, err error) {
	latest, ok := findLatestItem(items, baseName)
	if !ok {
		return "", "", notFoundError(resourceType, baseName)
	}

	return latest.ID, latest.Name, nil
}

func findLatestConfig(configs []ConfigResource, baseName string) (ConfigResource, bool) {
	items := configsToItems(configs)
	latest, ok := findLatestItem(items, baseName)
	if !ok {
		return ConfigResource{}, false
	}

	for _, c := range configs {
		if c.ID == latest.ID {
			return c, true
		}
	}

	return ConfigResource{}, false
}

func findLatestItem(items []namedItem, baseName string) (namedItem, bool) {
	var best namedItem
	var bestVersion int

	prefix := baseName + "_v"
	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) {
			vStr := strings.TrimPrefix(item.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v > bestVersion {
				bestVersion = v
				best = item
			}
		}
	}

	return best, best.ID != ""
}

func notFoundError(resourceType, baseName string) error {
	return errors.Wrapf(ErrNotFound, "%s %q not found (no %s_v* versions)", resourceType, baseName, baseName)
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
