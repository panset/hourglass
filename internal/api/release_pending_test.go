// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/panset/hourglass/internal/store"
	"github.com/panset/hourglass/pkg/hourglass"
)

// Tests of HOLD-6 to HOLD-9 over HTTP (docs/design/integration-api.md,
// "Releasing a deployed setup").

func newReleaseEnv(t *testing.T) *uenv {
	t.Helper()
	b := newBackend(t, backendOpts{start: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), startEvents: true,
		runtime: Runtime{PressureSource: "fake-pressure", MeteringPolicy: "fake-policy"}})
	b.policy.SetDefault(hourglass.Decision{Meter: true, Reason: "test"})
	return &uenv{b}
}

func (e *uenv) flushEvents() {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.ev.Flush(ctx); err != nil {
		e.t.Fatal(err)
	}
}

func (e *uenv) count(typ string) int {
	e.flushEvents()
	return len(e.hook.EventsOfType(typ))
}

func TestHOLD6_UserReleaseOfDeployedSetupAnswers202Once(t *testing.T) {
	e := newReleaseEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	hid := e.acquire(&dev, sid)
	path := "/api/v1/holds/" + uItoa(hid) + "/release"
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.poll()

	first := uWantOK(t, e.post(path, &dev, ""), http.StatusAccepted)
	if first["active"] != true || first["released_at"] != nil || first["released_by"] != nil || first["release_requested_at"] == nil {
		t.Fatalf("202 body %v", first)
	}
	e.clk.Advance(time.Minute)
	again := uWantOK(t, e.post(path, &dev, `{"force":false}`), http.StatusAccepted)
	if again["release_requested_at"] != first["release_requested_at"] {
		t.Errorf("release_requested_at changed on a repeat: %v -> %v", first["release_requested_at"], again["release_requested_at"])
	}
	if n := e.count(hourglass.EventHoldReleasing); n != 1 {
		t.Errorf("hold.releasing delivered %d times", n)
	}
	// The user-facing hold JSON exposes the marker (setups list and my holds).
	mine := uWantOK(t, e.get("/api/v1/me/holds?state=active", &dev), 200)["holds"].([]any)[0].(ujobj)
	if mine["release_requested_at"] != first["release_requested_at"] || mine["active"] != true {
		t.Errorf("my holds %v", mine)
	}
	sl := uWantOK(t, e.get("/api/v1/setups", &dev), 200)["setups"].([]any)[0].(ujobj)
	if hd := sl["holder"].(ujobj); hd["release_requested_at"] != first["release_requested_at"] {
		t.Errorf("setups list holder %v", hd)
	}
	// HOLD-7: still active, so the setup stays held.
	other := e.dev("other", store.RoleDeveloper)
	uWantErr(t, e.post("/api/v1/setups/"+uItoa(sid)+"/acquire", &other, ""), 409, "setup_held")
	// A pending release cannot be cancelled: there is no such route (404/405 from the router).
	if rec := e.do(http.MethodDelete, path, &dev, true, ""); rec.Code < 400 {
		t.Errorf("DELETE on a release path answered %d", rec.Code)
	}
}

func TestHOLD7_PendingReleaseCompletesAtThePollWithoutTheSetup(t *testing.T) {
	e := newReleaseEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	hid := e.acquire(&dev, sid)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.poll()
	uWantOK(t, e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, ""), http.StatusAccepted)
	e.step(5 * time.Minute)
	if h := e.getHoldRow(hid); h.ReleasedAt != nil {
		t.Fatal("released while the setup is still reported")
	}
	e.prov.SetAllocations()
	e.step(5 * time.Minute)
	h := e.getHoldRow(hid)
	if h.ReleasedAt == nil || h.ReleasedBy == nil || *h.ReleasedBy != dev.user.ID {
		t.Fatalf("hold %+v", h)
	}
	if n := e.count(hourglass.EventHoldReleased); n != 1 {
		t.Errorf("hold.released delivered %d times", n)
	}
	rel := e.hook.EventsOfType(hourglass.EventHoldReleased)[0]
	if rel.Data["forced"] != false || rel.Data["reason"] != "" {
		t.Errorf("event data %v", rel.Data)
	}
	// The setup is free again.
	if s := uWantOK(t, e.get("/api/v1/setups", &dev), 200)["setups"].([]any)[0].(ujobj); s["state"] != "free" {
		t.Errorf("setup %v", s)
	}
}

func (e *uenv) getHoldRow(id int64) store.Hold {
	e.t.Helper()
	h, err := e.st.GetHold(uBg, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func TestHOLD8_ServiceReleaseOverHTTP(t *testing.T) {
	e := newReleaseEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	hid := e.acquire(&dev, sid)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.poll()
	path := "/api/v1/integration/holds/" + uItoa(hid) + "/release"
	svc := e.mintServiceAccount("reaper", "holds:release")
	noScope := e.mintServiceAccount("viewer", "state:read")

	call := func(tok, p, body string) *httptest.ResponseRecorder { return e.serveService("POST", p, tok, body) }
	if rec := call(noScope.token, path, `{"reason":"x"}`); rec.Code != 403 {
		t.Fatalf("without the scope: %d", rec.Code)
	}
	for _, body := range []string{`{}`, `{"reason":""}`, `{"reason":"  \t "}`, `{"reason":"` + strings.Repeat("x", 501) + `"}`} {
		rec := call(svc.token, path, body)
		uWantErr(t, rec, 400, "validation")
		if _, f := uErrOf(t, rec); f != "reason" {
			t.Errorf("field %q", f)
		}
	}
	uWantErr(t, call(svc.token, path, `{"reason":"x","force":true}`), 400, "validation")
	uWantErr(t, call(svc.token, "/api/v1/integration/holds/99999/release", `{"reason":"x"}`), 404, "not_found")
	if h := e.getHoldRow(hid); h.ReleasedAt != nil {
		t.Fatal("hold released by a refused request")
	}

	// Pending first: the service release completes it at once, deployed or not.
	uWantOK(t, e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, ""), http.StatusAccepted)
	h := uWantOK(t, call(svc.token, path, `{"reason":" torn down "}`), 200)
	if h["active"] != false || h["released_at"] == nil || h["released_by"] != nil || h["release_reason"] != "torn down" ||
		h["forced"] != false || h["release_requested_at"] == nil {
		t.Fatalf("released hold %v", h)
	}
	row := e.getHoldRow(hid)
	if row.ReleasedByService != "reaper" || row.ReleasedBy != nil {
		t.Errorf("row %+v", row)
	}
	rows, err := e.st.ListAudit(uBg, store.AuditFilter{Action: "hold.service_release"})
	if err != nil || len(rows) != 1 || rows[0].ActorService != "reaper" || rows[0].ActorID != nil {
		t.Fatalf("audit %+v %v", rows, err)
	}
	if n := e.count(hourglass.EventHoldReleased); n != 1 {
		t.Fatalf("hold.released delivered %d times", n)
	}
	ev := e.hook.EventsOfType(hourglass.EventHoldReleased)[0]
	if ev.Data["service_account"] != "reaper" || ev.Data["forced"] != false || ev.Data["reason"] != "torn down" {
		t.Errorf("event data %v", ev.Data)
	}
	uWantErr(t, call(svc.token, path, `{"reason":"again"}`), 409, "conflict")
	// The audit row is listed to admins with the account as actor.
	adm := e.adminSess()
	body := e.get("/api/v1/admin/audit?limit=50", &adm).Body.String()
	if !strings.Contains(body, "hold.service_release") || !strings.Contains(body, "reaper") {
		t.Errorf("admin audit lacks the service release: %s", body)
	}
}

func TestHOLD7_AdminForceReleaseCompletesPendingRelease(t *testing.T) {
	e := newReleaseEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	adm := e.dev("boss", store.RoleAdmin)
	hid := e.acquire(&dev, sid)
	path := "/api/v1/holds/" + uItoa(hid) + "/release"
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.poll()
	uWantOK(t, e.post(path, &dev, ""), http.StatusAccepted)
	h := uWantOK(t, e.post(path, &adm, `{"force":true,"reason":"maintenance"}`), 200)
	if h["active"] != false || h["forced"] != true || h["release_reason"] != "maintenance" {
		t.Fatalf("hold %v", h)
	}
}

func TestHOLD9_FailedPollDoesNotCompleteReleaseOverHTTP(t *testing.T) {
	e := newReleaseEnv(t)
	sid := e.setup("lab-a", true, nil)
	dev := e.dev("dev", store.RoleDeveloper)
	hid := e.acquire(&dev, sid)
	e.prov.SetAllocations(hourglass.Allocation{ResourceName: "lab-a"})
	e.poll()
	uWantOK(t, e.post("/api/v1/holds/"+uItoa(hid)+"/release", &dev, ""), http.StatusAccepted)
	e.prov.SetAllocations()
	e.prov.SetError(context.DeadlineExceeded)
	e.clk.Advance(5 * time.Minute)
	if err := e.col.RunOnce(uBg); err == nil {
		t.Fatal("the poll should fail")
	}
	if h := e.getHoldRow(hid); h.ReleasedAt != nil {
		t.Fatal("a failed poll completed the release")
	}
	e.prov.SetError(nil)
	e.step(5 * time.Minute)
	if h := e.getHoldRow(hid); h.ReleasedAt == nil {
		t.Fatal("the next successful poll did not complete the release")
	}
}
