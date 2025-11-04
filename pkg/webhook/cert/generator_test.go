/*
Copyright The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cert

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSelfSignedCertificate(t *testing.T) {
	dnsNames := []string{
		"webhook.default.svc",
		"webhook.default.svc.cluster.local",
	}

	bundle, err := GenerateSelfSignedCertificate(dnsNames)
	require.NoError(t, err)
	require.NotNil(t, bundle)

	// Verify CA certificate
	caBlock, _ := pem.Decode(bundle.CAPEM)
	require.NotNil(t, caBlock)
	assert.Equal(t, "CERTIFICATE", caBlock.Type)

	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	require.NoError(t, err)
	assert.True(t, caCert.IsCA)
	assert.Equal(t, "kthena-webhook-ca", caCert.Subject.CommonName)

	// Verify server certificate
	certBlock, _ := pem.Decode(bundle.CertPEM)
	require.NotNil(t, certBlock)
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	serverCert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)
	assert.False(t, serverCert.IsCA)
	assert.Equal(t, dnsNames, serverCert.DNSNames)
	assert.Equal(t, dnsNames[0], serverCert.Subject.CommonName)

	// Verify server key
	keyBlock, _ := pem.Decode(bundle.KeyPEM)
	require.NotNil(t, keyBlock)
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

	_, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	require.NoError(t, err)

	// Verify that server certificate can be verified by CA
	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	opts := x509.VerifyOptions{
		DNSName: dnsNames[0],
		Roots:   roots,
	}

	_, err = serverCert.Verify(opts)
	require.NoError(t, err)
}

func TestGenerateSelfSignedCertificate_SingleDNSName(t *testing.T) {
	dnsNames := []string{"webhook.default.svc"}

	bundle, err := GenerateSelfSignedCertificate(dnsNames)
	require.NoError(t, err)
	require.NotNil(t, bundle)

	certBlock, _ := pem.Decode(bundle.CertPEM)
	serverCert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)

	assert.Equal(t, dnsNames, serverCert.DNSNames)
}

func TestGenerateSelfSignedCertificate_MultipleDNSNames(t *testing.T) {
	dnsNames := []string{
		"webhook1.default.svc",
		"webhook2.default.svc",
		"webhook3.default.svc.cluster.local",
	}

	bundle, err := GenerateSelfSignedCertificate(dnsNames)
	require.NoError(t, err)
	require.NotNil(t, bundle)

	certBlock, _ := pem.Decode(bundle.CertPEM)
	serverCert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)

	assert.Equal(t, dnsNames, serverCert.DNSNames)
	assert.Equal(t, dnsNames[0], serverCert.Subject.CommonName)
}

func TestGenerateSelfSignedCertificate_EmptyDNSNames(t *testing.T) {
	dnsNames := []string{}

	bundle, err := GenerateSelfSignedCertificate(dnsNames)
	require.Error(t, err)
	require.Nil(t, bundle)
	assert.Contains(t, err.Error(), "dnsNames cannot be empty")
}
