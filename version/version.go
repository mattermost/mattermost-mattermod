package version

import (
	"time"
)

// current version
const dev = "v0.3.5-alpha"

// Provisioned by ldflags
var (
	version    string
	commitHash string
	buildDate  string
)

type Info struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
	Date    string `json:"date"`
}

func init() {
	// Load defaults for info variables
	if version == "" {
		version = dev
	}
	if commitHash == "" {
		commitHash = dev
	}
	if buildDate == "" {
		buildDate = time.Now().Format(time.RFC3339)
	}
}

// Full return the full information including version, commit hash and build date
func Full() *Info {
	return &Info{
		Version: version,
		Hash:    commitHash,
		Date:    buildDate,
	}
}
