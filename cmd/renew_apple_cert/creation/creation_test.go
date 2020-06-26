package creation

import (
	"log"
	"os"
	"path"
	"testing"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	err := godotenv.Overload(path.Join("testdata", ".env.testdata"))
	if err != nil {
		log.Fatal(err.Error())
	}

	dirCSR := os.Getenv("DIR_CSR")
	dirDownloaded := os.Getenv("DIR_DOWNLOADED")
	dirs := []string{
		dirCSR,
		dirDownloaded,
	}
	for _, dir := range dirs {
		_, err = os.Stat(dir)
		if os.IsNotExist(err) {
			err = os.MkdirAll(dir, fileMode)
			if err != nil {
				log.Fatal(err.Error())
			}
		}
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
