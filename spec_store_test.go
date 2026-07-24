package jed_test

import (
	"context"

	"github.com/clarktrimble/jed"
)

type fakeStore struct {
	services map[string]jed.Service
	envs     map[string]jed.Env
	intents  map[string]jed.Intent
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		services: map[string]jed.Service{},
		envs:     map[string]jed.Env{},
		intents:  map[string]jed.Intent{},
	}
}

func (s *fakeStore) GetService(ctx context.Context, name, image string) (jed.Service, error) {
	svc, ok := s.services[name+"\x00"+image]
	if !ok {
		return jed.Service{}, jed.NotFoundError{Kind: "service", Name: name + "@" + image}
	}
	return svc, nil
}

func (s *fakeStore) SetService(ctx context.Context, svc jed.Service) error {
	s.services[svc.Name+"\x00"+svc.Image] = svc
	if _, ok := s.services[svc.Name]; ok {
		s.services[svc.Name] = svc
	}
	return nil
}

func (s *fakeStore) DelService(ctx context.Context, name, image string) error {
	delete(s.services, name+"\x00"+image)
	return nil
}

func (s *fakeStore) Services(ctx context.Context, name string) ([]jed.Service, error) {
	services := make([]jed.Service, 0, len(s.services))
	for _, svc := range s.services {
		if svc.Name == name {
			services = append(services, svc)
		}
	}
	return services, nil
}

func (s *fakeStore) AllServices(ctx context.Context) ([]jed.Service, error) {
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

func (s *fakeStore) GetIntent(ctx context.Context, name string) (jed.Intent, error) {
	intent, ok := s.intents[name]
	if !ok {
		return jed.Intent{}, jed.NotFoundError{Kind: "intent", Name: name}
	}
	return intent, nil
}

func (s *fakeStore) SetIntent(ctx context.Context, intent jed.Intent) error {
	s.intents[intent.Name] = intent
	return nil
}

func (s *fakeStore) DelIntent(ctx context.Context, name string) error {
	delete(s.intents, name)
	return nil
}

func (s *fakeStore) Intents(ctx context.Context) ([]jed.Intent, error) {
	intents := make([]jed.Intent, 0, len(s.intents))
	for _, intent := range s.intents {
		intents = append(intents, intent)
	}
	return intents, nil
}
