package signatures

import (
	"crypto/x509"
	"fmt"
	"github.com/foxboron/go-uefi/efi"
	"github.com/foxboron/go-uefi/efi/signature"
	"github.com/foxboron/go-uefi/efi/util"
	"github.com/kairos-io/kairos-sdk/types"
)

// GetKeyDatabase returns a single signature.SignatureDatabase for a given type
func GetKeyDatabase(sigType string) (*signature.SignatureDatabase, error) {
	var err error
	var sig *signature.SignatureDatabase

	switch sigType {
	case "PK", "pk":
		sig, err = efi.GetPK()
	case "KEK", "kek":
		sig, err = efi.GetKEK()
	case "DB", "db":
		sig, err = efi.Getdb()
	default:
		return nil, fmt.Errorf("signature type unkown (%s). Valid signature types are PK,KEK,DB", sigType)
	}

	return sig, err
}

// GetAllCerts returns a list of certs in the system
func GetAllCerts() (types.CertList, error) {
	var certList types.CertList
	pk, err := GetKeyDatabase("PK")
	if err != nil {
		return certList, err
	}
	kek, err := GetKeyDatabase("KEK")
	if err != nil {
		return certList, err
	}
	db, err := GetKeyDatabase("DB")
	if err != nil {
		return certList, err
	}

	for _, k := range *pk {
		if isValidSignature(k.SignatureType) {
			for _, k1 := range k.Signatures {
				// Note the S at the end of the function, we are parsing multiple certs, not just one
				certificates, err := x509.ParseCertificates(k1.Data)
				if err != nil {
					continue
				}
				for _, cert := range certificates {
					certList.PK = append(certList.PK, types.CertDetail{Owner: cert.Subject, Issuer: cert.Issuer})
				}
			}
		}
	}

	for _, k := range *kek {
		if isValidSignature(k.SignatureType) {
			for _, k1 := range k.Signatures {
				// Note the S at the end of the function, we are parsing multiple certs, not just one
				certificates, err := x509.ParseCertificates(k1.Data)
				if err != nil {
					continue
				}
				for _, cert := range certificates {
					certList.KEK = append(certList.KEK, types.CertDetail{Owner: cert.Subject, Issuer: cert.Issuer})
				}
			}
		}
	}

	for _, k := range *db {
		if isValidSignature(k.SignatureType) {
			for _, k1 := range k.Signatures {
				// Note the S at the end of the function, we are parsing multiple certs, not just one
				certificates, err := x509.ParseCertificates(k1.Data)
				if err != nil {
					continue
				}
				for _, cert := range certificates {
					certList.DB = append(certList.DB, types.CertDetail{Owner: cert.Subject, Issuer: cert.Issuer})
				}
			}
		}
	}

	return certList, nil

}

// isValidSignature identifies a signature based as a DER-encoded X.509 certificate
func isValidSignature(sign util.EFIGUID) bool {
	return sign == signature.CERT_X509_GUID
}

// GetEfiCertsCommonNames returns a simple list of the Common names of the certs
func GetEfiCertsCommonNames() types.EfiCerts {
	var data types.EfiCerts
	certs, _ := GetAllCerts() // Ignore errors here, we dont care about them
	for _, c := range certs.PK {
		data.PK = append(data.PK, c.Issuer.CommonName)
	}
	for _, c := range certs.KEK {
		data.KEK = append(data.KEK, c.Issuer.CommonName)
	}
	for _, c := range certs.DB {
		data.DB = append(data.DB, c.Issuer.CommonName)
	}
	return data
}
