// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ConfigPushProxy struct {
	ListenAddress           string
	ApplePushServer         string
	ApplePushCertPublic     string
	ApplePushCertPrivate    string
	ThrottlePerSec          int
	ThrottleMemoryStoreSize int
	ThrottleVaryByHeader    string
}

var CfgPP *ConfigPushProxy = &ConfigPushProxy{}

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

func LoadConfig(fileName string) {
	fileName = FindConfigFile(fileName)
	LogInfo("Loading " + fileName)

	file, err := os.Open(fileName)
	if err != nil {
		LogCritical("Error opening config file=" + fileName + ", err=" + err.Error())
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(CfgPP)
	if err != nil {
		LogCritical("Error decoding config file=" + fileName + ", err=" + err.Error())
	}
}
