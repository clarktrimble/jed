package jed

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Spec is rendered, runtime-neutral intended service state.
type Spec struct {
	Service Service
	Env     Env
}

// Render renders service command args, labels, and about link URLs using vars and env.Vars as template variables.
// env.Vars win over vars on key collisions. Render vars are not added to Env.
func Render(service Service, env Env, vars map[string]string) (Spec, error) {
	tplVars := make(map[string]string, len(vars)+len(env.Vars))
	maps.Copy(tplVars, vars)
	maps.Copy(tplVars, env.Vars)

	renderedService := cloneService(service)

	if len(service.Command) > 0 {
		command, err := expandVars(service.Command, tplVars)
		if err != nil {
			return Spec{}, err
		}
		renderedService.Command = command
	}

	if len(service.Labels) > 0 {
		labels, err := expandMapVars(service.Labels, tplVars)
		if err != nil {
			return Spec{}, err
		}
		renderedService.Labels = labels
	}

	if len(service.About.Links) > 0 {
		links, err := expandLinkVars(service.About.Links, tplVars)
		if err != nil {
			return Spec{}, err
		}
		renderedService.About.Links = links
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

func expandLinkVars(links []Link, vars map[string]string) ([]Link, error) {
	result := make([]Link, len(links))
	for i, link := range links {
		expanded, err := expandString(link.Url, vars)
		if err != nil {
			return nil, err
		}
		result[i] = link
		result[i].Url = expanded
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
