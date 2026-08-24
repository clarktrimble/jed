package jed_test

import (
	"github.com/clarktrimble/jed"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Images", func() {
	Describe("Configs", func() {
		It("returns configs by image ref for the requested platform", func() {
			amd64Config := jed.ImageConfig{Os: "linux", Architecture: "amd64", User: "app"}
			arm64Config := jed.ImageConfig{Os: "linux", Architecture: "arm64"}

			images := jed.Images{
				{
					Registry:   "registry.example.com",
					Repository: "repo/app",
					Tag:        "v1",
					Platforms: []jed.Platform{
						{Name: "linux/amd64", Config: amd64Config},
						{Name: "linux/arm64", Config: arm64Config},
					},
				},
				{
					Repository: "repo/other",
					Tag:        "v2",
					Platforms: []jed.Platform{
						{Name: "linux/amd64", Config: amd64Config},
					},
				},
			}

			Expect(images.Configs("linux/amd64")).To(Equal(map[string]jed.ImageConfig{
				"registry.example.com/repo/app:v1": amd64Config,
				"repo/other:v2":                    amd64Config,
			}))
		})

		It("returns an empty map when no images match the platform", func() {
			images := jed.Images{
				{
					Repository: "repo/app",
					Tag:        "v1",
					Platforms: []jed.Platform{
						{Name: "linux/arm64", Config: jed.ImageConfig{Os: "linux", Architecture: "arm64"}},
					},
				},
			}

			Expect(images.Configs("linux/amd64")).To(BeEmpty())
		})
	})
})
