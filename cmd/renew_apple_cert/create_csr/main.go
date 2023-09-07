package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"flag"
	"log"
	"os"
	"path"
)

const (
	csrSubDir        = "csr"
	downloadedSubDir = "downloaded"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config/config.json", "Configuration file for create_csr.")
	flag.Parse()

	runJob(configFile)
}

// runJob is an externalized version of main to facilitate testing.
func runJob(configFile string) {
	cfg, err := parseConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	err = createDirs(path.Join(cfg.CertDir, cfg.App))
	if err != nil {
		log.Fatal(err)
	}

	dirCSR := path.Join(cfg.CertDir, cfg.App, csrSubDir)
	key, err := createAndWritePrivateKey(cfg.App, dirCSR)
	if err != nil {
		log.Fatal("createAndWritePrivateKey:", err)
	}

	err = createAndWriteCSR(cfg, key, dirCSR)
	if err != nil {
		log.Fatal("createAndWriteCSR:", err)
	}
}

func createDirs(dir string) error {
	dirs := []string{
		path.Join(dir, csrSubDir),
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

func createAndWritePrivateKey(app, dirCSR string) (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	marshaledKey := x509.MarshalPKCS1PrivateKey(key)
	pemPrivateKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: marshaledKey,
		},
	)
	err = os.WriteFile(path.Join(dirCSR, app+".key"), pemPrivateKey, 0664)
	if err != nil {
		return nil, err
	}
	return key, err
}

func createAndWriteCSR(cfg config, key *rsa.PrivateKey, dirCSR string) error {
	subj := pkix.Name{
		CommonName:   cfg.ApplePushTopic,
		Country:      []string{cfg.Country},
		Province:     []string{cfg.Province},
		Locality:     []string{cfg.Locality},
		Organization: []string{cfg.Organization},
		ExtraNames: []pkix.AttributeTypeAndValue{
			{
				Type: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1},
				Value: asn1.RawValue{
					Tag:   asn1.TagIA5String,
					Bytes: []byte(cfg.Email),
				},
			},
		},
	}
	template := x509.CertificateRequest{
		Subject:            subj,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return err
	}
	cr := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	err = os.WriteFile(path.Join(dirCSR, cfg.App+".csr"), cr, 0664)
	if err != nil {
		return err
	}
	return nil
}
