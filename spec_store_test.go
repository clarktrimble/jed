package jed_test

import (
	"context"

	"github.com/clarktrimble/jed"
)

type fakeStore struct {
	services map[string]jed.Service
	envs     map[string]jed.Env
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		services: map[string]jed.Service{},
		envs:     map[string]jed.Env{},
	}
}

func (s *fakeStore) GetService(ctx context.Context, name string) (jed.Service, error) {
	svc, ok := s.services[name]
	if !ok {
		return jed.Service{}, jed.NotFoundError{Kind: "service", Name: name}
	}
	return svc, nil
}

func (s *fakeStore) SetService(ctx context.Context, svc jed.Service) error {
	s.services[svc.Name] = svc
	return nil
}

func (s *fakeStore) DelService(ctx context.Context, name string) error {
	delete(s.services, name)
	return nil
}

func (s *fakeStore) Services(ctx context.Context) ([]jed.Service, error) {
	services := make([]jed.Service, 0, len(s.services))
	for _, svc := range s.services {
		services = append(services, svc)
	}
	return services, nil
}

func (s *fakeStore) GetEnv(ctx context.Context, name string) (jed.Env, error) {
	env, ok := s.envs[name]
	if !ok {
		return jed.Env{Name: name, Vars: map[string]string{}}, nil
	}
	return env, nil
}

func (s *fakeStore) SetEnv(ctx context.Context, env jed.Env) error {
	s.envs[env.Name] = env
	return nil
}

func (s *fakeStore) DelEnv(ctx context.Context, name string) error {
	delete(s.envs, name)
	return nil
}

func (s *fakeStore) Envs(ctx context.Context) ([]jed.Env, error) {
	envs := make([]jed.Env, 0, len(s.envs))
	for _, env := range s.envs {
		envs = append(envs, env)
	}
	return envs, nil
}
