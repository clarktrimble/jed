package jed

import (
	"strings"

	"github.com/pkg/errors"
)

// Intent is the desired active image and replica count for a logical service.
// A missing Intent means the service is disabled.
type Intent struct {
	// Name is the logical service name.
	Name string `json:"name"`
	// Image is the selected service image.
	Image string `json:"image"`
	// Replicas is the number of service instances to run.
	Replicas int `json:"replicas,omitempty"`
}

// Validate checks that the intent has valid desired state.
func (intent Intent) Validate() error {
	var issues []string

	if intent.Name == "" {
		issues = append(issues, "name is required")
	}
	if intent.Image == "" {
		issues = append(issues, "image is required")
	}
	if intent.Replicas < 0 {
		issues = append(issues, "replicas cannot be negative")
	}

	if len(issues) > 0 {
		return errors.Errorf("intent %s invalid: %s", intent.Name, strings.Join(issues, ", "))
	}

	return nil
}
