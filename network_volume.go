package jed

import (
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

func networkVolumeMount(networkRoot, prefix, serviceName, target string) (NetworkMount, error) {
	server, root, ok := strings.Cut(networkRoot, ":/")
	if !ok || server == "" {
		return NetworkMount{}, fmt.Errorf("NETWORK_ROOT %q must be server:/absolute/export", networkRoot)
	}
	if strings.HasPrefix(server, "[") && strings.HasSuffix(server, "]") {
		server = strings.TrimSuffix(strings.TrimPrefix(server, "["), "]")
		if ip := net.ParseIP(server); ip == nil || !strings.Contains(server, ":") {
			return NetworkMount{}, fmt.Errorf("NETWORK_ROOT has invalid IPv6 server %q", server)
		}
	} else if !validNetworkServer(server) {
		return NetworkMount{}, fmt.Errorf("NETWORK_ROOT has invalid server %q", server)
	}
	root, err := cleanLocalPath("NETWORK_ROOT export", "/"+root)
	if err != nil {
		return NetworkMount{}, err
	}
	if strings.ContainsRune(root, 0) {
		return NetworkMount{}, fmt.Errorf("NETWORK_ROOT export cannot contain NUL")
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

func validNetworkServer(server string) bool {
	if strings.Contains(server, ":") { // IPv6 must be bracketed in NETWORK_ROOT.
		return false
	}
	if net.ParseIP(server) != nil {
		return true
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
