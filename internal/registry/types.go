package registry

// Media types we accept when fetching manifests.
const (
	MediaManifestListDocker = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaManifestDocker     = "application/vnd.docker.distribution.manifest.v2+json"
	MediaIndexOCI           = "application/vnd.oci.image.index.v1+json"
	MediaManifestOCI        = "application/vnd.oci.image.manifest.v1+json"
	MediaConfigDocker       = "application/vnd.docker.container.image.v1+json"
	MediaConfigOCI          = "application/vnd.oci.image.config.v1+json"
	MediaLayerGzip          = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	MediaLayerOCIGzip       = "application/vnd.oci.image.layer.v1.tar+gzip"
)

// Platform is an OCI platform selector.
type Platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant,omitempty"`
}

// Index is a manifest list / OCI index.
type Index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []IndexEntry `json:"manifests"`
}

// IndexEntry points at a platform-specific manifest.
type IndexEntry struct {
	MediaType string   `json:"mediaType"`
	Digest    string   `json:"digest"`
	Size      int64    `json:"size"`
	Platform  Platform `json:"platform"`
}

// Manifest is an image manifest (Docker v2 / OCI).
type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

// Descriptor is an OCI content descriptor.
type Descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// ImageConfig is the container image configuration JSON.
type ImageConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Created      string `json:"created"`
	Config       struct {
		User         string              `json:"User"`
		Env          []string            `json:"Env"`
		Entrypoint   []string            `json:"Entrypoint"`
		Cmd          []string            `json:"Cmd"`
		WorkingDir   string              `json:"WorkingDir"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}
