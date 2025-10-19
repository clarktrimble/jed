package jed_test

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"sync"

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

	for i := range services {
		// Todo: newServices without nil molehill
		if services[i].Labels == nil {
			services[i].Labels = make(map[string]string)
		}
		services[i].Labels["managed_by"] = "jed"
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

// FSServiceStore implements ServiceStore using an fs.FS.
// This is a read-only store that loads services from a filesystem.
type FSServiceStore struct {
	fs fs.FS
}

// NewFSServiceStore creates a new FSServiceStore from an fs.FS.
func NewFSServiceStore(filesystem fs.FS) *FSServiceStore {
	return &FSServiceStore{fs: filesystem}
}

// Get retrieves a service definition by name.
func (s *FSServiceStore) Get(ctx context.Context, serviceName string) (jed.Service, error) {
	services, err := s.List(ctx)
	if err != nil {
		return jed.Service{}, err
	}

	for _, svc := range services {
		if svc.Name == serviceName {
			return svc, nil
		}
	}

	return jed.Service{}, errors.Errorf("service %s not found", serviceName)
}

// Set is not supported for FSServiceStore (read-only).
func (s *FSServiceStore) Set(ctx context.Context, svc jed.Service) error {
	return errors.New("FSServiceStore is read-only")
}

// Del is not supported for FSServiceStore (read-only).
func (s *FSServiceStore) Del(ctx context.Context, serviceName string) error {
	return errors.New("FSServiceStore is read-only")
}

// List retrieves all service definitions from the filesystem.
func (s *FSServiceStore) List(ctx context.Context) ([]jed.Service, error) {
	return loadServices(s.fs)
}

// MemoryServiceStore implements ServiceStore using in-memory storage.
// Safe for concurrent use.
type MemoryServiceStore struct {
	mu   sync.RWMutex
	svcs map[string]jed.Service
}

// NewMemoryServiceStore creates a new empty MemoryServiceStore.
func NewMemoryServiceStore() *MemoryServiceStore {
	return &MemoryServiceStore{
		svcs: make(map[string]jed.Service),
	}
}

// Get retrieves a service definition by name.
func (s *MemoryServiceStore) Get(ctx context.Context, serviceName string) (jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.svcs[serviceName]
	if !ok {
		return jed.Service{}, errors.Errorf("service %s not found", serviceName)
	}
	return svc, nil
}

// Set persists a service definition.
func (s *MemoryServiceStore) Set(ctx context.Context, svc jed.Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.svcs[svc.Name] = svc
	return nil
}

// Del removes a service definition.
func (s *MemoryServiceStore) Del(ctx context.Context, serviceName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.svcs, serviceName)
	return nil
}

// List retrieves all service definitions.
func (s *MemoryServiceStore) List(ctx context.Context) ([]jed.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]jed.Service, 0, len(s.svcs))
	for _, svc := range s.svcs {
		services = append(services, svc)
	}
	return services, nil
}

// FSEnvStore implements EnvStore using an fs.FS.
// This is a read-only store that loads env vars from .env files.
type FSEnvStore struct {
	fs fs.FS
}

// NewFSEnvStore creates a new FSEnvStore from an fs.FS.
func NewFSEnvStore(filesystem fs.FS) *FSEnvStore {
	return &FSEnvStore{fs: filesystem}
}

// Get retrieves env vars for a service from a .env file.
func (s *FSEnvStore) Get(ctx context.Context, serviceName string) (map[string]string, error) {
	return loadEnv(s.fs, serviceName)
}

// Set is not supported for FSEnvStore (read-only).
func (s *FSEnvStore) Set(ctx context.Context, serviceName string, env map[string]string) error {
	return errors.New("FSEnvStore is read-only")
}

// Del is not supported for FSEnvStore (read-only).
func (s *FSEnvStore) Del(ctx context.Context, serviceName string) error {
	return errors.New("FSEnvStore is read-only")
}

// MemoryEnvStore implements EnvStore using in-memory storage.
// Safe for concurrent use.
type MemoryEnvStore struct {
	mu   sync.RWMutex
	envs map[string]map[string]string
}

// NewMemoryEnvStore creates a new empty MemoryEnvStore.
func NewMemoryEnvStore() *MemoryEnvStore {
	return &MemoryEnvStore{
		envs: make(map[string]map[string]string),
	}
}

// Get retrieves all env vars for a service.
func (s *MemoryEnvStore) Get(ctx context.Context, serviceName string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, ok := s.envs[serviceName]
	if !ok {
		return make(map[string]string), nil // Return empty map, not error
	}

	// Return a copy to prevent external modification
	return maps.Clone(env), nil
}

// Set persists all env vars for a service.
func (s *MemoryEnvStore) Set(ctx context.Context, serviceName string, env map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external modification
	s.envs[serviceName] = maps.Clone(env)
	return nil
}

// Del removes all stored env vars for a service.
func (s *MemoryEnvStore) Del(ctx context.Context, serviceName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.envs, serviceName)
	return nil
}

// loadMemoryStoreFromFS loads services from a filesystem into a MemoryServiceStore.
func loadMemoryStoreFromFS(fsPath string) *MemoryServiceStore {
	fsStore := NewFSServiceStore(os.DirFS(fsPath))
	services, err := fsStore.List(context.Background())
	Expect(err).NotTo(HaveOccurred())

	memStore := NewMemoryServiceStore()
	for _, svc := range services {
		err = memStore.Set(context.Background(), svc)
		Expect(err).NotTo(HaveOccurred())
	}
	return memStore
}
