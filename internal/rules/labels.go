// SPDX-License-Identifier: MIT

// Package rules implements label validation, the label-selector parser and
// ordered rule evaluation. It is pure and has no dependencies outside the
// standard library.
package rules

import (
	"fmt"
	"sort"
)

// Limits from SET-3.
const (
	MaxLabels      = 32
	MaxValueLength = 63
	maxKeyLength   = 63
)

func isKeyChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '/' || c == '-'
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// validKey reports whether k matches [a-z0-9]([a-z0-9._/-]{0,61}[a-z0-9])?.
func validKey(k string) bool {
	n := len(k)
	if n == 0 || n > maxKeyLength {
		return false
	}
	if !isAlnum(k[0]) || !isAlnum(k[n-1]) {
		return false
	}
	for i := 0; i < n; i++ {
		if !isKeyChar(k[i]) {
			return false
		}
	}
	return true
}

// ValidateLabels checks labels against SET-3: at most 32 labels, keys match
// the key grammar, values are at most 63 characters (empty allowed, any
// characters).
func ValidateLabels(labels map[string]string) error {
	if len(labels) > MaxLabels {
		return fmt.Errorf("too many labels: %d (at most %d)", len(labels), MaxLabels)
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !validKey(k) {
			return fmt.Errorf("invalid label key %q: must match [a-z0-9]([a-z0-9._/-]{0,61}[a-z0-9])?", k)
		}
		if n := len([]rune(labels[k])); n > MaxValueLength {
			return fmt.Errorf("label %q: value is %d characters (at most %d)", k, n, MaxValueLength)
		}
	}
	return nil
}
