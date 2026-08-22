package scanner

import (
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestScanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scanner Suite")
}

var _ = Describe("registry JSON decode", func() {
	Describe("ociIndex", func() {
		It("decodes the OCI index found in the registry transcript", func() {
			var idx ociIndex
			Expect(json.Unmarshal(loadRegistryData("index-oci.json"), &idx)).To(Succeed())

			Expect(idx.MediaType).To(Equal("application/vnd.oci.image.index.v1+json"))
			Expect(idx.Manifests).To(HaveLen(4))
			Expect(idx.Manifests[0].Digest).To(Equal("sha256:84fc4aa373de1d7a91647962a85d93169ed1df9f9c449790d8dfdb5c9d384300"))
			Expect(idx.Manifests[0].Platform.Os).To(Equal("linux"))
			Expect(idx.Manifests[0].Platform.Architecture).To(Equal("amd64"))
			Expect(idx.Manifests[2].Platform.Os).To(Equal("unknown"))
		})
	})

	Describe("ociManifest", func() {
		It("decodes an OCI manifest found through an index", func() {
			var mfst ociManifest
			Expect(json.Unmarshal(loadRegistryData("manifest-oci.json"), &mfst)).To(Succeed())

			Expect(mfst.MediaType).To(Equal("application/vnd.oci.image.manifest.v1+json"))
			Expect(mfst.Config.Digest).To(Equal("sha256:b072cefe8f23e0f6607d44753ae94c94a63f55f6e36e8761a52d2afd2a6d6504"))
			Expect(mfst.Layers).To(HaveLen(3))
			Expect(mfst.Layers[0].Size).To(Equal(int64(431)))
		})
	})

	Describe("ociConfig", func() {
		It("decodes config fields found in the registry transcript", func() {
			var cfg ociConfig
			Expect(json.Unmarshal(loadRegistryData("config-docker.json"), &cfg)).To(Succeed())

			Expect(cfg.Os).To(Equal("linux"))
			Expect(cfg.Architecture).To(Equal("amd64"))
			Expect(cfg.Config.User).To(Equal("appuser"))
			Expect(cfg.Config.Env).To(ContainElement("PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"))
			Expect(cfg.Config.Entrypoint).To(Equal([]string{"/main"}))
			Expect(cfg.Config.WorkingDir).To(Equal("/"))
			Expect(cfg.Config.Labels).To(BeNil())
		})

		It("decodes labels found on the pushed image", func() {
			var cfg ociConfig
			Expect(json.Unmarshal(loadRegistryData("config-labels.json"), &cfg)).To(Succeed())

			Expect(cfg.Config.Labels).To(Equal(map[string]string{
				"org.bastille.bip.service.env.path":  "/service.env",
				"org.bastille.bip.service.spec.path": "/service.yml",
			}))
		})
	})

})

func loadRegistryData(name string) []byte {
	data, err := os.ReadFile("../test/data/registry/" + name)
	Expect(err).NotTo(HaveOccurred())
	return data
}
