// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Same target settings as the Mattermost server's simple logging config.
const (
	logFilename     = "mattermod.log"
	logRotateSizeMB = 100
	logMaxQueueSize = 1000
	logMinLevelLen  = 5
	logMinMsgLen    = 45
)

func GetLogFileLocation(fileLocation string) string {
	if fileLocation == "" {
		fileLocation = findDir("logs")
	}

	return filepath.Join(fileLocation, logFilename)
}

// findDir looks for dir in the working directory and up to three parents, then
// the same paths relative to the executable, falling back to "./".
func findDir(dir string) string {
	bases := []string{".", "..", "../..", "../../.."}
	searchPaths := append([]string{}, bases...)
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			for _, base := range bases {
				searchPaths = append(searchPaths, filepath.Join(filepath.Dir(exe), base))
			}
		}
	}

	for _, parent := range searchPaths {
		path, err := filepath.Abs(filepath.Join(parent, dir))
		if err != nil {
			continue
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	return "./"
}

func SetupLogging(c *Config) error {
	cfg, err := loggerConfiguration(c)
	if err != nil {
		return fmt.Errorf("could not configure logger: %w", err)
	}

	logger, err := mlog.NewLogger()
	if err != nil {
		return err
	}

	if err := logger.ConfigureTargets(cfg, nil); err != nil {
		return fmt.Errorf("could not configure config targets: %w", err)
	}

	logger.RedirectStdLog(mlog.LvlStdLog)
	mlog.InitGlobalLogger(logger)

	return nil
}

func loggerConfiguration(c *Config) (mlog.LoggerConfiguration, error) {
	cfg := make(mlog.LoggerConfiguration)
	s := c.LogSettings

	if s.EnableConsole {
		target, err := newLogTarget("console", s.ConsoleLevel, s.ConsoleJSON, s.EnableColor, json.RawMessage(`{"out": "stdout"}`))
		if err != nil {
			return nil, err
		}
		cfg["_defConsole"] = target
	}

	if s.EnableFile {
		options, err := json.Marshal(map[string]any{
			"filename":    GetLogFileLocation(s.FileLocation),
			"max_size":    logRotateSizeMB,
			"max_age":     0,
			"max_backups": 0,
			"compress":    true,
		})
		if err != nil {
			return nil, err
		}
		target, err := newLogTarget("file", s.FileLevel, s.FileJSON, false, options)
		if err != nil {
			return nil, err
		}
		cfg["_defFile"] = target
	}

	return cfg, nil
}

func newLogTarget(targetType, level string, outputJSON, color bool, options json.RawMessage) (mlog.TargetCfg, error) {
	levels, err := levelsUpTo(level)
	if err != nil {
		return mlog.TargetCfg{}, err
	}

	target := mlog.TargetCfg{
		Type:         targetType,
		Levels:       levels,
		Options:      options,
		MaxQueueSize: logMaxQueueSize,
	}

	if outputJSON {
		target.Format = "json"
		target.FormatOptions = json.RawMessage(`{"enable_caller": true}`)
	} else {
		target.Format = "plain"
		target.FormatOptions = json.RawMessage(fmt.Sprintf(
			`{"delim": " ", "min_level_len": %d, "min_msg_len": %d, "enable_color": %t, "enable_caller": true}`,
			logMinLevelLen, logMinMsgLen, color))
	}

	return target, nil
}

// levelsUpTo returns every standard level up to and including the named one.
func levelsUpTo(name string) ([]mlog.Level, error) {
	name = strings.ToLower(name)
	var maxLevel *mlog.Level
	for i := range mlog.StdAll {
		if mlog.StdAll[i].Name == name {
			maxLevel = &mlog.StdAll[i]
			break
		}
	}
	if maxLevel == nil {
		return nil, fmt.Errorf("%s is not a standard level", name)
	}

	var levels []mlog.Level
	for _, l := range mlog.StdAll {
		if l.ID <= maxLevel.ID {
			levels = append(levels, l)
		}
	}

	return levels, nil
}
