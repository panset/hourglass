// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/panset/hourglass/pkg/hourglass"
	_ "github.com/panset/hourglass/providers/vsphere"
)

type rig struct {
	t       *testing.T
	ctl     string
	prov    hourglass.ResourceProvider
	pressSr hourglass.PressureSource
}

func newRig(t *testing.T) *rig {
	t.Helper()
	sim, err := NewSim(Config{SOAPAddr: "127.0.0.1:0", Username: "u1", Password: "p1", Hosts: 3})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Close)
	ts := httptest.NewServer(sim.Handler())
	t.Cleanup(ts.Close)

	cfg, _ := json.Marshal(map[string]any{
		"url": fmt.Sprintf("https://%s/sdk", sim.SOAPURL().Host), "username": "u1", "password": "p1",
		"insecure_skip_verify": true, "request_timeout": "10s",
	})
	prov, err := hourglass.NewProvider("vsphere", cfg)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := hourglass.NewPressureSource("vsphere", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &rig{t: t, ctl: ts.URL, prov: prov, pressSr: ps}
}

func (r *rig) do(method, path string, body any) (int, map[string]any) {
	r.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, r.ctl+path, rd)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (r *rig) listed() map[string]bool {
	r.t.Helper()
	got, err := r.prov.List(context.Background())
	if err != nil {
		r.t.Fatal(err)
	}
	m := map[string]bool{}
	for _, a := range got {
		m[a.ResourceName] = true
	}
	return m
}

func (r *rig) pressure() float64 {
	r.t.Helper()
	p, err := r.pressSr.Pressure(context.Background(), hourglass.Snapshot{})
	if err != nil {
		r.t.Fatal(err)
	}
	return p
}

func near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-3 {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestE2ESim_Healthz(t *testing.T) {
	r := newRig(t)
	if c, _ := r.do("GET", "/healthz", nil); c != 200 {
		t.Fatalf("healthz %d", c)
	}
}

func TestE2ESim_PoolsVisibleToRealProvider(t *testing.T) {
	r := newRig(t)
	if got := r.listed(); len(got) != 0 {
		t.Fatalf("fresh simulator lists %v", got)
	}
	if c, b := r.do("POST", "/pools", map[string]string{"name": "lab-01_RP"}); c != 201 || b["created"] != true {
		t.Fatalf("create: %d %v", c, b)
	}
	if c, b := r.do("POST", "/pools", map[string]string{"name": "lab-01_RP"}); c != 200 || b["created"] != false {
		t.Fatalf("idempotent create: %d %v", c, b)
	}
	r.do("POST", "/pools", map[string]string{"name": "lab-02_RP"})
	if got := r.listed(); !got["lab-01_RP"] || !got["lab-02_RP"] || len(got) != 2 {
		t.Fatalf("provider lists %v", got)
	}
	if _, b := r.do("GET", "/pools", nil); fmt.Sprint(b["pools"]) != "[lab-01_RP lab-02_RP]" {
		t.Fatalf("GET /pools %v", b)
	}
	if c, _ := r.do("DELETE", "/pools/lab-01_RP", nil); c != 204 {
		t.Fatalf("delete %d", c)
	}
	if c, _ := r.do("DELETE", "/pools/lab-01_RP", nil); c != 404 {
		t.Fatalf("second delete %d", c)
	}
	if got := r.listed(); got["lab-01_RP"] || !got["lab-02_RP"] {
		t.Fatalf("after delete provider lists %v", got)
	}
}

func TestE2ESim_PressureVisibleToRealSource(t *testing.T) {
	r := newRig(t)
	if p := r.pressure(); p > 0.05 {
		t.Fatalf("baseline pressure %v", p)
	}
	if c, b := r.do("PUT", "/pressure", map[string]float64{"cpu": 0.8, "memory": 0.2, "storage": 0.1}); c != 200 {
		t.Fatalf("put %d %v", c, b)
	}
	for i := 0; i < 3; i++ { // values survive repeated reads
		near(t, r.pressure(), 0.8)
	}
	_, b := r.do("GET", "/pressure", nil)
	near(t, b["cpu"].(float64), 0.8)
	near(t, b["memory"].(float64), 0.2)
	near(t, b["storage"].(float64), 0.1)

	r.do("PUT", "/pressure", map[string]float64{"storage": 0.95}) // omitted fields unchanged
	near(t, r.pressure(), 0.95)
	_, b = r.do("GET", "/pressure", nil)
	near(t, b["cpu"].(float64), 0.8)

	r.do("PUT", "/pressure", map[string]float64{"storage": 0, "memory": 0.99})
	near(t, r.pressure(), 0.99)
	r.do("PUT", "/pressure", map[string]float64{"memory": 0})
	near(t, r.pressure(), 0.8)

	if c, _ := r.do("PUT", "/pressure", map[string]float64{"cpu": 1.5}); c != 400 {
		t.Fatalf("out of range accepted: %d", c)
	}
	near(t, r.pressure(), 0.8)
}
