package server

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeSpinWickID(t *testing.T) {
	tests := []struct {
		repoName string
		prNumber int
	}{
		{"mattermost-server", 12345},
		{"mattermost-webapp", 54321},
		{"mattermost-fusion-reactor", 777},
	}

	for _, tc := range tests {
		t.Run(tc.repoName, func(t *testing.T) {
			id := makeSpinWickID(tc.repoName, tc.prNumber)
			assert.Contains(t, id, tc.repoName)
			assert.Contains(t, id, fmt.Sprintf("%d", tc.prNumber))
		})
	}
}

func TestIsSpinWickLabel(t *testing.T) {
	spinwickLabel := "spinwick"
	spinwickHALabel := "spinwick ha"
	s := &Server{
		Config: &ServerConfig{
			SetupSpinWick:   spinwickLabel,
			SetupSpinWickHA: spinwickHALabel,
		},
	}

	tests := []struct {
		label    string
		expected bool
	}{
		{spinwickLabel, true},
		{spinwickHALabel, true},
		{"not a spinwick label", false},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			require.Equal(t, tc.expected, s.isSpinWickLabel(tc.label))
		})
	}
}
