package scanner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/clarktrimble/jed/logger/loggertest"
	"github.com/clarktrimble/jed/scanner"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test registry responses are in the spirit of:
// test/data/registry/index-oci.json
// test/data/registry/manifest-docker.json
// test/data/registry/config-docker.json
// Ground mock responses against actual registry data stored in test/data.

var _ = Describe("Scanner", func() {
	Describe("Images", func() {
		var (
			ctx         context.Context
			client      *ClientMock
			logger      *loggertest.LoggerMock
			routes      map[string][]byte
			routeErrors map[string]error
			scn         *scanner.Scanner
			images      []scanner.Image
			err         error
		)

		BeforeEach(func() {
			ctx = context.Background()
			logger = loggertest.NewLoggerMock()
			routes = map[string][]byte{}
			routeErrors = map[string]error{}
			client = &ClientMock{
				UriFunc: func() string {
					return "https://registry.example.com"
				},
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					Expect(method).To(Equal(http.MethodGet))
					Expect(snd).To(BeNil())

					err, ok := routeErrors[path]
					if ok {
						return err
					}

					response, ok := routes[path]
					if !ok {
						return errors.Errorf("unexpected request: %s", path)
					}
					return json.Unmarshal(response, rcv)
				},
			}
			scn, err = scanner.New(client, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			images, err = scn.Scan(ctx)
		})

		When("a tag resolves through an image index", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "indexed")
				routes["/v2/repo/manifests/indexed"] = []byte(`{
					"mediaType": "application/vnd.oci.image.index.v1+json",
					"manifests": [
						{
							"digest": "sha256:amd64-manifest",
							"platform": {"os": "linux", "architecture": "amd64"}
						},
						{
							"digest": "sha256:arm64-manifest",
							"platform": {"os": "linux", "architecture": "arm64"}
						},
						{
							"digest": "sha256:arm-manifest",
							"platform": {"os": "linux", "architecture": "arm", "variant": "v7"}
						},
						{
							"digest": "sha256:attestation",
							"platform": {"os": "unknown", "architecture": "unknown"}
						}
					]
				}`)
				routes["/v2/repo/manifests/sha256:amd64-manifest"] = imageManifest("sha256:amd64-config")
				routes["/v2/repo/manifests/sha256:arm64-manifest"] = imageManifest("sha256:arm64-config")
				routes["/v2/repo/manifests/sha256:arm-manifest"] = imageManifest("sha256:arm-config")
				routes["/v2/repo/blobs/sha256:amd64-config"] = imageConfig("linux", "amd64")
				routes["/v2/repo/blobs/sha256:arm64-config"] = imageConfig("linux", "arm64")
				routes["/v2/repo/blobs/sha256:arm-config"] = imageConfig("linux", "arm")
			})

			It("returns configs for the indexed platforms", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(images).To(HaveLen(1))
				Expect(images[0].Registry).To(Equal("registry.example.com"))
				Expect(images[0].Repository).To(Equal("repo"))
				Expect(images[0].Tag).To(Equal("indexed"))
				Expect(platformNames(images[0].Platforms)).To(ConsistOf("linux/amd64", "linux/arm64", "linux/arm/v7"))
				Expect(scanner.Images(images).Configs("linux/amd64")).To(Equal(map[string]scanner.Config{
					"registry.example.com/repo:indexed": {
						Os:           "linux",
						Architecture: "amd64",
						User:         "appuser",
						WorkingDir:   "/",
					},
				}))
				Expect(logger.ErrorCalls()).To(BeEmpty())
			})
		})

		When("a tag resolves directly to an image manifest", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "direct")
				routes["/v2/repo/manifests/direct"] = []byte(`{
					"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
					"config": {"digest": "sha256:direct-config"}
				}`)
				routes["/v2/repo/blobs/sha256:direct-config"] = imageConfig("linux", "amd64")
			})

			It("returns config using the platform from image config", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(images).To(HaveLen(1))
				Expect(images[0].Platforms).To(HaveLen(1))
				Expect(images[0].Platforms[0].Name).To(Equal("linux/amd64"))
				Expect(images[0].Platforms[0].Config.Os).To(Equal("linux"))
				Expect(images[0].Platforms[0].Config.Architecture).To(Equal("amd64"))
				Expect(logger.ErrorCalls()).To(BeEmpty())
			})
		})

		When("a tag resolves to an unsupported manifest media type", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "unsupported")
				routes["/v2/repo/manifests/unsupported"] = []byte(`{
					"mediaType": "application/vnd.example.unsupported"
				}`)
			})

			It("logs the missing references and continues", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(images).To(HaveLen(1))
				Expect(images[0].Platforms).To(BeEmpty())
				Expect(errorMessages(logger)).To(ConsistOf("failed to get references"))
			})
		})

		When("an image index has no usable platform references", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "attestations")
				routes["/v2/repo/manifests/attestations"] = []byte(`{
					"mediaType": "application/vnd.oci.image.index.v1+json",
					"manifests": [
						{
							"digest": "sha256:attestation",
							"platform": {"os": "unknown", "architecture": "unknown"}
						}
					]
				}`)
			})

			It("logs the missing references and continues", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(images).To(HaveLen(1))
				Expect(images[0].Platforms).To(BeEmpty())
				Expect(errorMessages(logger)).To(ConsistOf("failed to get references"))
			})
		})

		When("a direct manifest config has no platform", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "missing-platform")
				routes["/v2/repo/manifests/missing-platform"] = []byte(`{
					"mediaType": "application/vnd.oci.image.manifest.v1+json",
					"config": {"digest": "sha256:missing-platform-config"}
				}`)
				routes["/v2/repo/blobs/sha256:missing-platform-config"] = imageConfig("", "")
			})

			It("logs the missing platform and skips the platform", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(images).To(HaveLen(1))
				Expect(images[0].Platforms).To(BeEmpty())
				Expect(errorMessages(logger)).To(ConsistOf("failed to determine platform for direct manifest"))
			})
		})

		When("listing repositories fails", func() {
			BeforeEach(func() {
				routeErrors["/v2/_catalog"] = errors.New("request failed")
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("request failed"))
				Expect(images).To(BeNil())
			})
		})

		When("listing tags fails", func() {
			BeforeEach(func() {
				routes["/v2/_catalog"] = []byte(`{"repositories": ["repo"]}`)
				routeErrors["/v2/repo/tags/list"] = errors.New("request failed")
			})

			It("returns the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("list tags for repo"))
				Expect(err.Error()).To(ContainSubstring("request failed"))
				Expect(images).To(BeNil())
			})
		})

		When("fetching a tag manifest fails", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "indexed")
				routeErrors["/v2/repo/manifests/indexed"] = errors.New("request failed")
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("request failed"))
				Expect(images).To(HaveLen(1))
			})
		})

		When("fetching an indexed image manifest fails", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "indexed")
				routes["/v2/repo/manifests/indexed"] = imageIndex("sha256:amd64-manifest", "linux", "amd64")
				routeErrors["/v2/repo/manifests/sha256:amd64-manifest"] = errors.New("request failed")
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("request failed"))
				Expect(images).To(HaveLen(1))
			})
		})

		When("fetching an image config fails", func() {
			BeforeEach(func() {
				addTag(routes, "repo", "indexed")
				routes["/v2/repo/manifests/indexed"] = imageIndex("sha256:amd64-manifest", "linux", "amd64")
				routes["/v2/repo/manifests/sha256:amd64-manifest"] = imageManifest("sha256:amd64-config")
				routeErrors["/v2/repo/blobs/sha256:amd64-config"] = errors.New("request failed")
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("request failed"))
				Expect(images).To(HaveLen(1))
			})
		})
	})
})

func addTag(routes map[string][]byte, repository, tag string) {
	routes["/v2/_catalog"] = []byte(fmt.Sprintf(`{"repositories": [%q]}`, repository))
	routes["/v2/"+repository+"/tags/list"] = []byte(fmt.Sprintf(`{"name": %q, "tags": [%q]}`, repository, tag))
}

func imageIndex(digest, os, architecture string) []byte {
	return []byte(fmt.Sprintf(`{
		"mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": [
			{
				"digest": %q,
				"platform": {"os": %q, "architecture": %q}
			}
		]
	}`, digest, os, architecture))
}

func imageManifest(configDigest string) []byte {
	return []byte(fmt.Sprintf(`{
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": {"digest": %q}
	}`, configDigest))
}

func imageConfig(os, architecture string) []byte {
	return []byte(fmt.Sprintf(`{
		"os": %q,
		"architecture": %q,
		"config": {
			"User": "appuser",
			"WorkingDir": "/"
		}
	}`, os, architecture))
}

func platformNames(platforms []scanner.Platform) []string {
	names := make([]string, len(platforms))
	for i, platform := range platforms {
		names[i] = platform.Name
	}
	return names
}

func errorMessages(logger *loggertest.LoggerMock) []string {
	calls := logger.ErrorCalls()
	messages := make([]string, len(calls))
	for i, call := range calls {
		messages[i] = call.Msg
	}
	return messages
}
