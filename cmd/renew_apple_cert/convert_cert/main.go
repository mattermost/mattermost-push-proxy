package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/joho/godotenv"
)

const fileMode = 0700

func init() {
	env := os.Getenv("ENV_PUSH_PROXY")
	fmt.Printf("Using environment %v", env)
	switch env {
	case "production":
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal(err.Error())
		}
	case "development":
		err := godotenv.Load(".env.example")
		if err != nil {
			log.Fatal(err.Error())
		}
	default:
		err := godotenv.Load("testdata/.env.testdata")
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	err := createDirs(path.Join(os.Getenv("CERT_DIR"), os.Getenv("APP")))
	if err != nil {
		log.Fatal(err.Error())
	}
}

func createDirs(dir string) error {
	dirs := []string{
		path.Join(dir, "csr"),
		path.Join(dir, "converted"),
		path.Join(dir, "downloaded"),
	}
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			err = os.MkdirAll(dir, fileMode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	app := os.Getenv("APP")

	dirCSR := path.Join(os.Getenv("CERT_DIR"), app, "csr")
	dirDownloaded := path.Join(os.Getenv("CERT_DIR"), app, "downloaded")
	dirConverted := path.Join(os.Getenv("CERT_DIR"), app, "converted")

	_, lookErr := exec.LookPath("openssl")
	if lookErr != nil {
		log.Fatal(lookErr.Error())
	}

	err := convertCerToPem(dirDownloaded, dirConverted)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = convertPemToP12(dirCSR, dirConverted, app)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = extractPrivateKey(dirConverted, app)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = verify(dirConverted, app)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func convertCerToPem(dirDownloaded string, dirConverted string) error {
	// openssl x509 -inform=der -in=certs/downloaded/aps.cer -outform=pem -out=certs/converted/aps.pem
	cmd := exec.Command("openssl", "x509", "-inform=der", "-in="+dirDownloaded+"/aps.cer", "-outform=pem", "-out="+dirConverted+"/aps.pem")
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func convertPemToP12(dirCSR string, dirConverted string, app string) error {
	// openssl pkcs12 -export -inkey=certs/csr/classic.key -in=certs/converted/aps.pem -out=certs/converted/aps.p12 -clcerts -passout=pass:
	cmd := exec.Command("openssl", "pkcs12", "-export", "-inkey="+dirCSR+"/"+app+".key", "-in="+dirConverted+"/aps.pem", "-out="+dirConverted+"/aps.p12", "-clcerts", "-passout=pass:")
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func extractPrivateKey(dirConverted string, app string) error {
	// openssl pkcs12 -in=certs/converted/aps.p12 -out=certs/converted/classic_priv.pem -nodes -clcerts -passin=pass:
	cmd := exec.Command("openssl", "pkcs12", "-in="+dirConverted+"/aps.p12", "-out="+dirConverted+"/"+app+"_priv.pem", "-nodes", "-clcerts", "-passin=pass:")
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func verify(dirConverted string, app string) error {
	// openssl s_client -connect=gateway.push.apple.com:2195 -cert=certs/converted/aps.pem -key=certs/converted/classic_priv.pem
	cmd := exec.Command("openssl", "s_client", "-connect=gateway.push.apple.com:2195", "-cert="+dirConverted+"/aps.pem", "-key="+dirConverted+"/"+app+"_priv.pem")
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func execCommand(cmd *exec.Cmd) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}
	fmt.Println("Result: " + out.String())
	return nil
}
