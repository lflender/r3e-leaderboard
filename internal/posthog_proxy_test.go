package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostHogForwardPath(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		want        string
	}{
		{name: "base path", requestPath: "/t", want: "/"},
		{name: "base slash", requestPath: "/t/", want: "/"},
		{name: "event path", requestPath: "/t/e/", want: "/e/"},
		{name: "decide path", requestPath: "/t/decide/", want: "/decide/"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := postHogForwardPath(tc.requestPath)
			if got != tc.want {
				t.Fatalf("postHogForwardPath(%q) = %q, want %q", tc.requestPath, got, tc.want)
			}
		})
	}
}

func TestConfigurePostHogProxy_ForwardsToConfiguredTarget(t *testing.T) {
	var gotPath string
	var gotHost string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHost = r.Host
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	t.Setenv(postHogProxyEnvVar, upstream.URL)

	mux := http.NewServeMux()
	configurePostHogProxy(mux)

	req := httptest.NewRequest(http.MethodPost, postHogProxyPath+"/e/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("proxy status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if gotPath != "/e/" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/e/")
	}

	if gotHost == "" {
		t.Fatalf("upstream host header should be set")
	}
}

func TestConfigurePostHogProxy_InvalidTargetDisablesRoute(t *testing.T) {
	t.Setenv(postHogProxyEnvVar, "://bad-url")

	mux := http.NewServeMux()
	configurePostHogProxy(mux)

	req := httptest.NewRequest(http.MethodGet, postHogProxyPath+"/e/", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d when proxy is disabled", rr.Code, http.StatusNotFound)
	}
}
