// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type ConfigPushProxy struct {
	ListenAddress           string
	ThrottlePerSec          int
	ThrottleMemoryStoreSize int
	ThrottleVaryByHeader    string
	EnableMetrics           bool
	ApplePushSettings       []ApplePushSettings
	AndroidPushSettings     []AndroidPushSettings
	EnableConsoleLog        bool
	EnableFileLog           bool
	LogFileLocation         string
}

type ApplePushSettings struct {
	Type                    string
	ApplePushUseDevelopment bool
	ApplePushCertPrivate    string
	ApplePushCertPassword   string
	ApplePushTopic          string
}

type AndroidPushSettings struct {
	Type          string
	AndroidAPIKey string `json:"AndroidApiKey"`
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

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var cfg *ConfigPushProxy
	err = json.Unmarshal(buf, &cfg)
	if err != nil {
		fmt.Println(buf, err)
		return nil, err
	}
	// If both are disabled, that means an old config file is being used. Atleast enable console log.
	if !cfg.EnableConsoleLog && !cfg.EnableFileLog {
		cfg.EnableConsoleLog = true
	}
	if cfg.EnableFileLog {
		if cfg.LogFileLocation == "" {
			// We just do an mkdir -p equivalent.
			// Otherwise, it would need 2 steps of statting and creating.
			err := os.MkdirAll("./logs", 0755)
			if err != nil {
				// If it fails, we log in the current directory itself
				cfg.LogFileLocation = "./push_proxy.log"
			} else {
				cfg.LogFileLocation = "./logs/push_proxy.log"
			}
		}
		// if file does not exist, create it.
		if _, err := os.Stat(cfg.LogFileLocation); os.IsNotExist(err) {
			f, err := os.Create(cfg.LogFileLocation)
			if err != nil {
				return nil, err
			}
			if err := f.Close(); err != nil {
				return nil, err
			}
		}
	}

	return cfg, nil
}
