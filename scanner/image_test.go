package scanner_test

import (
	"github.com/clarktrimble/jed/scanner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Images", func() {
	Describe("Configs", func() {
		It("returns configs by image ref for the requested platform", func() {
			amd64Config := scanner.Config{Os: "linux", Architecture: "amd64", User: "app"}
			arm64Config := scanner.Config{Os: "linux", Architecture: "arm64"}

			images := scanner.Images{
				{
					Registry:   "registry.example.com",
					Repository: "repo/app",
					Tag:        "v1",
					Platforms: []scanner.Platform{
						{Name: "linux/amd64", Config: amd64Config},
						{Name: "linux/arm64", Config: arm64Config},
					},
				},
				{
					Repository: "repo/other",
					Tag:        "v2",
					Platforms: []scanner.Platform{
						{Name: "linux/amd64", Config: amd64Config},
					},
				},
			}

			Expect(images.Configs("linux/amd64")).To(Equal(map[string]scanner.Config{
				"registry.example.com/repo/app:v1": amd64Config,
				"repo/other:v2":                    amd64Config,
			}))
		})

		It("returns an empty map when no images match the platform", func() {
			images := scanner.Images{
				{
					Repository: "repo/app",
					Tag:        "v1",
					Platforms: []scanner.Platform{
						{Name: "linux/arm64", Config: scanner.Config{Os: "linux", Architecture: "arm64"}},
					},
				},
			}

			Expect(images.Configs("linux/amd64")).To(BeEmpty())
		})
	})
})
