package jed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

var _ = Describe("Jed", func() {
	Describe("New", func() {
		It("loads render vars from the named env", func() {
			ctx := context.Background()
			store := newFakeStore()
			store.services["app"] = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"--host={{VHOST}}"},
			}
			store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{}}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}

			j, err := jed.New(ctx, store, "_global")
			Expect(err).NotTo(HaveOccurred())

			spec, err := j.Spec(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"--host=app.example.com"}))
		})

		It("rejects a nil store", func() {
			_, err := jed.New(context.Background(), nil, "_global")
			Expect(err).To(MatchError("jed has nil store"))
		})
	})

	Describe("Spec", func() {
		var (
			ctx   context.Context
			store *fakeStore
			j     *jed.Jed
			spec  jed.Spec
			err   error
		)

		BeforeEach(func() {
			ctx = context.Background()
			store = newFakeStore()
			store.services["app"] = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"serve", "--host={{VHOST}}", "--port={{PORT}}"},
			}
			store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{"PORT": "8080"}}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}

			j, err = jed.New(ctx, store, "_global")
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			spec, err = j.Spec(ctx, "app")
		})

		It("loads service and env and returns a rendered spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Name).To(Equal("app"))
			Expect(spec.Service.Command).To(Equal([]string{"serve", "--host=app.example.com", "--port=8080"}))
			Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
			Expect(spec.Env.Vars).NotTo(HaveKey("VHOST"))
		})

		When("the service is invalid", func() {
			BeforeEach(func() {
				store.services["app"] = jed.Service{Name: "app", Image: "local/app:v1"}
			})

			It("fails validation", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to validate service"))
			})
		})
	})
})

var _ = Describe("Spec", func() {
	Describe("NewSpec", func() {
		var (
			svc  jed.Service
			env  jed.Env
			vars map[string]string
		)

		BeforeEach(func() {
			svc = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"serve", "--host={{VHOST}}", "--port={{PORT}}"},
				Labels: map[string]string{
					"host": "{{VHOST}}",
					"mode": "prod",
				},
			}
			env = jed.Env{Name: "app", Vars: map[string]string{"PORT": "8080"}}
			vars = map[string]string{"VHOST": "app.example.com"}
		})

		It("expands command vars", func() {
			spec, err := jed.NewSpec(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"serve", "--host=app.example.com", "--port=8080"}))
		})

		It("expands label vars", func() {
			spec, err := jed.NewSpec(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "app.example.com"))
			Expect(spec.Service.Labels).To(HaveKeyWithValue("mode", "prod"))
		})

		It("lets service env win over injected vars", func() {
			env.Vars["VHOST"] = "override.example.com"

			spec, err := jed.NewSpec(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(ContainElement("--host=override.example.com"))
			Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "override.example.com"))
		})

		It("does not add injected vars to Spec.Env", func() {
			spec, err := jed.NewSpec(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
			Expect(spec.Env.Vars).NotTo(HaveKey("VHOST"))
		})

		It("fails on missing vars", func() {
			delete(vars, "VHOST")

			_, err := jed.NewSpec(svc, env, vars)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("VHOST"))
		})

		It("expands in a single pass", func() {
			svc.Command = []string{"run", "{{TRICKY}}"}
			env.Vars["TRICKY"] = "has{{NESTED}}braces"

			spec, err := jed.NewSpec(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"run", "has{{NESTED}}braces"}))
		})

		It("leaves unmatched opening braces unchanged", func() {
			svc.Command = []string{"run", "before {{VHOST"}

			spec, err := jed.NewSpec(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"run", "before {{VHOST"}))
		})
	})
})
