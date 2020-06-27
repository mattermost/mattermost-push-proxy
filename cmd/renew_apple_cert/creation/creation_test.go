package creation

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

	err = createDirs([]string{
		os.Getenv("DIR_CSR"),
		os.Getenv("DIR_DOWNLOADED"),
	})
	if err != nil {
		log.Fatal(err.Error())
	}
	defer os.RemoveAll(path.Join("testdata", "certs"))
	os.Exit(m.Run())
}

func TestCSRCreation(t *testing.T) {
	Creation()
	if _, err := os.Stat(path.Join("testdata", "certs", "csr", "mattermost.key")); os.IsNotExist(err) {
		t.Fatalf(err.Error())
	}

	if _, err := os.Stat(path.Join("testdata", "certs", "csr", "mattermost.csr")); os.IsNotExist(err) {
		t.Fatalf(err.Error())
	}
}
