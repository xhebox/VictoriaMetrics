package http2server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

const testResponse = "testH2TLSResp"

func TestHTTP2WithTLS(t *testing.T) {
	addr := "127.0.0.1:10999"
	createHTTP2ServerWithTLS(t, addr)
	defer stopHTTP2ServerWithTLS(t, addr)

	// wait for the http server to start
	time.Sleep(200 * time.Millisecond)

	// setup HTTP/2 client
	client := http.DefaultClient

	tr := http.DefaultTransport.(*http.Transport).Clone()
	// we use self-gen cert so it's not secure and require skipping the check.
	tr.TLSClientConfig.InsecureSkipVerify = true
	tr.TLSClientConfig.NextProtos = []string{"h2"}

	client.Transport = tr

	// verify the test server with TLS
	resp, err := client.Get(fmt.Sprintf("https://%s/", addr))
	if err != nil {
		t.Fatal("get HTTP/2 TLS server failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("incorrect HTTP/2 TLS server status code: got %d; want %d", resp.StatusCode, http.StatusOK)
	}

	if resp.Proto != "HTTP/2.0" {
		t.Fatalf("incorrect HTTP/2 TLS server proto: got %q; want %q", resp.Proto, "HTTP/2.0")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != testResponse {
		t.Fatalf("incorrect HTTP/2 TLS server response body: got %q; want %q", body, testResponse)
	}
}

func createHTTP2ServerWithTLS(t *testing.T, addr string) {
	testHTTPHandler := func(w http.ResponseWriter, r *http.Request) bool {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testResponse))
		return true
	}

	certFile, keyFile := createTLSFiles(t)

	tlsCfg, err := NewTLSConfig(certFile, keyFile, "TLS13", nil)
	if err != nil {
		t.Fatalf("unexpected error when creating TLS config: %v", err)
		return
	}

	Serve(addr, testHTTPHandler, tlsCfg)
}

func stopHTTP2ServerWithTLS(t *testing.T, addr string) {
	if err := Stop([]string{addr}); err != nil {
		t.Fatalf("unexpected error when stopping HTTP2 server: %v", err)
	}
}

func createTLSFiles(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("cannot generate private key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("cannot generate certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("cannot marshal private key: %v", err)
	}

	dir := t.TempDir()
	certFile := filepath.Join(dir, "test_http2_with_tls.crt")
	keyFile := filepath.Join(dir, "test_http2_with_tls.key")
	fs.MustWriteSync(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	fs.MustWriteSync(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certFile, keyFile
}
