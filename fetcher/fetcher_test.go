package fetcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// withServer points the fetcher at a local test server for the duration of a test.
func withServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	oldURL, oldClient, oldTimeout := baseURL, client, timeout
	baseURL = srv.URL + "/"
	client = srv.Client()
	timeout = 2 * time.Second
	t.Cleanup(func() { baseURL, client, timeout = oldURL, oldClient, oldTimeout })
}

func TestFetchStack_badID(t *testing.T) {
	if _, err := FetchStack("../passwd"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchStack_valid(t *testing.T) {
	var gotUA, gotPath string
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA, gotPath = r.Header.Get("User-Agent"), r.URL.Path
		fmt.Fprint(w, `{"id":"gleam","name":"Gleam","rules":{"gitignore":["erl_crash.dump","_build/"]}}`)
	})

	rule, err := FetchStack("gleam")
	if err != nil {
		t.Fatal(err)
	}
	if rule == nil || rule.Name != "Gleam" || len(rule.Rules.Gitignore) != 2 {
		t.Fatalf("bad rule: %+v", rule)
	}
	if gotPath != "/gleam.json" {
		t.Fatalf("requested %q", gotPath)
	}
	if !strings.HasPrefix(gotUA, "siig/") {
		t.Fatalf("user agent %q", gotUA)
	}
}

func TestFetchStack_notFound(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	rule, err := FetchStack("nope")
	if err != nil || rule != nil {
		t.Fatalf("404 should be a silent miss, got %+v err=%v", rule, err)
	}
}

func TestFetchStack_statusError(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	if _, err := FetchStack("gleam"); err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestFetchStack_rejectsBadPayloads(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed", body: "{", wantErr: "cannot parse"},
		{name: "unknown field", body: `{"id":"gleam","name":"G","bogus":1}`, wantErr: "cannot parse"},
		{name: "wrong id", body: `{"id":"other","name":"G"}`, wantErr: "rejected rule"},
		{name: "missing name", body: `{"id":"gleam"}`, wantErr: "rejected rule"},
		{name: "newline injection", body: `{"id":"gleam","name":"G","rules":{"gitignore":["a\n*"]}}`, wantErr: "rejected rule"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.body
			withServer(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, body)
			})
			_, err := FetchStack("gleam")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestFetchStack_capsResponseSize(t *testing.T) {
	huge := `{"id":"gleam","name":"G","rules":{"gitignore":["` + strings.Repeat("a", maxRuleBytes) + `"]}}`
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, huge)
	})

	if _, err := FetchStack("gleam"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized body should be refused, got %v", err)
	}
}

func TestFetchStack_timesOut(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		fmt.Fprint(w, `{"id":"gleam","name":"G"}`)
	})
	timeout = 20 * time.Millisecond

	if _, err := FetchStack("gleam"); err == nil {
		t.Fatal("expected timeout error")
	}
}
