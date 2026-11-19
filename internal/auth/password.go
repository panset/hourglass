// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// MinPasswordLength is the AUTH-7 minimum, counted in characters (runes).
const MinPasswordLength = 12

// maxPasswordBytes bounds the input to the hash so a huge body cannot make
// hashing expensive; it is far above any sensible password.
const maxPasswordBytes = 1024

// Argon2id parameters (RFC 9106, second recommended option: t=3, m=64 MiB, p=4).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Password errors. Their text never contains the password.
var (
	ErrPasswordTooShort  = fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	ErrPasswordTooLong   = fmt.Errorf("password must be at most %d bytes", maxPasswordBytes)
	ErrPasswordUnchanged = errors.New("new password must differ from the current one")
	errBadHash           = errors.New("malformed password hash")
)

// ValidatePassword applies the AUTH-7 rules to a new password.
func ValidatePassword(pw string) error {
	if len(pw) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	if utf8.RuneCountInString(pw) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// HashPassword returns an argon2id hash in PHC string format:
// $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash> (unpadded standard base64).
func HashPassword(pw string) (string, error) {
	if len(pw) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether pw matches a PHC argon2id hash, comparing in
// constant time. Parameters are read from the hash itself.
func VerifyPassword(encoded, pw string) (bool, error) {
	if len(pw) > maxPasswordBytes {
		return false, nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, errBadHash
	}
	// Bound the parameters so a corrupted row cannot exhaust memory or CPU.
	if m < 8 || m > 1<<20 || t < 1 || t > 16 || p < 1 {
		return false, errBadHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 128 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// generatedAlphabet is plain alphanumerics so a generated password survives
// copy and paste from any log viewer (24 chars give about 143 bits).
const generatedAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// GeneratePassword returns n characters drawn uniformly from crypto/rand.
func GeneratePassword(n int) (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(generatedAlphabet)))
	for range n {
		i, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(generatedAlphabet[i.Int64()])
	}
	return b.String(), nil
}
