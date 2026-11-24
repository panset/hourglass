// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/panset/hourglass/internal/collector"
	"github.com/panset/hourglass/internal/ledger"
	"github.com/panset/hourglass/internal/metering"
	"github.com/panset/hourglass/internal/rules"
	"github.com/panset/hourglass/internal/settings"
	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/internal/weights"
)

// adminState is a snapshot of what the admin views derive from: settings,
// rate rules, setups, active holds, ledger figures, roles and the current
// multiplier. It is loaded once per request; it is not one transaction, which
// is fine for read-only figures.
type adminState struct {
	now         time.Time
	period      string
	cfg         settings.Settings
	rateRules   []rules.Rule
	report      *weights.Report // nil when reported weights are off or before the first report
	setups      map[int64]store.Setup
	setupList   []store.Setup // ordered by name
	holdBySetup map[int64]store.Hold
	holdsByUser map[int64][]store.Hold
	samples     map[int64]store.LastSample // latest usage sample per active hold
	stats       map[int64]store.LedgerStat
	roles       map[int64][]string
	multiplier  float64
	poll        collector.PollView
}

// noLedgerStats as the userID of loadAdminState loads no ledger figures at all
// (callers that read balances from users.balance_mt).
const noLedgerStats int64 = -1

// loadAdminState reads the snapshot. A non-zero userID limits the ledger
// figures to that user (the single-user responses), noLedgerStats skips them,
// and a non-empty status limits them to users with that status, so a filtered
// list computes nothing for the users it does not return.
func (a *API) loadAdminState(ctx context.Context, userID int64, status string) (*adminState, error) {
	st := a.d.Store
	s := &adminState{
		now:         a.d.Clock.Now(),
		setups:      map[int64]store.Setup{},
		holdBySetup: map[int64]store.Hold{},
		holdsByUser: map[int64][]store.Hold{},
		stats:       map[int64]store.LedgerStat{},
	}
	s.period = ledger.PeriodOf(s.now, a.d.Location)
	var err error
	if s.cfg, err = settings.Load(ctx, st); err != nil {
		return nil, err
	}
	if s.rateRules, err = a.loadRateRules(ctx, st); err != nil {
		return nil, err
	}
	if s.report, err = a.weightReport(ctx, s.cfg.ReportedWeights); err != nil {
		return nil, err
	}
	if s.setupList, err = st.ListSetups(ctx, false); err != nil {
		return nil, err
	}
	for _, x := range s.setupList {
		s.setups[x.ID] = x
	}
	hs, err := st.ActiveHolds(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(hs))
	for i, h := range hs {
		s.holdBySetup[h.SetupID] = h
		s.holdsByUser[h.UserID] = append(s.holdsByUser[h.UserID], h)
		ids[i] = h.ID
	}
	if s.samples, err = st.HoldLastSamples(ctx, ids); err != nil {
		return nil, err
	}
	stats, err := st.LedgerStats(ctx, s.period, userID, status)
	if err != nil {
		return nil, err
	}
	for _, x := range stats {
		s.stats[x.UserID] = x
	}
	if s.roles, err = st.AllUserRoles(ctx); err != nil {
		return nil, err
	}
	if a.d.Collector != nil {
		s.poll = a.d.Collector.LastPoll()
	}
	switch {
	case s.poll.OK:
		s.multiplier = s.poll.Multiplier
	default:
		if ps, err := st.LatestPressure(ctx, 1); err != nil {
			return nil, err
		} else if len(ps) > 0 {
			s.multiplier = ps[0].Multiplier
		} else {
			s.multiplier = s.cfg.Bands.Multiplier(0)
		}
	}
	return s, nil
}

// loadRateRules reads the rate rules as the rules package wants them.
func (a *API) loadRateRules(ctx context.Context, q interface {
	ListRules(context.Context, string) ([]store.Rule, error)
}) ([]rules.Rule, error) {
	rows, err := q.ListRules(ctx, rules.KindRate)
	if err != nil {
		return nil, err
	}
	return toRulesRules(rows), nil
}

func toRulesRules(rows []store.Rule) []rules.Rule {
	out := make([]rules.Rule, len(rows))
	for i, r := range rows {
		out[i] = rules.Rule{ID: r.ID, Kind: r.Kind, Position: r.Position, Selector: r.Selector,
			Value: strconv.FormatFloat(r.Value, 'f', -1, 64)}
	}
	return out
}

// resolve is the weight resolver's answer for a setup at the request's time
// (A3). A rate rule that cannot be evaluated falls back to the default
// weight, as the admin views always did: the setup is resolved as if no rate
// rule existed, so an override or a usable reported weight still applies.
func (s *adminState) resolve(x store.Setup) weights.Result {
	return resolveLenient(x, s.rateRules, s.cfg, s.report, s.now)
}

// resolveLenient is the resolver's answer with the fallback above, shared by
// the admin views and the state route: a rate rule that cannot be evaluated
// is treated as if no rate rule existed.
func resolveLenient(x store.Setup, rateRules []rules.Rule, cfg settings.Settings, report *weights.Report, now time.Time) weights.Result {
	res, err := weights.Resolve(resolverSetup(x), rateRules, cfg, report, now)
	if err != nil {
		res, _ = weights.Resolve(resolverSetup(x), nil, cfg, report, now)
	}
	return res
}

func (s *adminState) balance(userID int64) (mt int64, ok bool) {
	st, ok := s.stats[userID]
	return st.BalanceMT, ok && st.Entries > 0
}

// ---- users ----

type activeHoldRef struct {
	HoldID    int64  `json:"hold_id"`
	SetupID   int64  `json:"setup_id"`
	SetupName string `json:"setup_name"`
}

type adminUserView struct {
	ID              int64           `json:"id"`
	Username        string          `json:"username"`
	Email           string          `json:"email"`
	Name            string          `json:"name"`
	AuthSource      string          `json:"auth_source"`
	Status          string          `json:"status"`
	Roles           []string        `json:"roles"`
	IsLocalAdmin    bool            `json:"is_local_admin"`
	BalanceTokens   *float64        `json:"balance_tokens"`
	Overdrawn       bool            `json:"overdrawn"`
	MonthToDateBurn *float64        `json:"month_to_date_burn_tokens"`
	BurnPerHour     float64         `json:"burn_per_hour_tokens"`
	ActiveHolds     []activeHoldRef `json:"active_holds"`
	FirstSignInAt   string          `json:"first_sign_in_at"`
	LastLoginAt     *string         `json:"last_login_at"`
	// Sort keys, not part of the JSON.
	createdAt time.Time
	sortName  string
}

// userView builds the AdminUser of a row. Balance and month-to-date burn are
// null for a user with no role (no allowance; the rollover skips them, LED-3,
// so a figure would be stale). The balance is also null for the local admin,
// whose burn is 0, and for a user whose ledger has never had an entry. A user
// is overdrawn when they hold a role and the balance is zero or below.
func (s *adminState) userView(u store.User) adminUserView {
	roles := s.roles[u.ID]
	if roles == nil {
		roles = []string{}
	}
	v := adminUserView{
		ID: u.ID, Username: u.Username, Email: u.Email, Name: u.Name, AuthSource: u.AuthSource,
		Status: u.Status, Roles: roles, IsLocalAdmin: u.AuthSource == store.AuthLocal,
		ActiveHolds: []activeHoldRef{}, FirstSignInAt: ts(u.CreatedAt), LastLoginAt: tsPtr(u.LastLoginAt),
	}
	if !v.IsLocalAdmin && len(roles) > 0 {
		if mt, ok := s.balance(u.ID); ok {
			b := tokens(mt)
			v.BalanceTokens = &b
			v.Overdrawn = mt <= 0
		}
		burn := tokens(s.stats[u.ID].DebitedMT)
		v.MonthToDateBurn = &burn
	}
	if v.IsLocalAdmin {
		zero := 0.0
		v.MonthToDateBurn = &zero // the local admin is never charged (unchanged)
	}
	var burn int64
	for _, h := range s.holdsByUser[u.ID] {
		x := s.setups[h.SetupID]
		v.ActiveHolds = append(v.ActiveHolds, activeHoldRef{HoldID: h.ID, SetupID: h.SetupID, SetupName: x.Name})
		if h.MeteringStartedAt != nil {
			// The weight the hold shows (A3): a policy's weight when its latest
			// sample has the source policy, else what the setup resolves to now.
			ls, sampled := s.samples[h.ID]
			burn += metering.HourlyCostMT(activeWeight(s.resolve(x), ls, sampled).weight, s.multiplier)
		}
	}
	v.BurnPerHour = tokens(burn)
	v.createdAt = u.CreatedAt
	v.sortName = strings.ToLower(u.Name)
	return v
}

// sortUsers orders a user list: pending requests oldest first; otherwise
// overdrawn users first, then by name, then id.
func sortUsers(list []adminUserView, pendingOnly bool) {
	sort.SliceStable(list, func(i, j int) bool {
		x, y := list[i], list[j]
		if pendingOnly {
			if !x.createdAt.Equal(y.createdAt) {
				return x.createdAt.Before(y.createdAt)
			}
			return x.ID < y.ID
		}
		if x.Overdrawn != y.Overdrawn {
			return x.Overdrawn
		}
		if x.sortName != y.sortName {
			return x.sortName < y.sortName
		}
		return x.ID < y.ID
	})
}

// userViewByID loads one user's AdminUser view.
func (a *API) userViewByID(ctx context.Context, id int64) (adminUserView, error) {
	u, err := a.d.Store.GetUser(ctx, id)
	if err != nil {
		return adminUserView{}, err
	}
	s, err := a.loadAdminState(ctx, id, "")
	if err != nil {
		return adminUserView{}, err
	}
	return s.userView(u), nil
}

// ---- setups ----

type holderView struct {
	UserID        int64   `json:"user_id"`
	HoldID        int64   `json:"hold_id"`
	Name          string  `json:"name"`
	BalanceTokens float64 `json:"balance_tokens"`
	Overdrawn     bool    `json:"overdrawn"`
	IsYou         bool    `json:"is_you"`
}

type meteringView struct {
	Active          bool    `json:"active"`
	StartedAt       *string `json:"started_at"`
	ExpectedStartAt *string `json:"expected_start_at"` // always null (decided: the UI shows the reason text)
	Reason          *string `json:"reason"`
}

type costView struct {
	Now    float64 `json:"now"`
	AtPeak float64 `json:"at_peak"`
}

type setupAdminView struct {
	Enabled             bool    `json:"enabled"`
	LastSeenAt          *string `json:"last_seen_at"`
	MatchedRuleSelector *string `json:"matched_rule_selector"`
	setupWeightAdminJSON
}

type adminSetupView struct {
	ID     int64             `json:"id"`
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Weight float64           `json:"weight"`
	// WeightSource and WeightAttributes are the resolver's (A3, C2).
	WeightSource     string            `json:"weight_source"`
	WeightAttributes map[string]string `json:"weight_attributes"`
	State            string            `json:"state"`
	Holder           *holderView       `json:"holder"`
	HeldSince        *string           `json:"held_since"`
	Deployed         bool              `json:"deployed"`
	Metering         *meteringView     `json:"metering"`
	Cost             costView          `json:"cost_per_hour_tokens"`
	Details          json.RawMessage   `json:"details"`
	Admin            *setupAdminView   `json:"admin"`
}

// setupView builds the admin view of a setup. callerID marks "is_you".
func (a *API) setupView(ctx context.Context, s *adminState, x store.Setup, callerID int64) (adminSetupView, error) {
	res := s.resolve(x)
	w := res.Weight
	labels := labelsNonNil(x.Labels)
	v := adminSetupView{
		ID: x.ID, Name: x.Name, Labels: labels, Weight: w, WeightSource: string(res.Source), WeightAttributes: res.Attributes,
		State:   "free",
		Cost:    costView{Now: tokens(metering.HourlyCostMT(w, s.multiplier)), AtPeak: tokens(metering.HourlyCostMT(w, s.cfg.Bands.Max()))},
		Details: json.RawMessage(x.DetailsJSON),
		Admin: &setupAdminView{Enabled: x.Enabled, MatchedRuleSelector: nullIfEmpty(res.RuleSelector),
			setupWeightAdminJSON: setupWeightAdmin(res, x)},
	}
	if s.poll.OK {
		v.Deployed = s.poll.Reported(x.Name) // D40: reported under the setup's exact name
	}
	if t, ok := s.poll.LastSeen[x.Name]; ok {
		v.Admin.LastSeenAt = nullIfEmpty(ts(t))
	}
	if !x.Enabled {
		v.State = "disabled"
	}
	h, held := s.holdBySetup[x.ID]
	if !held {
		return v, nil
	}
	if x.Enabled {
		v.State = "held"
	}
	hu, err := a.d.Store.GetUser(ctx, h.UserID)
	if err != nil {
		return v, err
	}
	bal := hu.BalanceMT // the users.balance_mt cache (LED-2)
	v.Holder = &holderView{UserID: h.UserID, HoldID: h.ID, Name: hu.Name, BalanceTokens: tokens(bal),
		Overdrawn: bal <= 0, IsYou: h.UserID == callerID}
	v.HeldSince = nullIfEmpty(ts(h.AcquiredAt))
	var reason string
	if dec, ok := s.poll.Decisions[h.ID]; ok {
		reason = dec.Reason // the policy's own words from the latest poll that saw the hold
	}
	v.Metering = &meteringView{Active: h.MeteringStartedAt != nil, StartedAt: tsPtr(h.MeteringStartedAt), Reason: nullIfEmpty(reason)}
	return v, nil
}

// detailsSentinel marks where a setup's details go in a marshalled response.
// It is random per process, so no stored string can forge it.
var detailsSentinel = func() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}()

// writeSetupJSON writes an adminSetupView with `details` copied byte for byte
// from storage. encoding/json would compact RawMessage output and escape
// <, > and & in it, which breaks SET-2 ("stored and returned unmodified").
func writeSetupJSON(w http.ResponseWriter, status int, v adminSetupView) error {
	raw := v.Details
	v.Details = json.RawMessage(strconv.Quote(detailsSentinel))
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}
	needle := []byte(`"details":"` + detailsSentinel + `"`)
	if !bytes.Contains(b, needle) {
		return errors.New("api: details placeholder missing")
	}
	b = bytes.Replace(b, needle, append([]byte(`"details":`), raw...), 1)
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(append(b, '\n'))
	return nil
}

// pollView is the collector's last successful poll; the zero value (OK
// false) when no collector is wired.
func (a *API) pollView() collector.PollView {
	if a.d.Collector == nil {
		return collector.PollView{}
	}
	return a.d.Collector.LastPoll()
}
