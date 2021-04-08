// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	App          string
	CertDir      string
	AppleGateway string
}

func parseConfig(path string) (config, error) {
	var cfg config
	file, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("could not decode file: %w", err)
	}

	if err = envconfig.Process("convert_cert", &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
