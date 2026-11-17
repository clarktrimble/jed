package swarm_test

import (
	"context"
	"encoding/json"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/swarm"
	"github.com/kelseyhightower/envconfig"
)

var _ = Describe("Config", func() {
	BeforeEach(func() {
		for _, name := range []string{
			"SWARM_SOCKET", "SWARM_RESTART_DELAY", "SWARM_RESTART_MAX_ATTEMPTS", "SWARM_RESTART_WINDOW",
		} {
			value, present := os.LookupEnv(name)
			if present {
				DeferCleanup(os.Setenv, name, value)
			} else {
				DeferCleanup(os.Unsetenv, name)
			}
			Expect(os.Unsetenv(name)).To(Succeed())
		}
	})

	It("creates a Swarm", func() {
		sw := (&swarm.Config{Socket: "/tmp/docker.sock"}).New(nopLogger{})
		Expect(sw).NotTo(BeNil())
	})

	It("projects the restart defaults populated by envconfig", func() {
		cfg := &swarm.Config{}
		Expect(envconfig.Process("swarm", cfg)).To(Succeed())
		Expect(cfg.RestartDelay).To(Equal(5 * time.Second))
		Expect(cfg.RestartMaxAttempts).To(Equal(5))
		Expect(cfg.RestartWindow).To(Equal(2 * time.Minute))

		body, err := cfg.New(nopLogger{}).Spec(context.Background(), restartSpec(jed.RestartAny))
		Expect(err).NotTo(HaveOccurred())
		Expect(body.TaskTemplate.RestartPolicy).To(Equal(swarm.RestartPolicy{
			Condition: "any", Delay: 5000000000, MaxAttempts: 5, Window: 120000000000,
		}))
		Expect(wireRestartPolicy(body)).To(HaveKeyWithValue("Window", float64(120000000000)))
	})

	It("projects restart settings supplied through the environment", func() {
		Expect(os.Setenv("SWARM_RESTART_DELAY", "1500ms")).To(Succeed())
		Expect(os.Setenv("SWARM_RESTART_MAX_ATTEMPTS", "3")).To(Succeed())
		Expect(os.Setenv("SWARM_RESTART_WINDOW", "90s")).To(Succeed())
		cfg := &swarm.Config{}
		Expect(envconfig.Process("swarm", cfg)).To(Succeed())

		body, err := cfg.New(nopLogger{}).Spec(context.Background(), restartSpec(jed.RestartAny))
		Expect(err).NotTo(HaveOccurred())
		Expect(body.TaskTemplate.RestartPolicy).To(Equal(swarm.RestartPolicy{
			Condition: "any", Delay: 1500000000, MaxAttempts: 3, Window: 90000000000,
		}))
	})

	Describe("custom restart policy", func() {
		var cfg *swarm.Config
		BeforeEach(func() {
			cfg = &swarm.Config{
				Socket:             "/tmp/docker.sock",
				RestartDelay:       1500*time.Millisecond + 7*time.Nanosecond,
				RestartMaxAttempts: 3,
				RestartWindow:      90*time.Second + 11*time.Nanosecond,
			}
		})

		It("preserves exact nanosecond durations for any", func() {
			body, err := cfg.New(nopLogger{}).Spec(context.Background(), restartSpec(jed.RestartAny))
			Expect(err).NotTo(HaveOccurred())
			Expect(body.TaskTemplate.RestartPolicy).To(Equal(swarm.RestartPolicy{
				Condition: "any", Delay: 1500000007, MaxAttempts: 3, Window: 90000000011,
			}))
			Expect(wireRestartPolicy(body)).To(Equal(map[string]any{
				"Condition": "any", "Delay": float64(1500000007),
				"MaxAttempts": float64(3), "Window": float64(90000000011),
			}))
		})

		It("keeps one retry and no window for on-failure", func() {
			body, err := cfg.New(nopLogger{}).Spec(context.Background(), restartSpec(jed.RestartOnFailure))
			Expect(err).NotTo(HaveOccurred())
			Expect(body.TaskTemplate.RestartPolicy).To(Equal(swarm.RestartPolicy{
				Condition: "on-failure", Delay: 1500000007, MaxAttempts: 1,
			}))
			Expect(wireRestartPolicy(body)).NotTo(HaveKey("Window"))
		})

		It("omits timing and retry fields for none", func() {
			body, err := cfg.New(nopLogger{}).Spec(context.Background(), restartSpec(jed.RestartNone))
			Expect(err).NotTo(HaveOccurred())
			Expect(wireRestartPolicy(body)).To(Equal(map[string]any{"Condition": "none"}))
		})
	})

	It("does not replace explicit zero settings with defaults", func() {
		body, err := (&swarm.Config{Socket: "/tmp/docker.sock"}).New(nopLogger{}).
			Spec(context.Background(), restartSpec(jed.RestartAny))
		Expect(err).NotTo(HaveOccurred())
		Expect(body.TaskTemplate.RestartPolicy).To(Equal(swarm.RestartPolicy{Condition: "any"}))
		Expect(wireRestartPolicy(body)).To(Equal(map[string]any{"Condition": "any"}))
	})
})

// Specs without secrets/configs let policy projection run without Docker I/O.
func restartSpec(condition string) jed.Spec {
	return jed.Spec{
		Service: jed.Service{
			Name: "app", Image: "app:v1", Network: "svc-net", User: "1000", Restart: condition,
		},
		Intent: jed.Intent{Name: "app", Image: "app:v1", Replicas: 1},
	}
}

func wireRestartPolicy(body swarm.Spec) map[string]any {
	data, err := json.Marshal(body)
	Expect(err).NotTo(HaveOccurred())
	var wire struct {
		TaskTemplate struct{ RestartPolicy map[string]any }
	}
	Expect(json.Unmarshal(data, &wire)).To(Succeed())
	return wire.TaskTemplate.RestartPolicy
}
