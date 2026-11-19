// SPDX-License-Identifier: MIT

package rules

import (
	"fmt"
	"strings"
)

type op int

const (
	opEq op = iota
	opNotEq
	opIn
	opNotIn
	opExists
	opNotExists
)

type requirement struct {
	key    string
	op     op
	values []string
}

// Selector is a parsed label selector: requirements joined by AND. The zero
// value matches everything.
type Selector struct {
	reqs []requirement
}

// Matches reports whether labels satisfy every requirement (Kubernetes
// semantics: != and notin are true when the key is absent).
func (s Selector) Matches(labels map[string]string) bool {
	for _, r := range s.reqs {
		v, ok := labels[r.key]
		switch r.op {
		case opEq:
			if !ok || v != r.values[0] {
				return false
			}
		case opNotEq:
			if ok && v == r.values[0] {
				return false
			}
		case opIn:
			if !ok || !contains(r.values, v) {
				return false
			}
		case opNotIn:
			if ok && contains(r.values, v) {
				return false
			}
		case opExists:
			if !ok {
				return false
			}
		case opNotExists:
			if ok {
				return false
			}
		}
	}
	return true
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Match parses selector and reports whether labels satisfy it.
func Match(selector string, labels map[string]string) (bool, error) {
	s, err := Parse(selector)
	if err != nil {
		return false, err
	}
	return s.Matches(labels), nil
}

type parser struct {
	in  string
	pos int
}

func (p *parser) errf(pos int, format string, a ...any) error {
	return fmt.Errorf("selector: %s at position %d", fmt.Sprintf(format, a...), pos+1)
}

func (p *parser) skipSpace() {
	for p.pos < len(p.in) && strings.IndexByte(" \t\r\n", p.in[p.pos]) >= 0 {
		p.pos++
	}
}

func (p *parser) eof() bool { return p.pos >= len(p.in) }

func isValueChar(c byte) bool {
	return strings.IndexByte(" \t\r\n,()=!", c) < 0
}

// token reads a run of value characters (a key or a value) without validating.
func (p *parser) token() (string, int) {
	start := p.pos
	for p.pos < len(p.in) && isValueChar(p.in[p.pos]) {
		p.pos++
	}
	return p.in[start:p.pos], start
}

func (p *parser) validValue(v string, at int) error {
	if n := len([]rune(v)); n > MaxValueLength {
		return p.errf(at, "value %q is %d characters (at most %d)", v, n, MaxValueLength)
	}
	return nil
}

func (p *parser) validKeyTok(k string, at int) error {
	if k == "" {
		return p.errf(at, "expected a label key")
	}
	if !validKey(k) {
		return p.errf(at, "invalid label key %q: must match [a-z0-9]([a-z0-9._/-]{0,61}[a-z0-9])?", k)
	}
	return nil
}

// Parse parses a selector. An empty or all-whitespace selector matches
// everything.
func Parse(selector string) (Selector, error) {
	p := &parser{in: selector}
	p.skipSpace()
	if p.eof() {
		return Selector{}, nil
	}
	var sel Selector
	for {
		r, err := p.requirement()
		if err != nil {
			return Selector{}, err
		}
		sel.reqs = append(sel.reqs, r)
		p.skipSpace()
		if p.eof() {
			return sel, nil
		}
		if p.in[p.pos] != ',' {
			return Selector{}, p.errf(p.pos, "unexpected %q, expected ',' or end of selector", p.in[p.pos:p.pos+1])
		}
		p.pos++
	}
}

func (p *parser) requirement() (requirement, error) {
	p.skipSpace()
	if p.eof() {
		return requirement{}, p.errf(p.pos, "expected a requirement, found end of selector")
	}
	if p.in[p.pos] == '!' {
		p.pos++
		if !p.eof() && p.in[p.pos] == '=' {
			return requirement{}, p.errf(p.pos-1, "'!=' needs a key before it")
		}
		p.skipSpace()
		key, at := p.token()
		if err := p.validKeyTok(key, at); err != nil {
			return requirement{}, err
		}
		return requirement{key: key, op: opNotExists}, nil
	}
	key, at := p.token()
	if key == "" {
		return requirement{}, p.errf(p.pos, "expected a label key, found %q", p.in[p.pos:p.pos+1])
	}
	if err := p.validKeyTok(key, at); err != nil {
		return requirement{}, err
	}
	p.skipSpace()
	if p.eof() || p.in[p.pos] == ',' {
		return requirement{key: key, op: opExists}, nil
	}
	switch c := p.in[p.pos]; {
	case c == '=':
		p.pos++
		if !p.eof() && p.in[p.pos] == '=' {
			p.pos++
		}
		return p.single(key, opEq)
	case c == '!':
		if p.pos+1 >= len(p.in) || p.in[p.pos+1] != '=' {
			return requirement{}, p.errf(p.pos, "expected '!=' after key %q", key)
		}
		p.pos += 2
		return p.single(key, opNotEq)
	case c == '(' || c == ')':
		return requirement{}, p.errf(p.pos, "unexpected %q after key %q", string(c), key)
	}
	word, wat := p.token()
	var o op
	switch word {
	case "in":
		o = opIn
	case "notin":
		o = opNotIn
	default:
		return requirement{}, p.errf(wat, "unknown operator %q after key %q (expected =, ==, !=, in, notin)", word, key)
	}
	p.skipSpace()
	if p.eof() || p.in[p.pos] != '(' {
		return requirement{}, p.errf(p.pos, "expected '(' after %q", word)
	}
	p.pos++
	var vals []string
	for {
		p.skipSpace()
		v, vat := p.token()
		if v == "" {
			return requirement{}, p.errf(p.pos, "expected a value in the %s list", word)
		}
		if err := p.validValue(v, vat); err != nil {
			return requirement{}, err
		}
		vals = append(vals, v)
		p.skipSpace()
		if p.eof() {
			return requirement{}, p.errf(p.pos, "missing ')' to close the %s list", word)
		}
		switch p.in[p.pos] {
		case ',':
			p.pos++
		case ')':
			p.pos++
			return requirement{key: key, op: o, values: vals}, nil
		default:
			return requirement{}, p.errf(p.pos, "unexpected %q in the %s list, expected ',' or ')'", p.in[p.pos:p.pos+1], word)
		}
	}
}

func (p *parser) single(key string, o op) (requirement, error) {
	p.skipSpace()
	v, at := p.token()
	if err := p.validValue(v, at); err != nil {
		return requirement{}, err
	}
	if !p.eof() {
		c := p.in[p.pos]
		if c == '(' || c == ')' || c == '=' || c == '!' {
			return requirement{}, p.errf(p.pos, "unexpected %q in value for key %q", string(c), key)
		}
	}
	return requirement{key: key, op: o, values: []string{v}}, nil
}
