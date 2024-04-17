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
