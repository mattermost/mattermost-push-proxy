package main

import (
	"os"
	"path"
	"testing"
)

func TestCSRCreation(t *testing.T) {
	defer os.RemoveAll(path.Join("testdata", "certs"))
	runJob(path.Join("testdata", "config.json"))
	if _, err := os.Stat(path.Join("testdata", "certs", "mattermost", "csr", "mattermost.key")); os.IsNotExist(err) {
		t.Error(err)
	}

	if _, err := os.Stat(path.Join("testdata", "certs", "mattermost", "csr", "mattermost.csr")); os.IsNotExist(err) {
		t.Error(err)
	}
}
