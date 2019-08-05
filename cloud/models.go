package cloud

// The following structs are copied from the mattermost-cloud repo to allow
// mattermod to interact with provisioning servers.
//
// TODO: consider moving the structs in mattermost-cloud for these models out
// of the /internal directory so that they can be vendored and imported here.
// When doing this, we should start using semver in the mattermost-cloud repo.

// CreateClusterRequest specifies the parameters for a new cluster.
type CreateClusterRequest struct {
	Provider string
	Size     string
	Zones    []string
}

// Cluster represents a Kubernetes cluster.
type Cluster struct {
	ID                  string
	Provider            string
	Provisioner         string
	ProviderMetadata    []byte `json:",omitempty"`
	ProvisionerMetadata []byte `json:",omitempty"`
	AllowInstallations  bool
	Size                string
	State               string
	CreateAt            int64
	DeleteAt            int64
	LockAcquiredBy      *string
	LockAcquiredAt      int64
}

// CreateInstallationRequest specifies the parameters for a new installation.
type CreateInstallationRequest struct {
	OwnerID  string
	Version  string
	DNS      string
	Size     string
	Affinity string
}

// Installation represents a Mattermost installation.
type Installation struct {
	ID             string
	OwnerID        string
	Version        string
	DNS            string
	Size           string
	Affinity       string
	GroupID        *string
	State          string
	CreateAt       int64
	DeleteAt       int64
	LockAcquiredBy *string
	LockAcquiredAt int64
}
