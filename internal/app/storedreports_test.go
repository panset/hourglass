// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/storedreports"
)

const storedExt = "app-test-stored"

var registerStoredOnce sync.Once

// storedProvider and storedPressure stand in for the bundled "reported"
// extensions, which this package may not import: they read the same window,
// pkg/storedreports, and so fail the same way when the server does not publish
// it. The core publishes the window; the extensions only consume it.
type storedProvider struct{}

func (storedProvider) Name() string { return storedExt }
func (storedProvider) List(ctx context.Context) ([]hourglass.Allocation, error) {
	r := storedreports.Current()
	if r == nil {
		return nil, errNoWindow
	}
	d, err := r.Deployments(ctx)
	if err != nil {
		return nil, err
	}
	var out []hourglass.Allocation
	for _, x := range d.Deployments {
		out = append(out, hourglass.Allocation{ResourceName: x.Setup})
	}
	return out, nil
}

type storedPressure struct{}

func (storedPressure) Name() string { return storedExt }
func (storedPressure) Pressure(ctx context.Context, _ hourglass.Snapshot) (float64, error) {
	r := storedreports.Current()
	if r == nil {
		return 0, errNoWindow
	}
	p, err := r.Pressure(ctx)
	if err != nil {
		return 0, err
	}
	return p.Value, nil
}

type noWindowErr struct{}

func (noWindowErr) Error() string { return "the server publishes no stored reports" }

var errNoWindow = noWindowErr{}

// INT-5 wiring: Run publishes the stored reports to the extensions. With a
// report in the database, a provider and a pressure source that read the window
// succeed on the first poll; if Run stopped publishing it, every poll would
// fail with "the server publishes no stored reports" and no poll would succeed.
func TestINT5_RunPublishesTheStoredReportsToTheExtensions(t *testing.T) {
	registerStoredOnce.Do(func() {
		hourglass.RegisterProvider(storedExt, func(json.RawMessage) (hourglass.ResourceProvider, error) { return storedProvider{}, nil })
		hourglass.RegisterPressureSource(storedExt, func(json.RawMessage) (hourglass.PressureSource, error) { return storedPressure{}, nil })
	})
	cfg, db := writeConfig(t, "provider: {type: "+storedExt+"}\npressure: {type: "+storedExt+"}\n")
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	rep := reported.New(st, systemClock{})
	if _, err := rep.SubmitDeployments(context.Background(), "svc", nil, []reported.Deployment{{Setup: "lab-01"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := rep.SubmitPressure(context.Background(), "svc", 0.5, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	r := start(t, cfg, db)
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, body := get(t, r.base+"/healthz")
		if !strings.Contains(body, `"last_poll_age_seconds":null`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no successful poll within 15s, so the extensions cannot read the stored reports: %s\nlog:\n%s", body, r.out.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := r.stop(); err != nil {
		t.Fatal(err)
	}
}
