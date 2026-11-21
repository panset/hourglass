// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func writeFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "hourglass.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCFG_DefaultsWithoutFile(t *testing.T) {
	c, err := Load("", env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != ":8080" || c.Database != "hourglass.db" || c.Timezone != "UTC" || !c.UI.Serve || c.Metrics.Listen != "" || c.Provider != nil {
		t.Fatalf("defaults: %+v", c)
	}
}

func TestCFG_EnvOnlyWithoutFile(t *testing.T) {
	c, err := Load("", env(map[string]string{"HOURGLASS_LISTEN": ":9", "HOURGLASS_UI_SERVE": "false", "HOURGLASS_CORS_ALLOWED_ORIGINS": " http://a.test , http://b.test,"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != ":9" || c.UI.Serve || len(c.CORS.AllowedOrigins) != 2 || c.CORS.AllowedOrigins[1] != "http://b.test" {
		t.Fatalf("%+v", c)
	}
}

func TestCFG_FileThenEnvOverride(t *testing.T) {
	p := writeFile(t, `
listen: ":1111"
database: /data/a.db
session_key: file-key-0123456789-0123456789-0123456789
timezone: Europe/Berlin
oidc: {issuer: https://idp.test, client_id: id, client_secret: file-secret, redirect_url: https://x.test/auth/callback}
ui: {serve: true}
cors: {allowed_origins: [http://file.test]}
metrics: {listen: ":9100"}
seed: seed.yaml
`)
	c, err := Load(p, env(map[string]string{
		"HOURGLASS_LISTEN":               ":2222",
		"HOURGLASS_DATABASE":             "/data/b.db",
		"HOURGLASS_SESSION_KEY":          "env-key-0123456789-0123456789-0123456789",
		"HOURGLASS_TIMEZONE":             "UTC",
		"HOURGLASS_ADMIN_PASSWORD":       "pw",
		"HOURGLASS_OIDC_ISSUER":          "https://other.test",
		"HOURGLASS_OIDC_CLIENT_ID":       "id2",
		"HOURGLASS_OIDC_CLIENT_SECRET":   "env-secret",
		"HOURGLASS_OIDC_REDIRECT_URL":    "https://y.test/auth/callback",
		"HOURGLASS_UI_SERVE":             "0",
		"HOURGLASS_CORS_ALLOWED_ORIGINS": "http://env.test",
		"HOURGLASS_METRICS_LISTEN":       ":9200",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != ":2222" || c.Database != "/data/b.db" || c.SessionKey != "env-key-0123456789-0123456789-0123456789" || c.Timezone != "UTC" ||
		c.AdminPassword != "pw" || c.OIDC != (OIDC{"https://other.test", "id2", "env-secret", "https://y.test/auth/callback"}) ||
		c.UI.Serve || len(c.CORS.AllowedOrigins) != 1 || c.CORS.AllowedOrigins[0] != "http://env.test" ||
		c.Metrics.Listen != ":9200" || c.Seed != "seed.yaml" {
		t.Fatalf("%+v", c)
	}
}

func TestCFG_EmptyEnvKeepsFileValue(t *testing.T) {
	p := writeFile(t, "listen: ':1111'\n")
	c, err := Load(p, env(map[string]string{"HOURGLASS_LISTEN": ""}))
	if err != nil || c.Listen != ":1111" {
		t.Fatalf("%v %+v", err, c)
	}
}

func TestEXT2_ExtensionConfigPassedThroughAsJSON(t *testing.T) {
	p := writeFile(t, `
provider:
  type: static
  config:
    file: /etc/static.yaml
    nested: {a: [1, 2.5, true, null], b: "x"}
pressure: {type: inventory}
metering:
  type: default
  config: {start_after: 2h, start_on_allocation: true}
hooks:
  - type: webhook
    config: {url: "http://h.test", secret: s3cret}
  - type: other
`)
	c, err := Load(p, env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Provider == nil || c.Provider.Type != "static" {
		t.Fatalf("provider %+v", c.Provider)
	}
	var got map[string]any
	if err := json.Unmarshal(c.Provider.Config, &got); err != nil {
		t.Fatalf("not json: %s", c.Provider.Config)
	}
	nested := got["nested"].(map[string]any)
	if got["file"] != "/etc/static.yaml" || nested["b"] != "x" || len(nested["a"].([]any)) != 4 || nested["a"].([]any)[1] != 2.5 {
		t.Fatalf("config %s", c.Provider.Config)
	}
	if c.Pressure.Type != "inventory" || c.Pressure.Config != nil {
		t.Fatalf("pressure %+v", c.Pressure)
	}
	if string(c.Metering.Config) != `{"start_after":"2h","start_on_allocation":true}` {
		t.Fatalf("metering %s", c.Metering.Config)
	}
	if len(c.Hooks) != 2 || c.Hooks[0].Type != "webhook" || c.Hooks[1].Config != nil {
		t.Fatalf("hooks %+v", c.Hooks)
	}
}

func TestCFG_UnknownTopLevelKeyIsError(t *testing.T) {
	_, err := Load(writeFile(t, "listen: ':1'\nlisten_adress: ':2'\n"), env(nil))
	if err == nil || !strings.Contains(err.Error(), "listen_adress") {
		t.Fatalf("err = %v", err)
	}
}

func TestCFG_UnknownKeyInExtensionBlockIsError(t *testing.T) {
	_, err := Load(writeFile(t, "pressure: {type: x, confg: {}}\n"), env(nil))
	if err == nil || !strings.Contains(err.Error(), "confg") {
		t.Fatalf("err = %v", err)
	}
}

func TestCFG_BadValues(t *testing.T) {
	cases := map[string]string{
		"timezone":      "timezone: Mars/Base\n",
		"metrics clash": "listen: ':8080'\nmetrics: {listen: ':8080'}\n",
		"hook no type":  "hooks: [{config: {}}]\n",
		"not a mapping": "- a\n- b\n",
	}
	for name, body := range cases {
		if _, err := Load(writeFile(t, body), env(nil)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := Load("", env(map[string]string{"HOURGLASS_UI_SERVE": "maybe"})); err == nil {
		t.Error("bad HOURGLASS_UI_SERVE accepted")
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml"), env(nil)); err == nil {
		t.Error("missing named file accepted")
	}
}

func TestCFG_EmptyFileIsDefaults(t *testing.T) {
	c, err := Load(writeFile(t, ""), env(nil))
	if err != nil || c.Listen != ":8080" {
		t.Fatalf("%v %+v", err, c)
	}
}

func TestNFR_RedactedNeverContainsSecrets(t *testing.T) {
	p := writeFile(t, `
session_key: SESSIONKEY-VALUE-0123456789-0123456789
admin_password: ADMINPW-VALUE
oidc: {issuer: https://idp.test, client_id: cid, client_secret: OIDCSECRET-VALUE, redirect_url: https://x.test/cb}
provider: {type: static, config: {password: PROVIDERPW-VALUE}}
hooks: [{type: webhook, config: {secret: HOOKSECRET-VALUE}}]
`)
	c, err := Load(p, env(nil))
	if err != nil {
		t.Fatal(err)
	}
	out := fmt.Sprintf("%v", c.Redacted())
	b, _ := json.Marshal(c.Redacted())
	out += string(b)
	for _, s := range []string{"SESSIONKEY-VALUE", "ADMINPW-VALUE", "OIDCSECRET-VALUE", "PROVIDERPW-VALUE", "HOOKSECRET-VALUE"} {
		if strings.Contains(out, s) {
			t.Errorf("redacted view leaks %s", s)
		}
	}
	if !strings.Contains(out, "static") || !strings.Contains(out, "https://idp.test") || !strings.Contains(out, "cid") {
		t.Errorf("redacted view lost non-secret fields: %s", out)
	}
	if c.Redacted()["admin_password"] != "[redacted]" {
		t.Errorf("set secret not marked: %v", c.Redacted()["admin_password"])
	}
	if d := Defaults().Redacted(); d["session_key"] != "" {
		t.Errorf("unset secret should stay empty, got %v", d["session_key"])
	}
}

func TestYAMLToJSON(t *testing.T) {
	b, err := YAMLToJSON([]byte("a: 1\nb: [x, y]\n"))
	if err != nil || string(b) != `{"a":1,"b":["x","y"]}` {
		t.Fatalf("%s %v", b, err)
	}
	if b, _ := YAMLToJSON([]byte("")); string(b) != "null" {
		t.Fatalf("empty: %s", b)
	}
	if _, err := YAMLToJSON([]byte("{1: a}")); err == nil {
		t.Fatal("non-string key accepted")
	}
}

// Startup validates the whole configuration before anything is created (task 23b item 5),
// and the CORS origin list holds only origins (task 23 finding 11).
func TestCFG_WholeConfigurationValidatedAtLoad(t *testing.T) {
	cases := map[string]string{
		"short session key":     "session_key: short\n",
		"oidc without client":   "oidc: {issuer: 'https://idp.test'}\n",
		"oidc client no issuer": "oidc: {client_id: cid}\n",
		"oidc bad issuer":       "oidc: {issuer: 'idp.test', client_id: c, redirect_url: 'https://x.test/cb'}\n",
		"oidc bad redirect":     "oidc: {issuer: 'https://idp.test', client_id: c, redirect_url: '/cb'}\n",
		"listen no port":        "listen: localhost\n",
		"listen bad port":       "listen: ':http'\n",
		"metrics bad listen":    "metrics: {listen: 'x:99999'}\n",
		"cors null":             "cors: {allowed_origins: ['null']}\n",
		"cors wildcard":         "cors: {allowed_origins: ['*']}\n",
		"cors wildcard host":    "cors: {allowed_origins: ['https://*.example.com']}\n",
		"cors path":             "cors: {allowed_origins: ['https://a.example.com/x']}\n",
		"cors no scheme":        "cors: {allowed_origins: ['a.example.com']}\n",
	}
	for name, body := range cases {
		_, err := Load(writeFile(t, body), env(nil))
		if err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	ok := "session_key: 0123456789-0123456789-0123456789-01\nlisten: '127.0.0.1:0'\n" +
		"oidc: {issuer: 'https://idp.test', client_id: c, redirect_url: 'https://x.test/cb'}\n" +
		"cors: {allowed_origins: ['http://dev.example:5173', 'https://a.example.com/']}\n"
	if _, err := Load(writeFile(t, ok), env(nil)); err != nil {
		t.Errorf("valid configuration rejected: %v", err)
	}
}

func TestReportedMaxAge(t *testing.T) {
	ext := func(typ, cfg string) *Extension { return &Extension{Type: typ, Config: json.RawMessage(cfg)} }
	for _, c := range []struct {
		name string
		e    *Extension
		want time.Duration
		sel  bool
		bad  bool
	}{
		{"nil", nil, 0, false, false},
		{"other type", ext("other", `{"max_age":"1s"}`), 0, false, false},
		{"default", ext("reported", ``), 10 * time.Minute, true, false},
		{"set", ext("reported", `{"max_age":"30m"}`), 30 * time.Minute, true, false},
		{"minimum", ext("reported", `{"max_age":"1m"}`), time.Minute, true, false},
		{"below minimum", ext("reported", `{"max_age":"59s"}`), 0, false, true},
		{"not a duration", ext("reported", `{"max_age":"soon"}`), 0, false, true},
	} {
		got, sel, err := ReportedMaxAge(c.e)
		if (err != nil) != c.bad || sel != c.sel || got != c.want {
			t.Errorf("%s: got %v %v %v", c.name, got, sel, err)
		}
	}
}

// D64: the certificate and key come from the file or the environment, and
// only as a pair.
func TestCFG_TLSCertAndKeyAreAPair(t *testing.T) {
	if c := Defaults(); c.TLS != (TLS{}) {
		t.Errorf("TLS is on by default: %+v", c.TLS)
	}
	p := writeFile(t, "tls: {cert: /f/cert.pem, key: /f/key.pem}\n")
	c, err := Load(p, env(nil))
	if err != nil || c.TLS != (TLS{Cert: "/f/cert.pem", Key: "/f/key.pem"}) {
		t.Fatalf("file: %v %+v", err, c.TLS)
	}
	c, err = Load(p, env(map[string]string{"HOURGLASS_TLS_CERT": "/e/cert.pem", "HOURGLASS_TLS_KEY": "/e/key.pem"}))
	if err != nil || c.TLS != (TLS{Cert: "/e/cert.pem", Key: "/e/key.pem"}) {
		t.Fatalf("env over file: %v %+v", err, c.TLS)
	}
	if got := c.Redacted()["tls"].(map[string]any); got["cert"] != "/e/cert.pem" || got["key"] != "/e/key.pem" {
		t.Errorf("redacted view: %v", got)
	}
	for _, one := range []map[string]string{{"HOURGLASS_TLS_CERT": "/e/cert.pem"}, {"HOURGLASS_TLS_KEY": "/e/key.pem"}} {
		if _, err := Load("", env(one)); err == nil || !strings.Contains(err.Error(), "set together") {
			t.Errorf("%v alone: %v", one, err)
		}
	}
}
