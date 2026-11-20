// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/panset/hourglass/internal/app/runhook"
	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/config"
	"github.com/panset/hourglass/internal/reported"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
	"github.com/panset/hourglass/pkg/storedreports"
)

// CheckOptions configure CheckProvider.
type CheckOptions struct {
	// Args are the arguments after the subcommand name (-config path).
	Args []string
	// DefaultPressureSource names the pressure source used when the config
	// names none, as in hourglass.Options.
	DefaultPressureSource string
	// Stdout receives the report; Stderr receives flag errors. Nil means
	// os.Stdout and os.Stderr.
	Stdout, Stderr io.Writer
}

// checkTimeout bounds each extension call, as the collector does.
const checkTimeout = collector.ExtensionTimeout

// CheckProvider implements `hourglassd check-provider`: it builds the
// configured provider and pressure source, makes one List call and one
// Pressure call, and prints how the allocations line up with the setups and
// what the pressure means. The database, if it exists, is opened read-only;
// nothing is written anywhere. Any error is returned (the caller exits
// non-zero).
func CheckProvider(ctx context.Context, opts CheckOptions) error {
	stdout, stderr := opts.Stdout, opts.Stderr
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	path, err := configPath(runhook.Options{Args: opts.Args}, stderr)
	if err != nil {
		return err
	}
	cfg, err := config.Load(path, os.Getenv)
	if err != nil {
		return err
	}
	ext, err := buildCheckExtensions(cfg, opts.DefaultPressureSource)
	if err != nil {
		return err
	}

	// The reported extensions read the stored reports; the database is
	// opened read-only, as for the setups below.
	var ages []string
	if _, _, sel := reportedSelected(cfg); sel {
		if _, statErr := os.Stat(cfg.Database); statErr == nil {
			rst, err := store.OpenReadOnly(cfg.Database)
			if err != nil {
				return err
			}
			defer rst.Close()
			rd := StoredReader{reported.New(rst, systemClock{})}
			defer storedreports.Publish(rd)()
			ages = reportAges(ctx, cfg, rd)
		} else {
			ages = reportAges(ctx, cfg, nil)
		}
	}

	set, setups, holdNames, source, err := loadSetups(ctx, cfg)
	if err != nil {
		return err
	}
	refs := make([]collector.SetupRef, 0, len(setups))
	var snap hourglass.Snapshot
	snap.Time = time.Now()
	snap.ProviderConfigured = ext.provider != nil
	for _, s := range setups {
		refs = append(refs, collector.SetupRef{Name: s.name, Enabled: s.enabled, Held: holdNames[s.name]})
		if s.enabled {
			snap.Setups = append(snap.Setups, hourglass.SetupState{
				Setup: hourglass.Setup{Name: s.name, Labels: s.labels, Details: s.details},
			})
		}
	}
	for name := range holdNames {
		snap.Holds = append(snap.Holds, hourglass.Hold{SetupName: name})
	}
	sort.Slice(snap.Holds, func(i, j int) bool { return snap.Holds[i].SetupName < snap.Holds[j].SetupName })

	var rec collector.Reconciliation
	if p := ext.provider; p != nil {
		list, err := callWithTimeout(ctx, p.List)
		if err != nil {
			return fmt.Errorf("provider %s: %w", p.Name(), err)
		}
		snap.Allocations = collector.DedupAllocations(list)
		rec = collector.Reconcile(refs, list)
	}
	raw, err := callWithTimeout(ctx, func(ctx context.Context) (float64, error) { return ext.pressure.Pressure(ctx, snap) })
	if err != nil {
		return fmt.Errorf("pressure source %s: %w", ext.pressure.Name(), err)
	}
	if !validPressure(raw) {
		return fmt.Errorf("pressure source %s: value %v is outside 0 to 1", ext.pressure.Name(), raw)
	}

	w := stdout
	fmt.Fprintf(w, "setups: %d (%s)\n", len(refs), source)
	for _, l := range ages {
		fmt.Fprintln(w, l)
	}
	if ext.provider == nil {
		fmt.Fprintln(w, "provider: none configured (no allocations to match)")
	} else {
		fmt.Fprintf(w, "provider: %s\n", ext.provider.Name())
		fmt.Fprintf(w, "matched setups (%d):\n", len(rec.Matched))
		for _, s := range rec.Matched {
			note := ""
			switch {
			case !s.Enabled:
				note = "  [disabled]"
			case s.Held:
				note = "  [held]"
			}
			fmt.Fprintf(w, "  %s%s\n", s.Name, note)
		}
		fmt.Fprintf(w, "enabled setups not reported (%d):\n", len(rec.Missing))
		for _, s := range rec.Missing {
			fmt.Fprintf(w, "  %s\n", s.Name)
		}
		fmt.Fprintf(w, "orphans (%d):\n", len(rec.Orphans))
		for _, o := range rec.Orphans {
			fmt.Fprintf(w, "  %s\n", o)
		}
		fmt.Fprintf(w, "ignored allocations: %d\n", rec.Ignored)
	}
	fmt.Fprintf(w, "pressure source: %s\n", ext.pressure.Name())
	fmt.Fprintf(w, "raw pressure: %.4f  multiplier: %gx (current bands, unsmoothed)\n", raw, set.Bands.Multiplier(raw))
	return nil
}

func validPressure(p float64) bool { return p >= 0 && p <= 1 } // false for NaN

// buildCheckExtensions builds only the provider and the pressure source.
func buildCheckExtensions(cfg config.Config, defPressure string) (extensions, error) {
	var x extensions
	var err error
	if cfg.HasProvider() {
		if x.provider, err = hourglass.NewProvider(cfg.Provider.Type, cfg.Provider.Config); err != nil {
			return x, fmt.Errorf("provider: %w", err)
		}
	}
	pt := cfg.Pressure.Type
	if pt == "" {
		pt = defPressure
	}
	if x.pressure, err = hourglass.NewPressureSource(pt, cfg.Pressure.Config); err != nil {
		return x, fmt.Errorf("pressure: %w", err)
	}
	return x, nil
}

type checkSetup struct {
	name    string
	labels  map[string]string
	details []byte
	enabled bool
}

// loadSetups reads setups, settings and active holds from the database
// (read-only) when the file exists, else from the seed file, else nothing.
func loadSetups(ctx context.Context, cfg config.Config) (settings.Settings, []checkSetup, map[string]bool, string, error) {
	set := settings.Defaults()
	held := map[string]bool{}
	_, statErr := os.Stat(cfg.Database)
	switch {
	case statErr == nil:
		st, err := store.OpenReadOnly(cfg.Database)
		if err != nil {
			return set, nil, nil, "", err
		}
		defer st.Close()
		if set, err = settings.Load(ctx, st); err != nil {
			return set, nil, nil, "", fmt.Errorf("load settings: %w", err)
		}
		rows, err := st.ListSetups(ctx, false)
		if err != nil {
			return set, nil, nil, "", fmt.Errorf("list setups: %w", err)
		}
		names := map[int64]string{}
		out := make([]checkSetup, 0, len(rows))
		for _, r := range rows {
			names[r.ID] = r.Name
			out = append(out, checkSetup{name: r.Name, labels: r.Labels,
				details: []byte(r.DetailsJSON), enabled: r.Enabled})
		}
		holds, err := st.ActiveHolds(ctx)
		if err != nil {
			return set, nil, nil, "", fmt.Errorf("list active holds: %w", err)
		}
		for _, h := range holds {
			held[names[h.SetupID]] = true
		}
		return set, out, held, "database " + cfg.Database, nil
	case !errors.Is(statErr, os.ErrNotExist):
		return set, nil, nil, "", statErr
	case cfg.Seed != "":
		f, err := ParseSeed(cfg.Seed)
		if err != nil {
			return set, nil, nil, "", err
		}
		out := make([]checkSetup, 0, len(f.Setups))
		for _, s := range f.Setups {
			out = append(out, checkSetup{name: s.Name, labels: s.Labels,
				details: s.Details, enabled: s.Enabled == nil || *s.Enabled})
		}
		return set, out, held, "no database yet; seed file " + cfg.Seed, nil
	default:
		return set, nil, held, "no database and no seed file", nil
	}
}

func callWithTimeout[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	return fn(ctx)
}

// reportedSelected reports whether the provider and the pressure source are
// the reported extensions.
func reportedSelected(cfg config.Config) (prov, press, any bool) {
	_, prov, _ = config.ReportedMaxAge(cfg.Provider)
	_, press, _ = config.ReportedMaxAge(&cfg.Pressure)
	return prov, press, prov || press
}

// reportAges describes the age of each stored report in use; rd is nil when
// there is no database yet.
func reportAges(ctx context.Context, cfg config.Config, rd storedreports.Reader) []string {
	prov, press, _ := reportedSelected(cfg)
	var out []string
	if prov {
		out = append(out, "deployment report: "+describeAge(func() (time.Time, time.Duration, error) {
			if rd == nil {
				return time.Time{}, 0, storedreports.ErrNoReport
			}
			d, err := rd.Deployments(ctx)
			return d.ObservedAt, d.Age, err
		}))
	}
	if press {
		out = append(out, "pressure report: "+describeAge(func() (time.Time, time.Duration, error) {
			if rd == nil {
				return time.Time{}, 0, storedreports.ErrNoReport
			}
			p, err := rd.Pressure(ctx)
			return p.ObservedAt, p.Age, err
		}))
	}
	return out
}

func describeAge(get func() (time.Time, time.Duration, error)) string {
	at, age, err := get()
	switch {
	case errors.Is(err, storedreports.ErrNoReport):
		return "none received yet"
	case err != nil:
		return "unreadable: " + err.Error()
	}
	return fmt.Sprintf("age %s (observed_at %s)", age.Round(time.Second), at.UTC().Format(time.RFC3339))
}
