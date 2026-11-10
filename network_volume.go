package jed

import (
	"context"
	"fmt"
	"hash/fnv"
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

func networkVolumeMount(server, root, serviceName, target string) (NetworkMount, error) {
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
	stem := service + "_" + path.Base(target)
	if !networkVolumeName.MatchString(stem) {
		return NetworkMount{}, fmt.Errorf("invalid network volume name stem %q", stem)
	}
	if len(stem) > 25 {
		stem = stem[:25]
	}
	// This encoding and hash are the storage identity shared with the NAS provisioner.
	hash := fnv.New64a()
	hash.Write([]byte(serviceName + "\x00" + target))
	name := fmt.Sprintf("%s_%06x", stem, hash.Sum64()&0xffffff)
	return NetworkMount{
		Name: name, Target: target, Server: server,
		Export: path.Join(root, name),
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

func (j *Jed) resolveNetworkVolumes(ctx context.Context, spec *Spec, server, root string) error {
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
		mount, err := networkVolumeMount(server, root, spec.Service.Name, target)
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
