package store

import (
	"context"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

// RunStoreContractTests runs a comprehensive test suite against any jed.Store implementation.
// This ensures all Store implementations behave consistently with the interface contract.
//
// Parameters:
//   - name: A descriptive name for the store implementation (e.g., "Bbolt", "Memo")
//   - newStore: A function that creates a fresh store instance for each test
//   - cleanup: A function called after each test to clean up resources
func RunStoreContractTests(
	name string,
	newStore func(ctx context.Context) (jed.Store, error),
	cleanup func(),
) {
	var _ = Describe(name+" Store Contract", func() {
		var (
			store   jed.Store
			ctx     context.Context
			err     error
			service jed.Service
			env     jed.Env
			intent  jed.Intent
		)

		BeforeEach(func() {
			ctx = context.Background()
			store, err = newStore(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		Describe("SetService and GetService", func() {
			var retrievedService jed.Service

			BeforeEach(func() {
				service = jed.Service{
					Name:    "postgres",
					Image:   "postgres:14",
					Network: "app-net",
					Restart: jed.RestartAny,
					Ports: map[string]string{
						"5432/tcp": "5432",
					},
					Labels: map[string]string{
						"app": "myapp",
					},
				}
			})

			JustBeforeEach(func() {
				err = store.SetService(ctx, service)
			})

			When("service is set successfully", func() {
				It("should store and retrieve the service", func() {
					Expect(err).NotTo(HaveOccurred())

					retrievedService, err = store.GetService(ctx, "postgres", "postgres:14")
					Expect(err).NotTo(HaveOccurred())
					Expect(retrievedService.Name).To(Equal("postgres"))
					Expect(retrievedService.Image).To(Equal("postgres:14"))
					Expect(retrievedService.Ports["5432/tcp"]).To(Equal("5432"))
				})
			})

			When("getting a non-existent service", func() {
				It("should return NotFoundError", func() {
					_, err = store.GetService(ctx, "nonexistent", "postgres:14")
					Expect(err).To(HaveOccurred())

					var notFound jed.NotFoundError
					Expect(errors.As(err, &notFound)).To(BeTrue())
					Expect(notFound.Kind).To(Equal("service"))
					Expect(notFound.Name).To(Equal("nonexistent@postgres:14"))
				})
			})
		})

		Describe("Services", func() {
			var services []jed.Service

			JustBeforeEach(func() {
				services, err = store.Services(ctx, "postgres")
			})

			When("no services exist", func() {
				It("should return empty slice", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(services).To(BeEmpty())
				})
			})

			When("multiple services exist for the same name", func() {
				BeforeEach(func() {
					err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:14"})
					Expect(err).NotTo(HaveOccurred())
					err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:15"})
					Expect(err).NotTo(HaveOccurred())
					err = store.SetService(ctx, jed.Service{Name: "redis", Image: "redis:7"})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return services with the requested name", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(services).To(HaveLen(2))
					images := []string{services[0].Image, services[1].Image}
					Expect(images).To(ContainElements("postgres:14", "postgres:15"))
				})
			})
		})

		Describe("AllServices", func() {
			var services []jed.Service

			JustBeforeEach(func() {
				services, err = store.AllServices(ctx)
			})

			When("no services exist", func() {
				It("should return empty slice", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(services).To(BeEmpty())
				})
			})

			When("multiple services exist", func() {
				BeforeEach(func() {
					err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:14"})
					Expect(err).NotTo(HaveOccurred())
					err = store.SetService(ctx, jed.Service{Name: "redis", Image: "redis:7"})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return all services", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(services).To(HaveLen(2))
					names := []string{services[0].Name, services[1].Name}
					Expect(names).To(ContainElements("postgres", "redis"))
				})
			})
		})

		Describe("DelService", func() {

			BeforeEach(func() {
				service = jed.Service{Name: "postgres", Image: "postgres:14"}
				err = store.SetService(ctx, service)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				err = store.DelService(ctx, "postgres", "postgres:14")
			})

			When("deleting an existing service", func() {
				It("should remove the service", func() {
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetService(ctx, "postgres", "postgres:14")
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("SetEnv and GetEnv", func() {
			var retrievedEnv jed.Env

			BeforeEach(func() {
				env = jed.Env{
					Name: "postgres",
					Vars: map[string]string{
						"POSTGRES_PASSWORD": "secret123",
						"POSTGRES_USER":     "admin",
					},
				}
			})

			JustBeforeEach(func() {
				err = store.SetEnv(ctx, env)
			})

			When("env is set successfully", func() {
				It("should store and retrieve the env", func() {
					Expect(err).NotTo(HaveOccurred())

					retrievedEnv, err = store.GetEnv(ctx, "postgres")
					Expect(err).NotTo(HaveOccurred())
					Expect(retrievedEnv.Name).To(Equal("postgres"))
					Expect(retrievedEnv.Vars["POSTGRES_PASSWORD"]).To(Equal("secret123"))
					Expect(retrievedEnv.Vars["POSTGRES_USER"]).To(Equal("admin"))
				})
			})

			When("getting a non-existent env", func() {
				It("should return empty env", func() {
					retrievedEnv, err := store.GetEnv(ctx, "nonexistent")
					Expect(err).NotTo(HaveOccurred())
					Expect(retrievedEnv.Name).To(Equal("nonexistent"))
					Expect(retrievedEnv.Vars).To(BeEmpty())
				})
			})
		})

		Describe("Envs", func() {
			var envs []jed.Env

			JustBeforeEach(func() {
				envs, err = store.Envs(ctx)
			})

			When("no envs exist", func() {
				It("should return empty slice", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(envs).To(BeEmpty())
				})
			})

			When("multiple envs exist", func() {
				BeforeEach(func() {
					err = store.SetEnv(ctx, jed.Env{
						Name: "postgres",
						Vars: map[string]string{"POSTGRES_PASSWORD": "secret"},
					})
					Expect(err).NotTo(HaveOccurred())
					err = store.SetEnv(ctx, jed.Env{
						Name: "redis",
						Vars: map[string]string{"REDIS_PASSWORD": "secret"},
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return all envs", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(envs).To(HaveLen(2))
					names := []string{envs[0].Name, envs[1].Name}
					Expect(names).To(ContainElements("postgres", "redis"))
				})
			})
		})

		Describe("DelEnv", func() {

			BeforeEach(func() {
				env = jed.Env{
					Name: "postgres",
					Vars: map[string]string{"POSTGRES_PASSWORD": "secret"},
				}
				err = store.SetEnv(ctx, env)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				err = store.DelEnv(ctx, "postgres")
			})

			When("deleting an existing env", func() {
				It("should remove the env", func() {
					Expect(err).NotTo(HaveOccurred())

					retrievedEnv, err := store.GetEnv(ctx, "postgres")
					Expect(err).NotTo(HaveOccurred())
					Expect(retrievedEnv.Vars).To(BeEmpty())
				})
			})
		})

		Describe("SetIntent and GetIntent", func() {
			var retrievedIntent jed.Intent

			BeforeEach(func() {
				intent = jed.Intent{Name: "postgres", Image: "postgres:14", Replicas: 1}
			})

			JustBeforeEach(func() {
				err = store.SetIntent(ctx, intent)
			})

			When("intent is set successfully", func() {
				It("should store and retrieve the intent", func() {
					Expect(err).NotTo(HaveOccurred())

					retrievedIntent, err = store.GetIntent(ctx, "postgres")
					Expect(err).NotTo(HaveOccurred())
					Expect(retrievedIntent.Name).To(Equal("postgres"))
					Expect(retrievedIntent.Image).To(Equal("postgres:14"))
					Expect(retrievedIntent.Replicas).To(Equal(1))
				})
			})

			When("getting a non-existent intent", func() {
				It("should return NotFoundError", func() {
					_, err = store.GetIntent(ctx, "nonexistent")
					Expect(err).To(HaveOccurred())

					var notFound jed.NotFoundError
					Expect(errors.As(err, &notFound)).To(BeTrue())
					Expect(notFound.Kind).To(Equal("intent"))
					Expect(notFound.Name).To(Equal("nonexistent"))
				})
			})
		})

		Describe("Intents", func() {
			var intents []jed.Intent

			JustBeforeEach(func() {
				intents, err = store.Intents(ctx)
			})

			When("no intents exist", func() {
				It("should return empty slice", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(intents).To(BeEmpty())
				})
			})

			When("multiple intents exist", func() {
				BeforeEach(func() {
					err = store.SetIntent(ctx, jed.Intent{Name: "postgres", Image: "postgres:14", Replicas: 1})
					Expect(err).NotTo(HaveOccurred())
					err = store.SetIntent(ctx, jed.Intent{Name: "redis", Image: "redis:7"})
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return all intents", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(intents).To(HaveLen(2))
					names := []string{intents[0].Name, intents[1].Name}
					Expect(names).To(ContainElements("postgres", "redis"))
				})
			})
		})

		Describe("DelIntent", func() {

			BeforeEach(func() {
				intent = jed.Intent{Name: "postgres", Image: "postgres:14"}
				err = store.SetIntent(ctx, intent)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				err = store.DelIntent(ctx, "postgres")
			})

			When("deleting an existing intent", func() {
				It("should remove the intent", func() {
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetIntent(ctx, "postgres")
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
}
