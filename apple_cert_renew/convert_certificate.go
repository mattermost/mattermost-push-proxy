package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
)

type inputConv struct {
	app string
}

func newInputConv(app string) (i *inputConv) {
	i = &inputConv{
		app: app,
	}
	return i
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dirCsr := os.Getenv("DIR_CSR")
	dirDownloaded := os.Getenv("DIR_DOWNLOADED")
	dirConverted := os.Getenv("DIR_CONVERTED")
	dirs := []string{
		dirCsr,
		dirDownloaded,
		dirConverted,
	}
	for _, dir := range dirs {
		_, err = os.Stat(dir)
		if os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0644)
			if err != nil {
				panic(err)
			}
		}
	}

	i := newInputConv(
		os.Getenv("APP"),
	)
	_, lookErr := exec.LookPath("openssl")
	if lookErr != nil {
		panic(lookErr)
	}

	err = convertCerToPem(dirDownloaded, dirConverted)
	if err != nil {
		panic(err)
	}

	err = convertPemToP12(dirCsr, dirConverted, i.app)
	if err != nil {
		panic(err)
	}

	err = extractPrivateKey(dirConverted, i.app)
	if err != nil {
		panic(err)
	}

	err = verify(dirConverted, i.app)
	if err != nil {
		panic(err)
	}
}

func convertCerToPem(dirDownloaded string, dirConverted string) (err error) {
	// openssl x509 -inform=der -in=certs/downloaded/aps.cer -outform=pem -out=certs/converted/aps.pem
	cmd := exec.Command("openssl", "x509", "-inform=der", "-in="+dirDownloaded+"/aps.cer", "-outform=pem", "-out="+dirConverted+"/aps.pem")
	err = execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func convertPemToP12(dirCsr string, dirConverted string, app string) (err error) {
	// openssl pkcs12 -export -inkey=certs/csr/classic.key -in=certs/converted/aps.pem -out=certs/converted/aps.p12 -clcerts -passout=pass:
	cmd := exec.Command("openssl", "pkcs12", "-export", "-inkey="+dirCsr+"/"+app+".key", "-in="+dirConverted+"/aps.pem", "-out="+dirConverted+"/aps.p12", "-clcerts", "-passout=pass:")
	err = execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func extractPrivateKey(dirConverted string, app string) (err error) {
	// openssl pkcs12 -in=certs/converted/aps.p12 -out=certs/converted/classic_priv.pem -nodes -clcerts -passin=pass:
	cmd := exec.Command("openssl", "pkcs12", "-in="+dirConverted+"/aps.p12", "-out="+dirConverted+"/"+app+"_priv.pem", "-nodes", "-clcerts", "-passin=pass:")
	err = execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func verify(dirConverted string, app string) (err error) {
	// openssl s_client -connect=gateway.push.apple.com:2195 -cert=certs/converted/aps.pem -key=certs/converted/classic_priv.pem
	cmd := exec.Command("openssl", "s_client", "-connect=gateway.push.apple.com:2195", "-cert="+dirConverted+"/aps.pem", "-key="+dirConverted+"/"+app+"_priv.pem")
	err = execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func execCommand(cmd *exec.Cmd) (err error) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	}
	fmt.Println("Result: " + out.String())
	return nil
}
