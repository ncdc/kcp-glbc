package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/kuadrant/kcp-glbc/pkg/util/env"
)

const (
	caSecretName string = "glbc-ca"
)

type Issuer interface {
	GetSecret() (*corev1.Secret, error)
	GetIssuer() *certman.Issuer
	GetTLSProvider() string
}

func newIssuer(tlsProvider, awsRegion string) Issuer {
	var issuer Issuer
	switch tlsProvider {
	case CertProviderCA:
		issuer = NewCAIssuer()
	case CertProviderLEStaging, CertProviderLEProd:
		issuer = NewLetsEncryptIssuer(tlsProvider, awsRegion)
	default:
		log.Fatalln(fmt.Errorf("unsupported TLS certificate issuer: %s", issuer.GetTLSProvider()))
	}
	return issuer
}

func main() {
	var outputFile string
	var tlsProvider = ""
	var awsRegion = ""

	flag.StringVar(&outputFile, "output-file", "./config/default/issuer.yaml", "Where to output the files")
	flag.StringVar(&tlsProvider, "glbc-tls-provider", env.GetEnvString("GLBC_TLS_PROVIDER", "glbc-ca"), "The TLS certificate issuer, one of [glbc-ca, le-staging, le-production]")
	flag.StringVar(&awsRegion, "region", env.GetEnvString("AWS_REGION", "eu-central-1"), "the region we should target with AWS clients")
	flag.Parse()

	printer := &printers.YAMLPrinter{}

	//Create file and destination of file
	issuerFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatalln(fmt.Errorf("error creating issuer file : %v", err))
	}
	defer issuerFile.Close()

	issuerObject := newIssuer(tlsProvider, awsRegion)
	issuerSecret, err := issuerObject.GetSecret()
	if err != nil {
		log.Fatalln(fmt.Errorf("failed to create issuer yaml file for : %s %w", tlsProvider, err))
	}
	issuerSecret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	issuer := issuerObject.GetIssuer()
	issuer.SetGroupVersionKind(certman.SchemeGroupVersion.WithKind("Issuer"))

	if err := printer.PrintObj(issuer, issuerFile); err != nil {
		log.Fatalln(fmt.Errorf("failed to generate issuer yaml : %s %w", tlsProvider, err))
	}
	if err := printer.PrintObj(issuerSecret, issuerFile); err != nil {
		log.Fatalln(fmt.Errorf("failed to generate issuer secret yaml : %s %w", tlsProvider, err))
	}

	log.Printf("Issuer %s yaml file successfully generated in %s directory ", tlsProvider, outputFile)
}
