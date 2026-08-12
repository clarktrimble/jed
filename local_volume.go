package jed

import (
	"fmt"
	"path"
	"strings"
)

func localVolumeMount(localRoot, serviceName, target string) (host string, cleanedTarget string, err error) {
	cleanedRoot, err := cleanLocalPath("LOCAL_ROOT", localRoot)
	if err != nil {
		return "", "", err
	}
	cleanedTarget, err = cleanLocalPath("local volume", target)
	if err != nil {
		return "", "", err
	}
	servicePart, err := flatPathPart("service name", serviceName)
	if err != nil {
		return "", "", err
	}
	targetPart := strings.TrimPrefix(cleanedTarget, "/")
	targetPart = strings.ReplaceAll(targetPart, "/", "_")

	host = path.Join(cleanedRoot, servicePart, "_"+targetPart)
	if !strings.HasPrefix(host, cleanedRoot+"/") {
		return "", "", fmt.Errorf("local volume host path %q escapes LOCAL_ROOT %q", host, cleanedRoot)
	}

	return host, cleanedTarget, nil
}

func cleanLocalPath(kind, value string) (string, error) {
	if !path.IsAbs(value) || path.Clean(value) == "/" {
		return "", fmt.Errorf("%s %q must be an absolute path below /", kind, value)
	}
	if hasDotPathSegment(value) {
		return "", fmt.Errorf("%s %q cannot contain . or .. path segments", kind, value)
	}
	return path.Clean(value), nil
}

func flatPathPart(kind, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s is required", kind)
	}
	if hasDotPathSegment(value) {
		return "", fmt.Errorf("%s %q cannot contain . or .. path segments", kind, value)
	}
	part := strings.Trim(path.Clean("/"+value), "/")
	if part == "" {
		return "", fmt.Errorf("%s %q must contain a path component", kind, value)
	}
	part = strings.ReplaceAll(part, "/", "_")
	return part, nil
}

func hasDotPathSegment(value string) bool {
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." {
			return true
		}
	}
	return false
}
