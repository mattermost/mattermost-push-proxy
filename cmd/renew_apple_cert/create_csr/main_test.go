package main

import (
	"log"
	"os"
	"path"
	"testing"
)

func TestMain(m *testing.M) {
	err := os.Unsetenv("ENV_PUSH_PROXY")
	if err != nil {
		log.Fatal(err.Error())
	}

	err = createDirs(os.Getenv("CERT_DIR"))
	if err != nil {
		log.Fatal(err.Error())
	}
	defer os.RemoveAll(path.Join("testdata", "certs"))
	os.Exit(m.Run())
}

func TestCSRCreation(t *testing.T) {
	main()
	if _, err := os.Stat(path.Join("testdata", "certs", "mattermost", "csr", "mattermost.key")); os.IsNotExist(err) {
		t.Fatalf(err.Error())
	}

	if _, err := os.Stat(path.Join("testdata", "certs", "mattermost", "csr", "mattermost.csr")); os.IsNotExist(err) {
		t.Fatalf(err.Error())
	}
}
