package creation

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
	"path"

	"github.com/joho/godotenv"
)

const fileMode = 0700

type input struct {
	app            string
	applePushTopic string
	country        string
	province       string
	locality       string
	organization   string
	email          string
}

func newInput(app string, applePushTopic string, country string, province string, locality string, organization string, email string) *input {
	return &input{
		app:            app,
		applePushTopic: applePushTopic,
		country:        country,
		province:       province,
		locality:       locality,
		organization:   organization,
		email:          email,
	}
}

func init() {
	err := godotenv.Load()
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
}

func Creation() {
	i := newInput(
		os.Getenv("APP"),
		os.Getenv("APPLE_PUSH_TOPIC"),
		os.Getenv("COUNTRY"),
		os.Getenv("PROVINCE"),
		os.Getenv("LOCALITY"),
		os.Getenv("ORGANIZATION"),
		os.Getenv("EMAIL"),
	)
	dirCSR := os.Getenv("DIR_CSR")

	key, err := createAndWritePrivateKey(i.app, dirCSR)
	if err != nil {
		log.Fatal(err.Error())
	}

	err = createAndWriteCSR(i, key, dirCSR)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func createAndWritePrivateKey(app string, dirCSR string) (*rsa.PrivateKey, error) {
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
	err = ioutil.WriteFile(path.Join(dirCSR, app+".key"), pemPrivateKey, fileMode)
	if err != nil {
		return nil, err
	}
	return key, err
}

func createAndWriteCSR(i *input, key *rsa.PrivateKey, dirCSR string) error {
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
	err = ioutil.WriteFile(path.Join(dirCSR, i.app+".csr"), cr, fileMode)
	if err != nil {
		return err
	}
	return nil
}
