// SPDX-License-Identifier: MIT

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClockOffsetControl(t *testing.T) {
	base := time.Date(2031, 3, 5, 12, 0, 0, 0, time.UTC)
	c := &offsetClock{wall: func() time.Time { return base }}
	srv := httptest.NewServer(controlHandler(c))
	defer srv.Close()

	do := func(method, body string) (int, string) {
		req, _ := http.NewRequest(method, srv.URL+"/clock", strings.NewReader(body))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var sb strings.Builder
		buf := make([]byte, 512)
		for {
			n, err := resp.Body.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return resp.StatusCode, sb.String()
	}

	if got := c.Now(); !got.Equal(base) {
		t.Fatalf("initial Now = %v", got)
	}
	if code, body := do("PUT", `{"offset":"720h"}`); code != 200 || !strings.Contains(body, `"offset":"720h0m0s"`) {
		t.Fatalf("put: %d %s", code, body)
	}
	if got, want := c.Now(), base.Add(720*time.Hour); !got.Equal(want) {
		t.Errorf("Now = %v, want %v", got, want)
	}
	// A second PUT replaces the offset, it does not add to it.
	if code, _ := do("PUT", `{"offset":"-1h"}`); code != 200 {
		t.Fatalf("put -1h: %d", code)
	}
	if got, want := c.Now(), base.Add(-time.Hour); !got.Equal(want) {
		t.Errorf("Now = %v, want %v", got, want)
	}
	if code, body := do("GET", ""); code != 200 || !strings.Contains(body, `"offset":"-1h0m0s"`) {
		t.Errorf("get: %d %s", code, body)
	}
	for _, bad := range []string{`{"offset":"soon"}`, `{"offset":1}`, `{"extra":1}`, `nope`} {
		if code, _ := do("PUT", bad); code != 400 {
			t.Errorf("PUT %s = %d, want 400", bad, code)
		}
	}
	if got, want := c.Now(), base.Add(-time.Hour); !got.Equal(want) {
		t.Errorf("a rejected PUT changed the clock: %v", got)
	}
	if code, _ := do("DELETE", ""); code != 405 {
		t.Errorf("DELETE = %d, want 405", code)
	}
}
