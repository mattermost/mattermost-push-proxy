package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"io/ioutil"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type inputCSR struct {
	app            string
	applePushTopic string
	country        string
	province       string
	locality       string
	organization   string
	email          string
}

const fileMode = 0600

func newInputCSR(app string, applePushTopic string, country string, province string, locality string, organization string, email string) (i *inputCSR) {
	i = &inputCSR{
		app:            app,
		applePushTopic: applePushTopic,
		country:        country,
		province:       province,
		locality:       locality,
		organization:   organization,
		email:          email,
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
	dirs := []string{
		dirCsr,
		dirDownloaded,
	}
	for _, dir := range dirs {
		_, err = os.Stat(dir)
		if os.IsNotExist(err) {
			err = os.MkdirAll(dir, fileMode)
			if err != nil {
				panic(err)
			}
		}
	}

	i := newInputCSR(
		os.Getenv("APP"),
		os.Getenv("APPLE_PUSH_TOPIC"),
		os.Getenv("COUNTRY"),
		os.Getenv("PROVINCE"),
		os.Getenv("LOCALITY"),
		os.Getenv("ORGANIZATION"),
		os.Getenv("EMAIL"),
	)

	key, err := createAndWritePrivateKey(i.app, dirCsr)
	if err != nil {
		panic(err)
	}

	err = createAndWriteCSR(i, key, dirCsr)
	if err != nil {
		panic(err)
	}
}

func createAndWritePrivateKey(app string, dirCsr string) (key *rsa.PrivateKey, err error) {
	key, err = rsa.GenerateKey(rand.Reader, 2048)
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
	err = ioutil.WriteFile(dirCsr+"/"+app+".key", pemPrivateKey, fileMode)
	if err != nil {
		return nil, err
	}
	return key, err
}

func createAndWriteCSR(i *inputCSR, key *rsa.PrivateKey, dirCsr string) (err error) {
	subj := pkix.Name{
		CommonName:   i.applePushTopic,
		Country:      []string{i.country},
		Province:     []string{i.province},
		Locality:     []string{i.locality},
		Organization: []string{i.organization},

		ExtraNames: []pkix.AttributeTypeAndValue{
			{
				Type: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1},
				Value: asn1.RawValue{
					Tag:   asn1.TagIA5String,
					Bytes: []byte(i.email),
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
	err = ioutil.WriteFile(dirCsr+"/"+i.app+".csr", cr, fileMode)
	if err != nil {
		return err
	}
	return nil
}
