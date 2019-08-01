package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// GetInstallation returns an installation for a given ID.
func GetInstallation(serverURL, installationID string) (Installation, error) {
	var installation Installation
	url := fmt.Sprintf("%s/api/installation/%s", serverURL, installationID)
	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		return installation, errors.Wrap(err, "unable to get installation")
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&installation)
	if err != nil && err != io.EOF {
		return installation, errors.Wrap(err, "Error decoding installation")
	}

	return installation, nil
}

// GetInstallationList returns a list on non-deleted installations from the
// provisioning server.
func GetInstallationList(serverURL string) ([]Installation, error) {
	var installations []Installation

	url := fmt.Sprintf("%s/api/installations", serverURL)
	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		return installations, errors.Wrap(err, "error gettings installations")
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&installations)
	if err != nil && err != io.EOF {
		return installations, errors.Wrap(err, "error decoding installations")
	}

	return installations, nil
}

// GetInstallationIDFromOwnerID returns the ID of an installtion that matches
// a given OwnerID. Multiple matches will return an error. No match will return
// an empty ID and no error.
func GetInstallationIDFromOwnerID(serverURL, ownerID string) (string, error) {
	installations, err := GetInstallationList(serverURL)
	if err != nil {
		return "", err
	}

	var matchingInstallations []Installation
	for _, installation := range installations {
		if installation.OwnerID == ownerID {
			matchingInstallations = append(matchingInstallations, installation)
		}
	}

	if len(matchingInstallations) == 0 {
		return "", nil
	}
	if len(matchingInstallations) == 1 {
		return matchingInstallations[0].ID, nil
	}

	return "", fmt.Errorf("found %d installations with ownerID %s", len(matchingInstallations), ownerID)
}

func makeRequest(method, url string, payload io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
