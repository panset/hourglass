// SPDX-License-Identifier: MIT

package servetls_test

import (
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/panset/hourglass/internal/servetls"
	"github.com/panset/hourglass/internal/servetls/servetlstest"
)

func TestLoad(t *testing.T) {
	if c, err := servetls.Load("", ""); c != nil || err != nil {
		t.Errorf("no files: %v %v, want nil nil (plain HTTP)", c, err)
	}
	cert, key, _ := servetlstest.WriteSelfSigned(t)
	if _, err := servetls.Load(cert, ""); !errors.Is(err, servetls.ErrOneOfPair) {
		t.Errorf("cert only: %v", err)
	}
	if _, err := servetls.Load("", key); !errors.Is(err, servetls.ErrOneOfPair) {
		t.Errorf("key only: %v", err)
	}
	c, err := servetls.Load(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Certificates) != 1 || c.MinVersion != tls.VersionTLS12 {
		t.Errorf("config: %d certificates, min version %x", len(c.Certificates), c.MinVersion)
	}
	if _, err := servetls.Load(cert, filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Error("a missing key file was accepted")
	}
	// A key that does not belong to the certificate is refused at start.
	_, otherKey, _ := servetlstest.WriteSelfSigned(t)
	if _, err := servetls.Load(cert, otherKey); err == nil {
		t.Error("a mismatched key was accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(bad, []byte("not pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := servetls.Load(bad, key); err == nil {
		t.Error("a file that is not PEM was accepted")
	}
}
