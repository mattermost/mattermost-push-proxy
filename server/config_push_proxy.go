// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ConfigPushProxy struct {
	AndroidPushSettings  []AndroidPushSettings
	ListenAddress        string
	ThrottleVaryByHeader string
	// Deprecated: Use it is maintained for backward compatibility of the Logger struct. Use LoggingCfgFile or LoggingCfgJSON instead.
	LogFileLocation   string
	SendTimeoutSec    int
	RetryTimeoutSec   int
	ApplePushSettings []ApplePushSettings
	EnableMetrics     bool
	// Deprecated: Same reason as LogFileLocation.
	EnableConsoleLog bool
	// Deprecated: Same reason as LogFileLocation.
	EnableFileLog           bool
	LoggingCfgFile          string
	LoggingCfgJSON          string
	ThrottlePerSec          int
	ThrottleMemoryStoreSize int
}

type ApplePushSettings struct {
	Type                    string
	ApplePushCertPrivate    string
	ApplePushCertPassword   string
	ApplePushTopic          string
	AppleAuthKeyFile        string
	AppleAuthKeyID          string
	AppleTeamID             string
	ApplePushUseDevelopment bool
}

type AndroidPushSettings struct {
	Type                string
	AndroidAPIKey       string `json:"AndroidApiKey"`
	ServiceFileLocation string `json:"ServiceFileLocation"`
}

// FindConfigFile searches for the filepath in a list of directories
// and then returns the absolute path to that file.
func FindConfigFile(fileName string) string {
	if _, err := os.Stat("/tmp/" + fileName); err == nil {
		fileName, _ = filepath.Abs("/tmp/" + fileName)
	} else if _, err := os.Stat("./config/" + fileName); err == nil {
		fileName, _ = filepath.Abs("./config/" + fileName)
	} else if _, err := os.Stat("../config/" + fileName); err == nil {
		fileName, _ = filepath.Abs("../config/" + fileName)
	} else if _, err := os.Stat(fileName); err == nil {
		fileName, _ = filepath.Abs(fileName)
	}

	return fileName
}

// LoadConfig loads the config from the given file path.
func LoadConfig(fileName string) (*ConfigPushProxy, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var cfg *ConfigPushProxy
	err = json.Unmarshal(buf, &cfg)
	if err != nil {
		fmt.Println(buf, err)
		return nil, err
	}

	// Set timeout defaults
	if cfg.SendTimeoutSec == 0 {
		cfg.SendTimeoutSec = 30
	}

	if cfg.RetryTimeoutSec == 0 {
		cfg.RetryTimeoutSec = 8
	}

	if cfg.RetryTimeoutSec > cfg.SendTimeoutSec {
		cfg.RetryTimeoutSec = cfg.SendTimeoutSec
	}

	return cfg, nil
}
