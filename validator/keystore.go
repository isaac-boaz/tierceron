package validator

import (
	"bufio"
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	eUtils "tierceron/utils"
	"time"

	"github.com/pavlo-v-chernykh/keystore-go/v4"

	"github.com/youmark/pkcs8"
	"golang.org/x/crypto/pkcs12"
	pkcs "golang.org/x/crypto/pkcs12"
)

// Copied from pkcs12.go... why can't they just make these public.  Gr...
// PEM block types
const (
	certificateType = "CERTIFICATE"
	privateKeyType  = "PRIVATE KEY"
)

func StoreKeystore(config *eUtils.DriverConfig, trustStorePassword string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	keystoreWriter := bufio.NewWriter(buffer)

	if config.KeyStore == nil {
		return nil, errors.New("Cert bundle not properly named")
	}
	config.KeyStore.Store(keystoreWriter, []byte(trustStorePassword))
	keystoreWriter.Flush()

	return buffer.Bytes(), nil
}

func AddToKeystore(config *eUtils.DriverConfig, alias string, password []byte, certBundleJks string, data []byte) error {
	// TODO: Add support for this format?  golang.org/x/crypto/pkcs12

	if !strings.HasSuffix(config.WantKeystore, ".jks") && strings.HasSuffix(certBundleJks, ".jks") {
		config.WantKeystore = certBundleJks
	}

	if config.KeyStore == nil {
		fmt.Println("Making new keystore.")
		ks := keystore.New()
		config.KeyStore = &ks
	}

	block, _ := pem.Decode(data)
	if block == nil {
		key, cert, err := pkcs12.Decode(data, string(password)) // Note the order of the return values.
		if err != nil {
			return err
		}
		pkcs8Key, err := pkcs8.ConvertPrivateKeyToPKCS8(key, password)
		if err != nil {
			return err
		}

		config.KeyStore.SetPrivateKeyEntry(alias, keystore.PrivateKeyEntry{
			CreationTime: time.Now(),
			PrivateKey:   pkcs8Key,
			CertificateChain: []keystore.Certificate{
				{
					Type:    "X509",
					Content: cert.Raw,
				},
			},
		}, password)

	} else {
		config.KeyStore.SetTrustedCertificateEntry(alias, keystore.TrustedCertificateEntry{
			CreationTime: time.Now(),
			Certificate: keystore.Certificate{
				Type:    "X509",
				Content: block.Bytes,
			},
		})
	}

	return nil
}

// ValidateKeyStore validates the sendgrid API key.
func ValidateKeyStore(config *eUtils.DriverConfig, filename string, pass string) (bool, error) {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, err
	}
	pemBlocks, errToPEM := pkcs.ToPEM(file, pass)
	if errToPEM != nil {
		return false, errors.New("failed to parse: " + err.Error())
	}
	isValid := false

	for _, pemBlock := range pemBlocks {
		// PEM constancts defined but not exposed in
		//	certificateType = "CERTIFICATE"
		//	privateKeyType  = "PRIVATE KEY"

		if (*pemBlock).Type == certificateType {
			var cert x509.Certificate
			_, errUnmarshal := asn1.Unmarshal((*pemBlock).Bytes, &cert)
			if errUnmarshal != nil {
				return false, errors.New("failed to parse: " + err.Error())
			}

			isCertValid, err := VerifyCertificate(&cert, "")
			if err != nil {
				eUtils.LogInfo(config, "Certificate validation failure.")
			}
			isValid = isCertValid
		} else if (*pemBlock).Type == privateKeyType {
			var key rsa.PrivateKey
			_, errUnmarshal := asn1.Unmarshal((*pemBlock).Bytes, &key)
			if errUnmarshal != nil {
				return false, errors.New("failed to parse: " + err.Error())
			}

			if err := key.Validate(); err != nil {
				eUtils.LogInfo(config, "key validation didn't work")
			}
		}
	}

	return isValid, err
}
