package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestMonitorDecoding(t *testing.T) {
	raw := `{
		"key": "k", "type": "job",
		"notify": {"alerts": ["ops"], "events": {"complete": true}},
		"environments": [{"key": "production", "name": "Production"}, "staging"],
		"realert_interval": 8,
		"grace_seconds": "120",
		"group": {"key": "prod", "name": "Prod"}
	}`
	var m Monitor
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Notify.Alerts, []string{"ops"}) || m.Notify.Events == nil || !m.Notify.Events.Complete || m.Notify.Events.Run {
		t.Errorf("notify = %+v", m.Notify)
	}
	if !reflect.DeepEqual([]string(m.Environments), []string{"production", "staging"}) {
		t.Errorf("environments = %v", m.Environments)
	}
	if *m.RealertInterval != "8" || *m.GraceSeconds != 120 || m.Group != "prod" {
		t.Errorf("realert=%v grace=%v group=%v", *m.RealertInterval, *m.GraceSeconds, m.Group)
	}

	var n Notify
	if err := json.Unmarshal([]byte(`["default"]`), &n); err != nil || n.Events != nil || n.Alerts[0] != "default" {
		t.Errorf("array notify = %+v, %v", n, err)
	}
	if err := json.Unmarshal([]byte(`{"alerts": [], "events": true}`), &n); err != nil || n.Events == nil || !n.Events.Run {
		t.Errorf("events=true notify = %+v, %v", n, err)
	}
}

func TestNotifyEncoding(t *testing.T) {
	b, _ := json.Marshal(Notify{Alerts: []string{"a"}})
	if string(b) != `["a"]` {
		t.Errorf("got %s", b)
	}
	b, _ = json.Marshal(Notify{Alerts: []string{"a"}, Events: &NotifyEvents{Run: true}})
	if string(b) != `{"alerts":["a"],"events":{"run":true,"complete":false}}` {
		t.Errorf("got %s", b)
	}
}

func TestRequestHeadersAndRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "secret" || pass != "" {
			t.Errorf("basic auth = %q %q %v", user, pass, ok)
		}
		if got := r.Header.Get("Cronitor-Version"); got != APIVersion {
			t.Errorf("Cronitor-Version = %q", got)
		}
		if r.URL.Path != "/api/monitors/a%2Fb" && r.URL.RawPath != "/api/monitors/a%2Fb" {
			t.Errorf("path = %q (raw %q)", r.URL.Path, r.URL.RawPath)
		}
		_, _ = w.Write([]byte(`{"key": "a/b", "type": "job"}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL+"/api/", "secret")
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.GetMonitor(context.Background(), "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if m.Key != "a/b" || calls.Load() != 2 {
		t.Errorf("key=%q calls=%d", m.Key, calls.Load())
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail": "Not found."}`))
	}))
	defer srv.Close()

	c, _ := New(srv.URL, "k")
	_, err := c.GetGroup(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}
