package store

import (
	"context"

	"github.com/pkg/errors"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

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
	var _ = ginkgo.Describe(name+" Store Contract", func() {
		var (
			store   jed.Store
			ctx     context.Context
			err     error
			service jed.Service
			image   jed.Image
			env     jed.Env
			intent  jed.Intent
		)

		ginkgo.BeforeEach(func() {
			ctx = context.Background()
			store, err = newStore(ctx)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		ginkgo.Describe("SetService and GetService", func() {
			var retrievedService jed.Service

			ginkgo.BeforeEach(func() {
				service = jed.Service{
					Name:           "postgres",
					Image:          "postgres:14",
					Network:        "app-net",
					Restart:        jed.RestartAny,
					NetworkVolumes: []string{"/var/lib/postgresql/data"},
					Integration: &jed.Integration{
						Name:     "infra",
						Desc:     "Shared infrastructure services",
						Versions: []string{"v1.2.3"},
					},
					Ports: map[string]string{
						"5432/tcp": "5432",
					},
					Labels: map[string]string{
						"app": "myapp",
					},
				}
			})

			ginkgo.JustBeforeEach(func() {
				err = store.SetService(ctx, service)
			})

			ginkgo.When("service is set successfully", func() {
				ginkgo.It("should store and retrieve the service", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					retrievedService, err = store.GetService(ctx, "postgres", "postgres:14")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(retrievedService.Name).To(gomega.Equal("postgres"))
					gomega.Expect(retrievedService.Image).To(gomega.Equal("postgres:14"))
					gomega.Expect(retrievedService.Integration).To(gomega.Equal(service.Integration))
					gomega.Expect(retrievedService.NetworkVolumes).To(gomega.Equal(service.NetworkVolumes))
					gomega.Expect(retrievedService.Ports["5432/tcp"]).To(gomega.Equal("5432"))
				})
			})

			ginkgo.When("getting a non-existent service", func() {
				ginkgo.It("should return NotFoundError", func() {
					_, err = store.GetService(ctx, "nonexistent", "postgres:14")
					gomega.Expect(err).To(gomega.HaveOccurred())

					var notFound jed.NotFoundError
					gomega.Expect(errors.As(err, &notFound)).To(gomega.BeTrue())
					gomega.Expect(notFound.Kind).To(gomega.Equal("service"))
					gomega.Expect(notFound.Name).To(gomega.Equal("nonexistent@postgres:14"))
				})
			})
		})

		ginkgo.Describe("Services", func() {
			var services []jed.Service

			ginkgo.JustBeforeEach(func() {
				services, err = store.Services(ctx, "postgres")
			})

			ginkgo.When("no services exist", func() {
				ginkgo.It("should return empty slice", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(services).To(gomega.BeEmpty())
				})
			})

			ginkgo.When("multiple services exist for the same name", func() {
				ginkgo.BeforeEach(func() {
					err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:14"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:15"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = store.SetService(ctx, jed.Service{Name: "redis", Image: "redis:7"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})

				ginkgo.It("should return services with the requested name", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(services).To(gomega.HaveLen(2))
					images := []string{services[0].Image, services[1].Image}
					gomega.Expect(images).To(gomega.ContainElements("postgres:14", "postgres:15"))
				})
			})
		})

		ginkgo.Describe("AllServices", func() {
			var services []jed.Service

			ginkgo.JustBeforeEach(func() {
				services, err = store.AllServices(ctx)
			})

			ginkgo.When("no services exist", func() {
				ginkgo.It("should return empty slice", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(services).To(gomega.BeEmpty())
				})
			})

			ginkgo.When("multiple services exist", func() {
				ginkgo.BeforeEach(func() {
					err = store.SetService(ctx, jed.Service{Name: "postgres", Image: "postgres:14"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = store.SetService(ctx, jed.Service{Name: "redis", Image: "redis:7"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})

				ginkgo.It("should return all services", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(services).To(gomega.HaveLen(2))
					names := []string{services[0].Name, services[1].Name}
					gomega.Expect(names).To(gomega.ContainElements("postgres", "redis"))
				})
			})
		})

		ginkgo.Describe("Images", func() {
			var images []jed.Image

			ginkgo.JustBeforeEach(func() {
				images, err = store.Images(ctx)
			})

			ginkgo.When("no images exist", func() {
				ginkgo.It("should return empty slice", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(images).To(gomega.BeEmpty())
				})
			})

			ginkgo.When("multiple images exist", func() {
				ginkgo.BeforeEach(func() {
					err = store.SetImage(ctx, jed.Image{Repository: "postgres", Tag: "16"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = store.SetImage(ctx, jed.Image{Registry: "registry.example.com", Repository: "app/api", Tag: "v1"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})

				ginkgo.It("should return all images", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(images).To(gomega.HaveLen(2))

					imageRefs := []string{images[0].ImageRef(), images[1].ImageRef()}
					gomega.Expect(imageRefs).To(gomega.ContainElements("postgres:16", "registry.example.com/app/api:v1"))
				})
			})
		})

		ginkgo.Describe("SetImage and GetImage", func() {
			var retrievedImage jed.Image

			ginkgo.BeforeEach(func() {
				image = jed.Image{
					Registry:   "registry.example.com",
					Repository: "app/api",
					Tag:        "v1",
					Platforms: []jed.Platform{
						{Name: "linux/amd64", Config: jed.ImageConfig{Os: "linux", Architecture: "amd64", User: "app"}},
					},
				}
			})

			ginkgo.JustBeforeEach(func() {
				err = store.SetImage(ctx, image)
			})

			ginkgo.When("image is set successfully", func() {
				ginkgo.It("should store and retrieve the image", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					retrievedImage, err = store.GetImage(ctx, "registry.example.com/app/api:v1")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(retrievedImage).To(gomega.Equal(image))
				})
			})

			ginkgo.When("getting a non-existent image", func() {
				ginkgo.It("should return NotFoundError", func() {
					_, err = store.GetImage(ctx, "missing:latest")
					gomega.Expect(err).To(gomega.HaveOccurred())

					var notFound jed.NotFoundError
					gomega.Expect(errors.As(err, &notFound)).To(gomega.BeTrue())
					gomega.Expect(notFound.Kind).To(gomega.Equal("image"))
					gomega.Expect(notFound.Name).To(gomega.Equal("missing:latest"))
				})
			})
		})

		ginkgo.Describe("DelImage", func() {
			ginkgo.BeforeEach(func() {
				image = jed.Image{Repository: "postgres", Tag: "16"}
				err = store.SetImage(ctx, image)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.JustBeforeEach(func() {
				err = store.DelImage(ctx, "postgres:16")
			})

			ginkgo.When("deleting an existing image", func() {
				ginkgo.It("should remove the image", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					_, err = store.GetImage(ctx, "postgres:16")
					gomega.Expect(err).To(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Describe("DelService", func() {

			ginkgo.BeforeEach(func() {
				service = jed.Service{Name: "postgres", Image: "postgres:14"}
				err = store.SetService(ctx, service)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.JustBeforeEach(func() {
				err = store.DelService(ctx, "postgres", "postgres:14")
			})

			ginkgo.When("deleting an existing service", func() {
				ginkgo.It("should remove the service", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					_, err = store.GetService(ctx, "postgres", "postgres:14")
					gomega.Expect(err).To(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Describe("SetEnv and GetEnv", func() {
			var retrievedEnv jed.Env

			ginkgo.BeforeEach(func() {
				env = jed.Env{
					Name: "postgres",
					Vars: map[string]string{
						"POSTGRES_PASSWORD": "secret123",
						"POSTGRES_USER":     "admin",
					},
				}
			})

			ginkgo.JustBeforeEach(func() {
				err = store.SetEnv(ctx, env)
			})

			ginkgo.When("env is set successfully", func() {
				ginkgo.It("should store and retrieve the env", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					retrievedEnv, err = store.GetEnv(ctx, "postgres")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(retrievedEnv.Name).To(gomega.Equal("postgres"))
					gomega.Expect(retrievedEnv.Vars["POSTGRES_PASSWORD"]).To(gomega.Equal("secret123"))
					gomega.Expect(retrievedEnv.Vars["POSTGRES_USER"]).To(gomega.Equal("admin"))
				})
			})

			ginkgo.When("getting a non-existent env", func() {
				ginkgo.It("should return empty env", func() {
					retrievedEnv, err := store.GetEnv(ctx, "nonexistent")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(retrievedEnv.Name).To(gomega.Equal("nonexistent"))
					gomega.Expect(retrievedEnv.Vars).To(gomega.BeEmpty())
				})
			})
		})

		ginkgo.Describe("Envs", func() {
			var envs []jed.Env

			ginkgo.JustBeforeEach(func() {
				envs, err = store.Envs(ctx)
			})

			ginkgo.When("no envs exist", func() {
				ginkgo.It("should return empty slice", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(envs).To(gomega.BeEmpty())
				})
			})

			ginkgo.When("multiple envs exist", func() {
				ginkgo.BeforeEach(func() {
					err = store.SetEnv(ctx, jed.Env{
						Name: "postgres",
						Vars: map[string]string{"POSTGRES_PASSWORD": "secret"},
					})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = store.SetEnv(ctx, jed.Env{
						Name: "redis",
						Vars: map[string]string{"REDIS_PASSWORD": "secret"},
					})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})

				ginkgo.It("should return all envs", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(envs).To(gomega.HaveLen(2))
					names := []string{envs[0].Name, envs[1].Name}
					gomega.Expect(names).To(gomega.ContainElements("postgres", "redis"))
				})
			})
		})

		ginkgo.Describe("DelEnv", func() {

			ginkgo.BeforeEach(func() {
				env = jed.Env{
					Name: "postgres",
					Vars: map[string]string{"POSTGRES_PASSWORD": "secret"},
				}
				err = store.SetEnv(ctx, env)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.JustBeforeEach(func() {
				err = store.DelEnv(ctx, "postgres")
			})

			ginkgo.When("deleting an existing env", func() {
				ginkgo.It("should remove the env", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					retrievedEnv, err := store.GetEnv(ctx, "postgres")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(retrievedEnv.Vars).To(gomega.BeEmpty())
				})
			})
		})

		ginkgo.Describe("SetIntent and GetIntent", func() {
			var retrievedIntent jed.Intent

			ginkgo.BeforeEach(func() {
				intent = jed.Intent{Name: "postgres", Image: "postgres:14", Replicas: 1}
			})

			ginkgo.JustBeforeEach(func() {
				err = store.SetIntent(ctx, intent)
			})

			ginkgo.When("intent is set successfully", func() {
				ginkgo.It("should store and retrieve the intent", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					retrievedIntent, err = store.GetIntent(ctx, "postgres")
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(retrievedIntent.Name).To(gomega.Equal("postgres"))
					gomega.Expect(retrievedIntent.Image).To(gomega.Equal("postgres:14"))
					gomega.Expect(retrievedIntent.Replicas).To(gomega.Equal(1))
				})
			})

			ginkgo.When("getting a non-existent intent", func() {
				ginkgo.It("should return NotFoundError", func() {
					_, err = store.GetIntent(ctx, "nonexistent")
					gomega.Expect(err).To(gomega.HaveOccurred())

					var notFound jed.NotFoundError
					gomega.Expect(errors.As(err, &notFound)).To(gomega.BeTrue())
					gomega.Expect(notFound.Kind).To(gomega.Equal("intent"))
					gomega.Expect(notFound.Name).To(gomega.Equal("nonexistent"))
				})
			})
		})

		ginkgo.Describe("Intents", func() {
			var intents []jed.Intent

			ginkgo.JustBeforeEach(func() {
				intents, err = store.Intents(ctx)
			})

			ginkgo.When("no intents exist", func() {
				ginkgo.It("should return empty slice", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(intents).To(gomega.BeEmpty())
				})
			})

			ginkgo.When("multiple intents exist", func() {
				ginkgo.BeforeEach(func() {
					err = store.SetIntent(ctx, jed.Intent{Name: "postgres", Image: "postgres:14", Replicas: 1})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					err = store.SetIntent(ctx, jed.Intent{Name: "redis", Image: "redis:7"})
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				})

				ginkgo.It("should return all intents", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(intents).To(gomega.HaveLen(2))
					names := []string{intents[0].Name, intents[1].Name}
					gomega.Expect(names).To(gomega.ContainElements("postgres", "redis"))
				})
			})
		})

		ginkgo.Describe("DelIntent", func() {

			ginkgo.BeforeEach(func() {
				intent = jed.Intent{Name: "postgres", Image: "postgres:14"}
				err = store.SetIntent(ctx, intent)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.JustBeforeEach(func() {
				err = store.DelIntent(ctx, "postgres")
			})

			ginkgo.When("deleting an existing intent", func() {
				ginkgo.It("should remove the intent", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					_, err = store.GetIntent(ctx, "postgres")
					gomega.Expect(err).To(gomega.HaveOccurred())
				})
			})
		})
	})
}
