package jed

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Spec is rendered, runtime-neutral intended service state.
type Spec struct {
	// Service is the rendered service definition ready for a runtime adapter.
	Service Service `json:"service"`
	// Env is the service environment used to render the spec.
	Env Env `json:"env"`
}

// Render renders all string values in service using vars and env.Vars as template variables.
// env.Vars win over vars on key collisions. Render vars are not added to Env.
func Render(service Service, env Env, vars map[string]string) (Spec, error) {
	tplVars := make(map[string]string, len(vars)+len(env.Vars))
	maps.Copy(tplVars, vars)
	maps.Copy(tplVars, env.Vars)

	renderedService := cloneService(service)
	if err := expandStrings(reflect.ValueOf(&renderedService).Elem(), tplVars); err != nil {
		return Spec{}, err
	}

	return Spec{
		Service: renderedService,
		Env:     cloneEnv(env),
	}, nil
}

func cloneService(service Service) Service {
	clone := service
	clone.Command = slices.Clone(service.Command)
	clone.Ports = maps.Clone(service.Ports)
	clone.Labels = maps.Clone(service.Labels)
	clone.Volumes = maps.Clone(service.Volumes)
	clone.Secrets = slices.Clone(service.Secrets)
	clone.Configs = maps.Clone(service.Configs)
	clone.Hosts = slices.Clone(service.Hosts)
	clone.About.Links = slices.Clone(service.About.Links)
	clone.About.Notes = slices.Clone(service.About.Notes)

	if service.Restart.MaxAttempts != nil {
		attempts := *service.Restart.MaxAttempts
		clone.Restart.MaxAttempts = &attempts
	}
	if service.Traefik != nil {
		traefik := *service.Traefik
		clone.Traefik = &traefik
	}

	return clone
}

func cloneEnv(env Env) Env {
	return Env{
		Name: env.Name,
		Vars: maps.Clone(env.Vars),
	}
}

// expandStrings walks Go values directly rather than marshal/replace/unmarshal.
// Rendering is a Service-domain operation, not a YAML/JSON text transform: walking
// values avoids coupling render behavior to serialization tags, omitempty behavior,
// timestamp formatting, or nil-vs-empty collection round trips.
func expandStrings(v reflect.Value, vars map[string]string) error {
	if !v.IsValid() {
		return nil
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return expandStrings(v.Elem(), vars)
	case reflect.Struct:
		for i := range v.NumField() {
			field := v.Field(i)
			if !field.CanSet() {
				continue
			}
			if err := expandStrings(field, vars); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := range v.Len() {
			if err := expandStrings(v.Index(i), vars); err != nil {
				return err
			}
		}
	case reflect.Map:
		expanded := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, key := range v.MapKeys() {
			expandedKey, err := expandMapKey(key, vars)
			if err != nil {
				return err
			}

			expandedVal := reflect.New(v.Type().Elem()).Elem()
			expandedVal.Set(v.MapIndex(key))
			if err := expandStrings(expandedVal, vars); err != nil {
				return err
			}

			expanded.SetMapIndex(expandedKey, expandedVal)
		}
		v.Set(expanded)
	case reflect.String:
		expanded, err := expandString(v.String(), vars)
		if err != nil {
			return err
		}
		v.SetString(expanded)
	}

	return nil
}

func expandMapKey(key reflect.Value, vars map[string]string) (reflect.Value, error) {
	if key.Kind() != reflect.String {
		return key, nil
	}

	expanded, err := expandString(key.String(), vars)
	if err != nil {
		return reflect.Value{}, err
	}

	expandedKey := reflect.New(key.Type()).Elem()
	expandedKey.SetString(expanded)
	return expandedKey, nil
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
