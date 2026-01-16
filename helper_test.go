package jed_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/clarktrimble/jed"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/gomega"
)

const (
	configFile string = "services.yaml"
	envSuffix  string = "env"
)

// loadServices loads services from a filesystem.
func loadServices(cfs fs.FS) (services []jed.Service, err error) {

	data, err := fs.ReadFile(cfs, configFile)
	if err != nil {
		err = errors.Wrap(err, "failed to read containers.yaml")
		return
	}

	err = yaml.Unmarshal(data, &services)
	if err != nil {
		err = errors.Wrapf(err, "failed to decode services config")
		return
	}

	return
}

// loadEnv loads environment variables from a .env file.
func loadEnv(cfs fs.FS, name string) (env map[string]string, err error) {

	file := fmt.Sprintf("%s.%s", name, envSuffix)

	env = map[string]string{}
	envData, err := fs.ReadFile(cfs, file)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
		return
	}
	if err != nil {
		err = errors.Wrapf(err, "cannot read %s", file)
		return
	}

	env, err = godotenv.Unmarshal(string(envData))
	err = errors.Wrapf(err, "cannot unmarshal %s", file)
	return
}

// newMockStore creates a new in-memory mock store for testing.
// This is the preferred way to create stores for unit tests.
func newMockStore() *StoreMock {
	// Internal storage
	services := make(map[string]jed.Service)
	envs := make(map[string]jed.Env)

	return &StoreMock{
		GetServiceFunc: func(ctx context.Context, name string) (jed.Service, error) {
			svc, ok := services[name]
			if !ok {
				return jed.Service{}, errors.Errorf("service not found: %s", name)
			}
			return svc, nil
		},
		SetServiceFunc: func(ctx context.Context, svc jed.Service) error {
			services[svc.Name] = svc
			return nil
		},
		DelServiceFunc: func(ctx context.Context, name string) error {
			delete(services, name)
			return nil
		},
		ServicesFunc: func(ctx context.Context) ([]jed.Service, error) {
			result := make([]jed.Service, 0, len(services))
			for _, svc := range services {
				result = append(result, svc)
			}
			return result, nil
		},
		GetEnvFunc: func(ctx context.Context, name string) (jed.Env, error) {
			env, ok := envs[name]
			if !ok {
				// Return empty env when not found (not an error condition)
				return jed.Env{Name: name, Vars: make(map[string]string)}, nil
			}
			return env, nil
		},
		SetEnvFunc: func(ctx context.Context, env jed.Env) error {
			envs[env.Name] = env
			return nil
		},
		DelEnvFunc: func(ctx context.Context, name string) error {
			delete(envs, name)
			return nil
		},
		EnvsFunc: func(ctx context.Context) ([]jed.Env, error) {
			result := make([]jed.Env, 0, len(envs))
			for _, env := range envs {
				result = append(result, env)
			}
			return result, nil
		},
	}
}

// loadMockStoreFromFS loads services and envs from filesystem into a mock store.
func loadMockStoreFromFS(fsPath string) *StoreMock {
	ctx := context.Background()
	filesystem := os.DirFS(fsPath)

	store := newMockStore()

	// Load services from filesystem
	services, err := loadServices(filesystem)
	Expect(err).NotTo(HaveOccurred())

	// Populate store with services and envs
	for _, svc := range services {
		err = store.SetServiceFunc(ctx, svc)
		Expect(err).NotTo(HaveOccurred())

		// Load and set env vars if they exist
		envVars, err := loadEnv(filesystem, svc.Name)
		Expect(err).NotTo(HaveOccurred())
		if len(envVars) > 0 {
			err = store.SetEnvFunc(ctx, jed.Env{Name: svc.Name, Vars: envVars})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	return store
}

// mockResponse marshals obj to JSON and unmarshals into rcv.
func mockResponse(obj, rcv any) {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	err = json.Unmarshal(data, rcv)
	Expect(err).NotTo(HaveOccurred())
}

// findCall searches for a call matching method and path substring.
func findCall(calls []struct {
	Ctx    context.Context
	Method string
	Path   string
	Snd    any
	Rcv    any
}, method, pathSubstring string) *struct {
	Ctx    context.Context
	Method string
	Path   string
	Snd    any
	Rcv    any
} {
	for i := range calls {
		if calls[i].Method == method && strings.Contains(calls[i].Path, pathSubstring) {
			return &calls[i]
		}
	}
	return nil
}

// mockContainers creates a mock response for Containers() call.
func mockContainers(containers []jed.Container) func(context.Context, string, string, any, any) error {
	return func(ctx context.Context, method, path string, snd, rcv any) error {
		if method == "GET" && strings.Contains(path, "/containers/json") {
			mockResponse(containers, rcv)
		}
		// Mock checkImage - always succeed
		if method == "GET" && strings.Contains(path, "/images/") {
			return nil
		}
		return nil
	}
}

// testContainerList returns a standard set of test containers for mocking.
func testContainerList() []jed.Container {
	return []jed.Container{
		{
			Id:     "abc123",
			Names:  []string{"/postgres-x7y9z2n"},
			Image:  "postgres:14",
			State:  "running",
			Status: "Up 2 hours",
			Labels: map[string]string{"managed_by": "jed"},
		},
		{
			Id:     "def456",
			Names:  []string{"/redis-k3m5p1q"},
			Image:  "redis:7",
			State:  "running",
			Status: "Up 1 hour",
			Labels: map[string]string{"managed_by": "jed"},
		},
		{
			Id:     "ghi789",
			Names:  []string{"/nginx-w8x2y4z"},
			Image:  "nginx:latest",
			State:  "exited",
			Status: "Exited (0) 5 minutes ago",
			Labels: map[string]string{"managed_by": "jed"},
		},
	}
}

// mockCreate creates a mock response for Deploy() create call.
func mockCreate(containerID string) func(context.Context, string, string, any, any) error {
	return func(ctx context.Context, method, path string, snd, rcv any) error {
		if method == "POST" && strings.Contains(path, "/containers/create") && rcv != nil {
			mockResponse(map[string]string{"Id": containerID}, rcv)
		}
		// Mock checkImage - always succeed
		if method == "GET" && strings.Contains(path, "/images/") {
			return nil
		}
		return nil
	}
}
