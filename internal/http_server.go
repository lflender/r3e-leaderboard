package internal

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func StartHTTPServer(port int) *http.Server {
	mux := http.NewServeMux()

	configurePostHogProxy(mux)

	// Serve static files from current directory
	fs := http.FileServer(http.Dir("."))

	// Specialized handler to serve driver_index with gzip when supported
	mux.HandleFunc("/cache/driver_index.json", func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept-Encoding")
		wantGzip := strings.Contains(accept, "gzip")
		gzPath := "cache/driver_index.json.gz"

		f, err := os.Open(gzPath)
		if err != nil {
			log.Printf("❌ Failed to open %s: %v", gzPath, err)
			http.NotFound(w, r)
			return
		}
		defer f.Close()

		w.Header().Set("Vary", "Accept-Encoding")

		if wantGzip {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "application/json")
			if _, copyErr := io.Copy(w, f); copyErr != nil {
				log.Printf("⚠️ Failed streaming gz driver index: %v", copyErr)
			}
			return
		}

		// Client does not accept gzip: decompress server-side
		gr, zerr := gzip.NewReader(f)
		if zerr != nil {
			log.Printf("⚠️ Failed to create gzip reader: %v", zerr)
			http.Error(w, "Failed to read driver index", http.StatusInternalServerError)
			return
		}
		defer gr.Close()
		w.Header().Set("Content-Type", "application/json")
		if _, copyErr := io.Copy(w, gr); copyErr != nil {
			log.Printf("⚠️ Failed streaming decompressed driver index: %v", copyErr)
		}
	})

	// Default handler for all other paths
	mux.Handle("/", fs)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("🌐 HTTP server starting on port %d", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ HTTP server error: %v", err)
		}
	}()

	return httpServer
}
