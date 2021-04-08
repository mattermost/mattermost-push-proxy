// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
)

const (
	csrSubDir        = "csr"
	downloadedSubDir = "downloaded"
	convertedSubDir  = "converted"

	// Important files
	apsP12 = "aps.p12"
	apsPem = "aps.pem"
	apsCer = "aps.cer"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config/config.json", "Configuration file for convert_cert.")
	flag.Parse()

	cfg, err := parseConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	err = createDirs(path.Join(cfg.CertDir, cfg.App))
	if err != nil {
		log.Fatal(err.Error())
	}

	dirCSR := path.Join(cfg.CertDir, cfg.App, csrSubDir)
	dirConverted := path.Join(cfg.CertDir, cfg.App, convertedSubDir)
	dirDownloaded := path.Join(cfg.CertDir, cfg.App, downloadedSubDir)

	err = convertCerToPem(dirDownloaded, dirConverted)
	if err != nil {
		log.Fatal("convertCerToPem:", err.Error())
	}

	err = convertPemToP12(dirCSR, dirConverted, cfg.App)
	if err != nil {
		log.Fatal("convertPemToP12:", err.Error())
	}

	err = extractPrivateKey(dirConverted, cfg.App)
	if err != nil {
		log.Fatal("extractPrivateKey:", err.Error())
	}

	err = verify(dirConverted, cfg.App, cfg.AppleGateway)
	if err != nil {
		log.Fatal("verify:", err.Error())
	}
}

func createDirs(dir string) error {
	dirs := []string{
		path.Join(dir, csrSubDir),
		path.Join(dir, convertedSubDir),
		path.Join(dir, downloadedSubDir),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertCerToPem(dirDownloaded, dirConverted string) error {
	// openssl x509 -inform=der -in=certs/mattermost/downloaded/aps.cer -outform=pem -out=certs/mattermost/converted/aps.pem
	cmd := exec.Command("openssl", "x509",
		"-inform=der",
		"-in="+path.Join(dirDownloaded, apsCer),
		"-outform=pem",
		"-out="+path.Join(dirConverted, apsPem),
	)
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func convertPemToP12(dirCSR, dirConverted, app string) error {
	// openssl pkcs12 -export -inkey=certs/mattermost/csr/mattermost.key -in=certs/mattermost/converted/aps.pem -out=certs/mattermost/converted/aps.p12 -clcerts -passout=pass:
	cmd := exec.Command("openssl", "pkcs12",
		"-export",
		"-inkey="+path.Join(dirCSR, app+".key"),
		"-in="+path.Join(dirConverted, apsPem),
		"-out="+path.Join(dirConverted, apsP12),
		"-clcerts",
		"-passout=pass:",
	)
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func extractPrivateKey(dirConverted, app string) error {
	// openssl pkcs12 -in=certs/mattermost/converted/aps.p12 -out=certs/mattermost/mattermost/converted/classic_priv.pem -nodes -clcerts -passin=pass:
	cmd := exec.Command("openssl", "pkcs12",
		"-in="+path.Join(dirConverted, apsP12),
		"-out="+path.Join(dirConverted, app+"_priv.pem"),
		"-nodes",
		"-clcerts",
		"-passin=pass:",
	)
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func verify(dirConverted, app, gateway string) error {
	// openssl s_client -connect=gateway.push.apple.com:2195 -cert=certs/mattermost/mattermost/converted/aps.pem -key=certs/mattermost/mattermost/converted/classic_priv.pem
	cmd := exec.Command("openssl", "s_client",
		"-connect="+gateway,
		"-cert="+path.Join(dirConverted, apsPem),
		"-key="+path.Join(dirConverted, app+"_priv.pem"),
	)
	err := execCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func execCommand(cmd *exec.Cmd) error {
	buf, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(buf), err)
	}
	if len(buf) == 0 {
		return nil
	}
	log.Printf("Result: %s\n", string(buf))
	return nil
}
