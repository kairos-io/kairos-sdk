package types

import "crypto/x509/pkix"

// CertList provides a list of certs on the system from the Efivars and properly parsed
type CertList struct {
	PK  []CertDetail
	KEK []CertDetail
	DB  []CertDetail
}

type CertDetail struct {
	Owner  pkix.Name
	Issuer pkix.Name
}

// EfiCerts is a simplified version of a CertList which only provides the Common names for the certs
type EfiCerts struct {
	PK  []string
	KEK []string
	DB  []string
}
