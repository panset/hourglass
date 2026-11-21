// SPDX-License-Identifier: MIT

// Package servetls loads the optional certificate and key that make a
// listener serve HTTPS itself (docs/decisions.md D64). Both hourglassd and
// hourglass-ui use it. With no files the caller serves plain HTTP, as before,
// and TLS is terminated in front.
package servetls

import (
	"crypto/tls"
	"errors"
	"fmt"
)

// ErrOneOfPair is returned when only one of the two files is named.
var ErrOneOfPair = errors.New("the TLS certificate and key must be set together (one is set, the other is empty)")

// Load reads the PEM certificate (the leaf first, then any intermediates) and
// its PEM private key. Both paths empty returns nil, nil: plain HTTP. The
// files are read once; a renewed certificate needs a restart.
func Load(certFile, keyFile string) (*tls.Config, error) {
	if certFile == "" && keyFile == "" {
		return nil, nil
	}
	if certFile == "" || keyFile == "" {
		return nil, ErrOneOfPair
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("tls: certificate %s, key %s: %w", certFile, keyFile, err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
