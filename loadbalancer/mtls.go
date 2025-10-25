package loadbalancer

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "strconv"
)

// NewMTLSTransportFromEnv builds an mTLS-capable HTTP transport using env vars:
// MTLS_ENABLED=true|false, MTLS_CERT_FILE, MTLS_KEY_FILE, MTLS_CA_FILE, MTLS_INSECURE_SKIP_VERIFY=true|false
func NewMTLSTransportFromEnv() (*http.Transport, error) {
    if os.Getenv("MTLS_ENABLED") != "true" {
        return nil, nil
    }

    certFile := os.Getenv("MTLS_CERT_FILE")
    keyFile := os.Getenv("MTLS_KEY_FILE")
    caFile := os.Getenv("MTLS_CA_FILE")
    insecureSkip, _ := strconv.ParseBool(os.Getenv("MTLS_INSECURE_SKIP_VERIFY"))

    if certFile == "" || keyFile == "" {
        return nil, fmt.Errorf("mTLS enabled but MTLS_CERT_FILE/MTLS_KEY_FILE not provided")
    }

    // Load client cert
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load client certificate: %w", err)
    }

    // Load CA cert pool if provided
    var rootCAs *x509.CertPool
    if caFile != "" {
        caCert, err := ioutil.ReadFile(caFile)
        if err != nil {
            return nil, fmt.Errorf("failed to read CA file: %w", err)
        }
        rootCAs = x509.NewCertPool()
        if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
            return nil, fmt.Errorf("failed to append CA certificate")
        }
    }

    tlsCfg := &tls.Config{
        Certificates:       []tls.Certificate{cert},
        InsecureSkipVerify: insecureSkip, // use only for testing
        MinVersion:         tls.VersionTLS12,
    }
    if rootCAs != nil {
        tlsCfg.RootCAs = rootCAs
    }

    return &http.Transport{TLSClientConfig: tlsCfg}, nil
}
