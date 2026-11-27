// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggerConfiguration(t *testing.T) {
	t.Run("console and file targets", func(t *testing.T) {
		c := &Config{}
		c.LogSettings.EnableConsole = true
		c.LogSettings.ConsoleJSON = true
		c.LogSettings.ConsoleLevel = "DEBUG"
		c.LogSettings.EnableFile = true
		c.LogSettings.FileLevel = "info"
		c.LogSettings.FileLocation = "/var/log/mattermod"

		cfg, err := loggerConfiguration(c)
		require.NoError(t, err)
		require.Len(t, cfg, 2)

		console := cfg["_defConsole"]
		require.Equal(t, "console", console.Type)
		require.Equal(t, "json", console.Format)
		require.JSONEq(t, `{"out": "stdout"}`, string(console.Options))
		require.Equal(t, "debug", console.Levels[len(console.Levels)-1].Name)

		file := cfg["_defFile"]
		require.Equal(t, "file", file.Type)
		require.Equal(t, "plain", file.Format)
		require.Equal(t, "info", file.Levels[len(file.Levels)-1].Name)
		var options struct {
			Filename string `json:"filename"`
		}
		require.NoError(t, json.Unmarshal(file.Options, &options))
		require.Equal(t, filepath.Join("/var/log/mattermod", logFilename), options.Filename)
	})

	t.Run("no targets when both are disabled", func(t *testing.T) {
		cfg, err := loggerConfiguration(&Config{})
		require.NoError(t, err)
		require.Empty(t, cfg)
	})

	t.Run("unknown level", func(t *testing.T) {
		c := &Config{}
		c.LogSettings.EnableConsole = true
		c.LogSettings.ConsoleLevel = "verbose"

		_, err := loggerConfiguration(c)
		require.EqualError(t, err, "verbose is not a standard level")
	})
}

func TestLevelsUpTo(t *testing.T) {
	levels, err := levelsUpTo("WARN")
	require.NoError(t, err)

	names := make([]string, 0, len(levels))
	for _, l := range levels {
		names = append(names, l.Name)
	}
	require.Equal(t, []string{"panic", "fatal", "error", "warn"}, names)
}
