package jed

import (
	"context"
	"fmt"
	"net"
	"path"
	"regexp"
	"strings"
)

// NetworkMount describes a convention-resolved NFS mount in a rendered Spec.
// It is not part of the persisted service definition.
type NetworkMount struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Server string `json:"server"`
	Export string `json:"export"`
}

var (
	networkVolumeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	networkHostLabel  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)
)

func networkVolumeMount(server, root, prefix, serviceName, target string) (NetworkMount, error) {
	if !validNetworkServer(server) {
		return NetworkMount{}, fmt.Errorf("NFS_HOST has invalid server %q", server)
	}
	root, err := cleanLocalPath("NFS_ROOT", root)
	if err != nil {
		return NetworkMount{}, err
	}
	if strings.ContainsRune(root, 0) {
		return NetworkMount{}, fmt.Errorf("NFS_ROOT cannot contain NUL")
	}
	target, err = cleanLocalPath("network volume", target)
	if err != nil {
		return NetworkMount{}, err
	}
	service, err := flatPathPart("service name", serviceName)
	if err != nil {
		return NetworkMount{}, err
	}
	if prefix == "" {
		prefix = "jed"
	}
	if !networkVolumeName.MatchString(prefix) {
		return NetworkMount{}, fmt.Errorf("invalid NETWORK_VOLUME_PREFIX %q", prefix)
	}
	flatTarget := strings.ReplaceAll(strings.TrimPrefix(target, "/"), "/", "_")
	name := prefix + "-" + service + "-" + flatTarget
	if !networkVolumeName.MatchString(name) {
		return NetworkMount{}, fmt.Errorf("invalid derived network volume name %q", name)
	}
	return NetworkMount{
		Name: name, Target: target, Server: server,
		Export: path.Join(root, service, "_"+flatTarget),
	}, nil
}

func validateConventionalVolumeTargets(service Service) error {
	if len(service.NetworkVolumes) == 0 {
		return nil
	}
	localTargets := make(map[string]bool, len(service.LocalVolumes))
	for _, rawTarget := range service.LocalVolumes {
		target, err := cleanLocalPath("local volume", rawTarget)
		if err != nil {
			return err
		}
		localTargets[target] = true
	}
	for _, rawTarget := range service.NetworkVolumes {
		target, err := cleanLocalPath("network volume", rawTarget)
		if err != nil {
			return err
		}
		if localTargets[target] {
			return fmt.Errorf("local and network volumes share container target %q", target)
		}
	}
	return nil
}

func (j *Jed) resolveNetworkVolumes(ctx context.Context, spec *Spec, server, root, prefix string) error {
	if len(spec.Service.NetworkVolumes) == 0 {
		return nil
	}
	explicitTargets := make(map[string]bool, len(spec.Service.Volumes))
	for _, target := range spec.Service.Volumes {
		explicitTargets[path.Clean(target)] = true
	}
	seenTargets := make(map[string]bool)
	names := make(map[string]string)
	for _, target := range spec.Service.NetworkVolumes {
		mount, err := networkVolumeMount(server, root, prefix, spec.Service.Name, target)
		if err != nil {
			return err
		}
		if seenTargets[mount.Target] {
			continue
		}
		seenTargets[mount.Target] = true
		if explicitTargets[mount.Target] {
			j.logger.Error(ctx, "network volume conflicts with explicit volume; using explicit volume",
				fmt.Errorf("network volume target %q already mounted", mount.Target), "service", spec.Service.Name, "target", mount.Target)
			continue
		}
		if previous, exists := names[mount.Name]; exists {
			return fmt.Errorf("targets %q and %q produce the same network volume name %q", previous, mount.Target, mount.Name)
		}
		names[mount.Name] = mount.Target
		spec.NetworkMounts = append(spec.NetworkMounts, mount)
	}
	spec.Service.NetworkVolumes = nil
	return nil
}

func validNetworkServer(server string) bool {
	if net.ParseIP(server) != nil {
		return true
	}
	if strings.Contains(server, ":") {
		return false
	}
	if len(server) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(server, "."), ".") {
		if len(label) > 63 || !networkHostLabel.MatchString(label) {
			return false
		}
	}
	return true
}
