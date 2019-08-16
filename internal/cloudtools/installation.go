package cloudtools

import (
	"fmt"

	cloudModel "github.com/mattermost/mattermost-cloud/model"
)

// GetInstallationIDFromOwnerID returns the ID of an installation that matches
// a given OwnerID. Multiple matches will return an error. No match will return
// an empty ID and no error.
func GetInstallationIDFromOwnerID(serverURL, ownerID string) (string, error) {
	cloudClient := cloudModel.NewClient(serverURL)
	installations, err := cloudClient.GetInstallations(&cloudModel.GetInstallationsRequest{
		OwnerID:        ownerID,
		Page:           0,
		PerPage:        100,
		IncludeDeleted: false,
	})
	if err != nil {
		return "", err
	}

	if len(installations) == 0 {
		return "", nil
	}
	if len(installations) == 1 {
		return installations[0].ID, nil
	}

	return "", fmt.Errorf("found %d installations with ownerID %s", len(installations), ownerID)
}
