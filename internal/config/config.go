// SPDX-License-Identifier: MIT

// Package config loads the process configuration (spec "Configuration"): a
// YAML file, overridden by HOURGLASS_* environment variables. Extension
// config blocks stay opaque: each is converted to JSON and handed to its
// factory untouched. The one exception is the type name `reported` and its
// max_age (ReportedMaxAge), which the integration API needs from the config alone.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Extension names one extension and carries its opaque config block.
type Extension struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config,omitempty"` // JSON; nil when the block is absent
}

// UnmarshalYAML reads {type, config}; config is converted to JSON.
func (e *Extension) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: expected a mapping with type and config", n.Line)
	}
	*e = Extension{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "type":
			if err := v.Decode(&e.Type); err != nil {
				return fmt.Errorf("line %d: type: %w", v.Line, err)
			}
		case "config":
			raw, err := nodeToJSON(v)
			if err != nil {
				return fmt.Errorf("line %d: config: %w", v.Line, err)
			}
			if string(raw) != "null" {
				e.Config = raw
			}
		default:
			return fmt.Errorf("line %d: unknown key %q (allowed: type, config)", k.Line, k.Value)
		}
	}
	return nil
}

// OIDC is the OIDC client registration; empty Issuer means OIDC is off.
type OIDC struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

// UI selects bundled mode (Serve true) or split mode (UI-3).
type UI struct {
	Serve bool `yaml:"serve"`
}

// CORS lists the origins allowed for credentialed CORS (UI-4).
type CORS struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// Metrics configures the optional metrics listener; empty Listen is off.
type Metrics struct {
	Listen string `yaml:"listen"`
}

// Config is the typed configuration.
type Config struct {
	Listen        string      `yaml:"listen"`
	Database      string      `yaml:"database"`
	SessionKey    string      `yaml:"session_key"`
	Timezone      string      `yaml:"timezone"`
	AdminPassword string      `yaml:"admin_password"`
	OIDC          OIDC        `yaml:"oidc"`
	Provider      *Extension  `yaml:"provider"` // optional
	Pressure      Extension   `yaml:"pressure"` // empty Type: the caller's default
	Metering      Extension   `yaml:"metering"` // empty Type: the caller's default
	Hooks         []Extension `yaml:"hooks"`
	Seed          string      `yaml:"seed"`
	UI            UI          `yaml:"ui"`
	CORS          CORS        `yaml:"cors"`
	Metrics       Metrics     `yaml:"metrics"`
}

// Defaults returns the built-in defaults.
func Defaults() Config {
	return Config{
		Listen:   ":8080",
		Database: "hourglass.db",
		Timezone: "UTC",
		UI:       UI{Serve: true},
	}
}

// Load reads path (empty means no file: defaults and environment only) and
// applies the environment from getenv (os.Getenv when nil). A named file that
// cannot be read is an error; so is an unknown key anywhere in it.
func Load(path string, getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	c := Defaults()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
		if err := decodeYAML(data, &c); err != nil {
			return Config{}, fmt.Errorf("config: %s: %w", path, err)
		}
	}
	if err := applyEnv(&c, getenv); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func decodeYAML(data []byte, c *Config) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) { // io.EOF: empty file
		return err
	}
	return nil
}

func applyEnv(c *Config, getenv func(string) string) error {
	str := func(name string, dst *string) {
		if v, ok := lookup(getenv, name); ok {
			*dst = v
		}
	}
	str("HOURGLASS_LISTEN", &c.Listen)
	str("HOURGLASS_DATABASE", &c.Database)
	str("HOURGLASS_SESSION_KEY", &c.SessionKey)
	str("HOURGLASS_TIMEZONE", &c.Timezone)
	str("HOURGLASS_ADMIN_PASSWORD", &c.AdminPassword)
	str("HOURGLASS_OIDC_ISSUER", &c.OIDC.Issuer)
	str("HOURGLASS_OIDC_CLIENT_ID", &c.OIDC.ClientID)
	str("HOURGLASS_OIDC_CLIENT_SECRET", &c.OIDC.ClientSecret)
	str("HOURGLASS_OIDC_REDIRECT_URL", &c.OIDC.RedirectURL)
	str("HOURGLASS_METRICS_LISTEN", &c.Metrics.Listen)
	if v, ok := lookup(getenv, "HOURGLASS_UI_SERVE"); ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("config: HOURGLASS_UI_SERVE: %q is not a boolean", v)
		}
		c.UI.Serve = b
	}
	if v, ok := lookup(getenv, "HOURGLASS_CORS_ALLOWED_ORIGINS"); ok {
		c.CORS.AllowedOrigins = splitList(v)
	}
	return nil
}

// lookup treats an empty variable as unset, so an empty value in a compose
// file does not blank a setting from the file.
func lookup(getenv func(string) string, name string) (string, bool) {
	v := getenv(name)
	return v, v != ""
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Validate checks values that do not depend on the extension registry.
func (c Config) Validate() error {
	if c.Listen == "" {
		return errors.New("config: listen must not be empty")
	}
	if c.Database == "" {
		return errors.New("config: database must not be empty")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("config: timezone %q: %w", c.Timezone, err)
	}
	if err := checkListenAddr("listen", c.Listen); err != nil {
		return err
	}
	if c.Metrics.Listen != "" {
		if err := checkListenAddr("metrics.listen", c.Metrics.Listen); err != nil {
			return err
		}
	}
	// An unset key is generated at start (sessions then do not survive a restart); a set one must be strong.
	if c.SessionKey != "" && len(c.SessionKey) < MinSessionKeyBytes {
		return fmt.Errorf("config: session_key must be at least %d bytes (it is %d)", MinSessionKeyBytes, len(c.SessionKey))
	}
	if err := c.OIDC.validate(); err != nil {
		return err
	}
	for i, o := range c.CORS.AllowedOrigins {
		if err := checkOrigin(o); err != nil {
			return fmt.Errorf("config: cors.allowed_origins[%d]: %w", i, err)
		}
	}
	if c.Metrics.Listen != "" && c.Metrics.Listen == c.Listen {
		return errors.New("config: metrics.listen must differ from listen: metrics are never served on the public port")
	}
	for i, h := range c.Hooks {
		if h.Type == "" {
			return fmt.Errorf("config: hooks[%d]: type is required", i)
		}
	}
	if c.Provider != nil && c.Provider.Type == "" && len(c.Provider.Config) > 0 {
		return errors.New("config: provider has a config block but no type")
	}
	if _, _, err := ReportedMaxAge(c.Provider); err != nil {
		return fmt.Errorf("config: provider: %w", err)
	}
	if _, _, err := ReportedMaxAge(&c.Pressure); err != nil {
		return fmt.Errorf("config: pressure: %w", err)
	}
	return nil
}

// ReportedType is the type name of the bundled extensions whose data the
// integration API stores (docs/design/integration-api.md). The core only
// compares the name; it never imports the extension package.
const ReportedType = "reported"

// Bounds of the reported extensions' max_age setting.
const (
	DefaultReportedMaxAge = 10 * time.Minute
	MinReportedMaxAge     = time.Minute
)

// ReportedMaxAge reports whether the extension block e selects the reported
// type and, if so, its max_age (default 10m, minimum 1m). A nil block, or any
// other type, is not selected. The API uses it to answer not_configured and to
// compute expires_at without knowing the extension's code.
func ReportedMaxAge(e *Extension) (maxAge time.Duration, selected bool, err error) {
	if e == nil || e.Type != ReportedType {
		return 0, false, nil
	}
	maxAge = DefaultReportedMaxAge
	if len(e.Config) > 0 {
		var blk struct {
			MaxAge *string `json:"max_age"`
		}
		if err := json.Unmarshal(e.Config, &blk); err != nil {
			return 0, false, fmt.Errorf("config: %v", err)
		}
		if blk.MaxAge != nil {
			d, err := time.ParseDuration(*blk.MaxAge)
			if err != nil {
				return 0, false, fmt.Errorf("max_age %q is not a duration (for example 10m)", *blk.MaxAge)
			}
			maxAge = d
		}
	}
	if maxAge < MinReportedMaxAge {
		return 0, false, fmt.Errorf("max_age must be at least %s", MinReportedMaxAge)
	}
	return maxAge, true, nil
}

// MinSessionKeyBytes is the shortest session key accepted; internal/auth
// enforces the same bound.
const MinSessionKeyBytes = 32

// checkListenAddr accepts host:port with a numeric port 0..65535 (the host may be empty).
func checkListenAddr(name, addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("config: %s %q: %v (want host:port, for example :8080)", name, addr, err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("config: %s %q: port must be a number from 0 to 65535", name, addr)
	}
	return nil
}

// validate checks the shape of the OIDC block: unset, or an issuer with a
// client id and redirect URL, the URLs being absolute http(s) URLs.
func (o OIDC) validate() error {
	if o.Issuer == "" {
		if o.ClientID != "" || o.ClientSecret != "" || o.RedirectURL != "" {
			return errors.New("config: oidc.client_id, client_secret or redirect_url is set but oidc.issuer is not")
		}
		return nil
	}
	if o.ClientID == "" || o.RedirectURL == "" {
		return errors.New("config: oidc.issuer is set, so oidc.client_id and oidc.redirect_url are required")
	}
	for name, v := range map[string]string{"oidc.issuer": o.Issuer, "oidc.redirect_url": o.RedirectURL} {
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("config: %s must be an absolute http or https URL", name)
		}
	}
	return nil
}

// checkOrigin accepts only scheme://host[:port] (a trailing slash is
// tolerated): never "null", "*", a path, a query or credentials.
func checkOrigin(o string) error {
	u, err := url.Parse(o)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || strings.Contains(o, "*") {
		return fmt.Errorf("%q is not an origin (want scheme://host[:port], for example https://hourglass.example.com)", o)
	}
	return nil
}

// Location returns the configured time zone.
func (c Config) Location() (*time.Location, error) { return time.LoadLocation(c.Timezone) }

// HasProvider reports whether a resource provider is configured.
func (c Config) HasProvider() bool { return c.Provider != nil && c.Provider.Type != "" }

const redacted = "[redacted]"

// Redacted returns a view safe to log: secrets and extension config blocks
// (which may hold secrets) are replaced by "[redacted]" when set.
func (c Config) Redacted() map[string]any {
	mask := func(s string) string {
		if s == "" {
			return ""
		}
		return redacted
	}
	ext := func(e Extension) map[string]any {
		m := map[string]any{"type": e.Type}
		if len(e.Config) > 0 {
			m["config"] = redacted
		}
		return m
	}
	hooks := make([]map[string]any, 0, len(c.Hooks))
	for _, h := range c.Hooks {
		hooks = append(hooks, ext(h))
	}
	out := map[string]any{
		"listen":         c.Listen,
		"database":       c.Database,
		"session_key":    mask(c.SessionKey),
		"timezone":       c.Timezone,
		"admin_password": mask(c.AdminPassword),
		"oidc": map[string]any{
			"issuer":        c.OIDC.Issuer,
			"client_id":     c.OIDC.ClientID,
			"client_secret": mask(c.OIDC.ClientSecret),
			"redirect_url":  c.OIDC.RedirectURL,
		},
		"pressure": ext(c.Pressure),
		"metering": ext(c.Metering),
		"hooks":    hooks,
		"seed":     c.Seed,
		"ui":       map[string]any{"serve": c.UI.Serve},
		"cors":     map[string]any{"allowed_origins": append([]string{}, c.CORS.AllowedOrigins...)},
		"metrics":  map[string]any{"listen": c.Metrics.Listen},
	}
	if c.Provider != nil {
		out["provider"] = ext(*c.Provider)
	}
	return out
}

// YAMLToJSON converts a YAML (or JSON) document to JSON. Mapping keys must
// be strings. Used for extension config blocks and for the seed file.
func YAMLToJSON(data []byte) (json.RawMessage, error) {
	var n yaml.Node
	if err := yaml.Unmarshal(data, &n); err != nil {
		return nil, err
	}
	if n.Kind == 0 {
		return json.RawMessage("null"), nil
	}
	return nodeToJSON(&n)
}

func nodeToJSON(n *yaml.Node) (json.RawMessage, error) {
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, err
	}
	v, err := jsonable(v)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// jsonable rewrites the generic decoding of YAML into values json.Marshal
// accepts: maps with non-string keys are rejected, timestamps become strings.
func jsonable(v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			r, err := jsonable(e)
			if err != nil {
				return nil, err
			}
			x[k] = r
		}
		return x, nil
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("mapping key %v is not a string", k)
			}
			r, err := jsonable(e)
			if err != nil {
				return nil, err
			}
			m[ks] = r
		}
		return m, nil
	case []any:
		for i, e := range x {
			r, err := jsonable(e)
			if err != nil {
				return nil, err
			}
			x[i] = r
		}
		return x, nil
	case time.Time:
		return x.Format(time.RFC3339Nano), nil
	default:
		return v, nil
	}
}
