// SPDX-License-Identifier: MIT

package vsphere

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// PasswordEnv is the environment variable used when the config password is empty.
const PasswordEnv = "HOURGLASS_VSPHERE_PASSWORD"

// DefaultRequestTimeout applies when request_timeout is not set.
const DefaultRequestTimeout = 20 * time.Second

// secret is a string that never prints: not with %v, %+v, %#v, %s, nor in JSON.
type secret string

func (secret) String() string               { return "[redacted]" }
func (secret) GoString() string             { return `"[redacted]"` }
func (secret) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }
func (s secret) reveal() string             { return string(s) }

// config is shared by the provider and the pressure source so a deployment can
// give both the same block (SPEC Configuration table, vsphere rows).
type config struct {
	URL                string
	Username           string
	Password           secret
	InsecureSkipVerify bool
	Datacenter         string
	Clusters           []string
	RequestTimeout     time.Duration
}

// String lists the fields with the password redacted.
func (c config) String() string {
	return fmt.Sprintf("vsphere config{url=%s username=%s password=%s insecure_skip_verify=%t datacenter=%q clusters=%q request_timeout=%s}",
		redactURL(c.URL), c.Username, c.Password, c.InsecureSkipVerify, c.Datacenter, c.Clusters, c.RequestTimeout)
}

// connKey identifies a vCenter session; equal keys share one client.
type connKey struct {
	url, username, password, datacenter string
	insecure                            bool
	timeout                             time.Duration
}

func (c config) key() connKey {
	return connKey{c.URL, c.Username, c.Password.reveal(), c.Datacenter, c.InsecureSkipVerify, c.RequestTimeout}
}

// String keeps the password out of any accidental print of a key.
func (k connKey) String() string { return "vsphere connection " + redactURL(k.url) }

// GoString keeps the password out of %#v.
func (k connKey) GoString() string { return k.String() }

type rawConfig struct {
	URL                string   `json:"url"`
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	InsecureSkipVerify bool     `json:"insecure_skip_verify"`
	Datacenter         string   `json:"datacenter"`
	Clusters           []string `json:"clusters"`
	RequestTimeout     string   `json:"request_timeout"`
}

// parseConfig decodes and validates a config block. Unknown fields are
// rejected. Errors never contain the password.
func parseConfig(kind string, raw json.RawMessage) (config, error) {
	var r rawConfig
	if t := bytes.TrimSpace(raw); len(t) == 0 || string(t) == "null" {
		return config{}, fmt.Errorf("%s: config is required (url, username, password)", kind)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return config{}, fmt.Errorf("%s: invalid config: %w", kind, err)
	}
	if dec.More() {
		return config{}, fmt.Errorf("%s: invalid config: trailing data", kind)
	}
	c := config{
		URL: strings.TrimSpace(r.URL), Username: r.Username, Password: secret(r.Password),
		InsecureSkipVerify: r.InsecureSkipVerify, Datacenter: r.Datacenter,
		RequestTimeout: DefaultRequestTimeout,
	}
	if c.URL == "" {
		return config{}, fmt.Errorf(`%s: config "url" is required`, kind)
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return config{}, fmt.Errorf(`%s: config "url" must be an http(s) URL`, kind)
	}
	if u.User != nil {
		return config{}, fmt.Errorf(`%s: config "url" must not contain credentials; use "username" and "password"`, kind)
	}
	if c.Username == "" {
		return config{}, fmt.Errorf(`%s: config "username" is required`, kind)
	}
	if c.Password == "" {
		c.Password = secret(os.Getenv(PasswordEnv))
	}
	if c.Password == "" {
		return config{}, fmt.Errorf(`%s: config "password" is required (or set %s)`, kind, PasswordEnv)
	}
	if r.RequestTimeout != "" {
		d, err := time.ParseDuration(r.RequestTimeout)
		if err != nil {
			return config{}, fmt.Errorf(`%s: config "request_timeout" is not a duration such as "20s"`, kind)
		}
		if d <= 0 {
			return config{}, fmt.Errorf(`%s: config "request_timeout" must be positive`, kind)
		}
		c.RequestTimeout = d
	}
	seen := map[string]bool{}
	for _, n := range r.Clusters {
		if n == "" {
			return config{}, fmt.Errorf(`%s: config "clusters" has an empty name`, kind)
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		c.Clusters = append(c.Clusters, n)
	}
	return c, nil
}

// redactURL drops any userinfo from a URL string.
func redactURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "[unparseable url]"
	}
	u.User = nil
	return u.String()
}

// inScope reports whether a cluster name is selected by the config.
func (c config) inScope(cluster string) bool {
	if len(c.Clusters) == 0 {
		return true
	}
	for _, n := range c.Clusters {
		if n == cluster {
			return true
		}
	}
	return false
}
