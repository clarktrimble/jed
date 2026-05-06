package store_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/store"
)

func TestStoreContract(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store Contract Suite")
}

// Simple in-memory store implementation for testing the contract itself.
type mockStore struct {
	services map[string]jed.Service
	envs     map[string]jed.Env
}

func newMockStore() *mockStore {
	return &mockStore{
		services: make(map[string]jed.Service),
		envs:     make(map[string]jed.Env),
	}
}

func (m *mockStore) GetService(ctx context.Context, name string) (jed.Service, error) {
	svc, ok := m.services[name]
	if !ok {
		return jed.Service{}, jed.NotFoundError{Kind: "service", Name: name}
	}
	return svc, nil
}

func (m *mockStore) SetService(ctx context.Context, svc jed.Service) error {
	m.services[svc.Name] = svc
	return nil
}

func (m *mockStore) DelService(ctx context.Context, name string) error {
	delete(m.services, name)
	return nil
}

func (m *mockStore) Services(ctx context.Context) ([]jed.Service, error) {
	result := make([]jed.Service, 0, len(m.services))
	for _, svc := range m.services {
		result = append(result, svc)
	}
	return result, nil
}

func (m *mockStore) GetEnv(ctx context.Context, name string) (jed.Env, error) {
	env, ok := m.envs[name]
	if !ok {
		return jed.Env{Name: name, Vars: make(map[string]string)}, nil
	}
	return env, nil
}

func (m *mockStore) SetEnv(ctx context.Context, env jed.Env) error {
	m.envs[env.Name] = env
	return nil
}

func (m *mockStore) DelEnv(ctx context.Context, name string) error {
	delete(m.envs, name)
	return nil
}

func (m *mockStore) Envs(ctx context.Context) ([]jed.Env, error) {
	result := make([]jed.Env, 0, len(m.envs))
	for _, env := range m.envs {
		result = append(result, env)
	}
	return result, nil
}

var _ = Describe("Contract Tests", func() {
	store.RunStoreContractTests(
		"Mock",
		func(ctx context.Context) (jed.Store, error) {
			return newMockStore(), nil
		},
		func() {
			// No cleanup needed
		},
	)
})
