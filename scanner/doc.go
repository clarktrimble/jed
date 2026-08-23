// Package scanner lists images in a container registry and returns selected
// image config fields grouped by platform (os/architecture[/variant]).
//
// # Indexed and direct manifests
//
// Registry tags may resolve to a config through an image index or directly via
// an image manifest.
//
// Indexed:
//
//	{
//	  "mediaType": "application/vnd.oci.image.index.v1+json",
//	  "manifests": [
//	    {
//	      "digest": "sha256:...",
//	      "platform": {"os": "linux", "architecture": "amd64"}
//	    }
//	  ]
//	}
//
// Direct:
//
//	{
//	  "mediaType": "application/vnd.oci.image.manifest.v1+json",
//	  "config": {"digest": "sha256:..."}
//	}
package scanner
