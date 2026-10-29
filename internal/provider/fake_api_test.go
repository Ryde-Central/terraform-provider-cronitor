package provider

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI is an in-memory stand-in for the Cronitor REST API. It mimics the
// documented behaviour that matters to the provider: server-side defaults,
// partial updates via the bulk monitor PUT, alternate response shapes
// (environments as objects, normalised intervals and timestamps) and 404s.
type fakeAPI struct {
	t     *testing.T
	mu    sync.Mutex
	seq   int
	store map[string]map[string]map[string]any // collection -> key -> object
}

func newFakeAPI(t *testing.T) *httptest.Server {
	f := &fakeAPI{t: t, store: map[string]map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return srv
}

func (f *fakeAPI) coll(name string) map[string]map[string]any {
	if f.store[name] == nil {
		f.store[name] = map[string]map[string]any{}
	}
	return f.store[name]
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if user, _, ok := r.BasicAuth(); !ok || user != "test-key" {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"detail": "Authentication credentials were not provided."})
		return
	}
	if v := r.Header.Get("Cronitor-Version"); v != "2025-11-28" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unexpected version " + v})
		return
	}

	var body map[string]any
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api"), "/"), "/")
	collection, key := parts[0], ""
	if len(parts) > 1 {
		key = parts[1]
	}

	if collection == "monitors" {
		f.monitors(w, r.Method, key, body)
		return
	}
	if collection == "ping" {
		f.ping(w, key, r.URL.Query().Get("env"))
		return
	}
	defaults, ok := fakeDefaults[collection]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found."})
		return
	}
	f.generic(w, r.Method, collection, key, body, defaults)
}

// ping is telemetry (cronitor.link/p/<ping key>/<monitor>?env=<env>): like
// the API, it creates an unknown monitor and is the only way a monitor joins
// an environment. Like the API, a monitor can list an environment more than
// once.
func (f *fakeAPI) ping(w http.ResponseWriter, key, env string) {
	c := f.coll("monitors")
	m, ok := c[key]
	if !ok {
		m = map[string]any{"key": key, "name": key, "type": "heartbeat", "notify": []any{"default"},
			"paused": false, "environments": []any{}, "created": time.Now().UTC().Format(time.RFC3339)}
		c[key] = m
	}
	if env != "" {
		envs, _ := m["environments"].([]any)
		m["environments"] = append(envs, env)
	}
	w.WriteHeader(http.StatusOK)
}

func (f *fakeAPI) monitors(w http.ResponseWriter, method, key string, body map[string]any) {
	c := f.coll("monitors")
	switch {
	case method == http.MethodGet && key != "":
		m, ok := c[key]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found."})
			return
		}
		writeJSON(w, http.StatusOK, monitorView(m))
	case method == http.MethodPost && key == "":
		k, _ := body["key"].(string)
		if _, exists := c[k]; exists {
			writeJSON(w, http.StatusBadRequest, map[string]any{"key": []string{"Duplicate key error"}})
			return
		}
		if body["type"] == "check" && body["request"] == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"request": []string{"This field is required for check monitors"}})
			return
		}
		m := map[string]any{
			"name":                        k,
			"notify":                      []any{"default"},
			"failure_tolerance":           0,
			"schedule_tolerance":          0,
			"consecutive_alert_threshold": 10,
			"timezone":                    "UTC",
			"realert_interval":            "8 hours",
			"paused":                      false,
			"environments":                []any{},
			"created":                     time.Now().UTC().Format(time.RFC3339),
			"badges": map[string]any{
				"simple":   "https://cronitor.io/badges/" + k + "/simple.svg",
				"detailed": "https://cronitor.io/badges/" + k + "/detailed.svg",
			},
		}
		if body["type"] == "job" {
			m["grace_seconds"], m["platform"] = 60, "linux cron"
		} else {
			m["grace_seconds"] = 0
		}
		if body["type"] == "check" {
			m["platform"] = "http"
		}
		// Like the API, creating ignores environments; only telemetry adds them
		create := maps.Clone(body)
		delete(create, "environments")
		mergeMonitor(m, create)
		c[k] = m
		writeJSON(w, http.StatusCreated, monitorView(m))
	case method == http.MethodPut && key == "":
		list, _ := body["monitors"].([]any)
		var out []any
		for _, item := range list {
			in, _ := item.(map[string]any)
			k, _ := in["key"].(string)
			m, ok := c[k]
			if !ok {
				m = map[string]any{"name": k, "created": time.Now().UTC().Format(time.RFC3339)}
				c[k] = m
			}
			if added := addedEnvironments(m, in); len(added) > 0 {
				writeJSON(w, http.StatusBadRequest, map[string]any{k: map[string]any{"environments": []string{
					"Environments can be removed from a monitor but not added. To associate a monitor with an environment, start sending telemetry there."}}})
				return
			}
			mergeMonitor(m, in)
			out = append(out, monitorView(m))
		}
		writeJSON(w, http.StatusOK, map[string]any{"monitors": out})
	case method == http.MethodDelete && key != "":
		if _, ok := c[key]; !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found."})
			return
		}
		delete(c, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, nil)
	}
}

// addedEnvironments returns the environments an update would add to m.
func addedEnvironments(m, in map[string]any) []any {
	list, ok := in["environments"].([]any)
	if !ok {
		return nil
	}
	have, _ := m["environments"].([]any)
	var added []any
	for _, e := range list {
		if obj, ok := e.(map[string]any); ok && !slices.Contains(have, obj["key"]) {
			added = append(added, obj["key"])
		}
	}
	return added
}

// mergeMonitor applies a partial update, normalising values the way the API does.
func mergeMonitor(m, in map[string]any) {
	for k, v := range in {
		switch k {
		case "realert_interval":
			if s, ok := v.(string); ok && strings.HasSuffix(s, "h") {
				v = strings.TrimSuffix(s, "h") + " hours"
			}
		case "environments":
			// Like the API: only environment objects associate the monitor;
			// bare key strings are ignored.
			keys := []any{}
			if list, ok := v.([]any); ok {
				for _, e := range list {
					if obj, ok := e.(map[string]any); ok {
						keys = append(keys, obj["key"])
					}
				}
			}
			v = keys
		case "request":
			if rq, ok := v.(map[string]any); ok {
				merged := map[string]any{"method": "GET", "timeout_seconds": 10, "follow_redirects": true,
					"verify_ssl": true, "use_ipv6": false, "regions": []any{"us-east-1", "eu-central-1"},
					"headers": map[string]any{}, "cookies": map[string]any{}, "body": nil}
				if prev, ok := m["request"].(map[string]any); ok {
					maps.Copy(merged, prev)
				}
				maps.Copy(merged, rq)
				v = merged
			}
		}
		m[k] = v
	}
}

// monitorView renders a stored monitor as the API returns it: environments
// expanded to objects and unset optional values as null or [].
func monitorView(m map[string]any) map[string]any {
	out := maps.Clone(m)
	if envs, ok := m["environments"].([]any); ok {
		objs := make([]any, 0, len(envs))
		for _, e := range envs {
			objs = append(objs, map[string]any{"key": e, "name": strings.ToUpper(fmt.Sprint(e)[:1]) + fmt.Sprint(e)[1:]})
		}
		out["environments"] = objs
	}
	for _, k := range []string{"assertions", "tags", "statuspages", "schedules"} {
		if out[k] == nil {
			out[k] = []any{}
		}
	}
	for _, k := range []string{"group", "note", "request", "site"} {
		if _, ok := out[k]; !ok {
			out[k] = nil
		}
	}
	return out
}

type fakeCollection struct {
	keyPrefix string         // generated keys look like <prefix><n>; empty means derive from name
	defaults  map[string]any // applied on create
	listKeys  []string       // attributes returned as [] when unset
}

var fakeDefaults = map[string]fakeCollection{
	"notifications": {
		defaults: map[string]any{"status": "active", "environments": []any{}},
	},
	"groups": {
		defaults: map[string]any{"monitors": []any{}},
	},
	"environments": {
		defaults: map[string]any{"with_alerts": false, "default": false},
	},
	"statuspages": {
		keyPrefix: "sp_",
		defaults: map[string]any{"access": "public", "allow_search_index": true, "enable_performance_metrics": true,
			"enable_dark_mode": false, "default_time_period": "90d", "environment": "production",
			"logo_url": nil, "website_url": nil, "support_url": nil, "custom_domain": nil, "ip_allowlist": nil},
	},
	"statuspage_components": {
		keyPrefix: "comp_",
		defaults:  map[string]any{"autopublish": true, "position": "1", "description": nil, "type": "job"},
	},
	"maintenance_windows": {
		keyPrefix: "mw_",
		defaults: map[string]any{"affects_all_monitors": false, "affects_statuspage_components": false,
			"description": nil, "environment": nil, "statuspages": []any{}, "monitors": []any{}, "groups": []any{}},
	},
}

func (f *fakeAPI) generic(w http.ResponseWriter, method, collection, key string, body map[string]any, spec fakeCollection) {
	c := f.coll(collection)
	switch {
	case method == http.MethodGet && key != "":
		obj, ok := c[key]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found."})
			return
		}
		writeJSON(w, http.StatusOK, obj)
	case method == http.MethodPost && key == "":
		obj := maps.Clone(spec.defaults)
		obj["created"] = time.Now().UTC().Format(time.RFC3339)
		for k, v := range body {
			obj[k] = v
		}
		k, _ := body["key"].(string)
		if spec.keyPrefix != "" || k == "" {
			f.seq++
			if spec.keyPrefix != "" {
				k = fmt.Sprintf("%s%d", spec.keyPrefix, f.seq)
			} else {
				k = strings.ToLower(strings.ReplaceAll(fmt.Sprint(body["name"]), " ", "-"))
			}
		}
		if _, exists := c[k]; exists {
			writeJSON(w, http.StatusBadRequest, map[string]any{"key": []string{"key must be unique"}})
			return
		}
		obj["key"] = k
		normalise(collection, obj)
		c[k] = obj
		writeJSON(w, http.StatusCreated, obj)
	case method == http.MethodPut && key != "":
		obj, ok := c[key]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found."})
			return
		}
		for k, v := range body {
			if k == "key" {
				continue
			}
			obj[k] = v
		}
		normalise(collection, obj)
		writeJSON(w, http.StatusOK, obj)
	case method == http.MethodDelete && key != "":
		if _, ok := c[key]; !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found."})
			return
		}
		delete(c, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, nil)
	}
}

// normalise mimics server-side processing of stored objects.
func normalise(collection string, obj map[string]any) {
	switch collection {
	case "statuspages":
		delete(obj, "password") // write-only
		obj["badge_url_light"] = "https://cronitor.io/badges/" + fmt.Sprint(obj["key"]) + "/light.svg"
		obj["badge_url_dark"] = "https://cronitor.io/badges/" + fmt.Sprint(obj["key"]) + "/dark.svg"
	case "maintenance_windows":
		var dur time.Duration
		for _, k := range []string{"start", "end"} {
			if t, err := time.Parse(time.RFC3339, fmt.Sprint(obj[k])); err == nil {
				obj[k] = t.UTC().Format("2006-01-02T15:04:05+00:00")
			}
		}
		s, _ := time.Parse(time.RFC3339, fmt.Sprint(obj["start"]))
		e, _ := time.Parse(time.RFC3339, fmt.Sprint(obj["end"]))
		dur = e.Sub(s)
		obj["duration"] = int(dur.Seconds())
		obj["state"] = "upcoming"
	case "notifications":
		if n, ok := obj["notifications"].(map[string]any); ok {
			// Webhooks are returned as identifiers rather than URLs.
			if hooks, ok := n["webhooks"].([]any); ok {
				ids := make([]any, len(hooks))
				for i := range hooks {
					ids[i] = fmt.Sprintf("webhook-%d", i+1)
				}
				n["webhooks"] = ids
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
