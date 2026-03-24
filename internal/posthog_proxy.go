package internal

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

const (
	postHogProxyEnvVar = "POSTHOG_PROXY_TARGET"
	postHogDefaultURL  = "https://us.i.posthog.com"
	postHogProxyPath   = "/t"
)

func configurePostHogProxy(mux *http.ServeMux) {
	targetRaw := strings.TrimSpace(os.Getenv(postHogProxyEnvVar))
	if targetRaw == "" {
		targetRaw = postHogDefaultURL
	}

	target, err := url.Parse(targetRaw)
	if err != nil || target.Scheme == "" || target.Host == "" {
		log.Printf("⚠️ Invalid %s=%q, PostHog proxy disabled", postHogProxyEnvVar, targetRaw)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		forwardPath := postHogForwardPath(req.URL.Path)
		basePath := strings.TrimSuffix(target.Path, "/")
		req.URL.Path = basePath + forwardPath
		req.URL.RawPath = req.URL.Path
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("⚠️ PostHog proxy error for %s: %v", r.URL.Path, err)
		http.Error(w, "PostHog upstream unavailable", http.StatusBadGateway)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	mux.Handle(postHogProxyPath, handler)
	mux.Handle(postHogProxyPath+"/", handler)
	log.Printf("📈 PostHog ingest proxy enabled: %s/* -> %s", postHogProxyPath, target.String())
}

func postHogForwardPath(requestPath string) string {
	forwardPath := strings.TrimPrefix(requestPath, postHogProxyPath)
	if forwardPath == "" {
		forwardPath = "/"
	}
	if !strings.HasPrefix(forwardPath, "/") {
		forwardPath = "/" + forwardPath
	}
	return forwardPath
}
