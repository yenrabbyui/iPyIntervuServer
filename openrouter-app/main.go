package main

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

const (
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	maxBodySize   = 1 << 20 // 1 MiB
)

func main() {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	privateKeyPEM, err := loadPrivateKeyPEM()
	if err != nil {
		log.Fatal(err)
	}

	auth, err := newAuthService(privateKeyPEM)
	if err != nil {
		log.Fatalf("auth setup failed: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	if listenAddr := os.Getenv("LISTEN_ADDR"); listenAddr != "" {
		addr = listenAddr + ":" + port
	}

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/auth/challenge", auth.handleChallenge)
	mux.HandleFunc("POST /api/auth/verify", auth.handleVerify)
	mux.HandleFunc("POST /api/chat", auth.requireSession(handleChat(apiKey)))
	mux.Handle("GET /{$}", http.FileServer(http.FS(static)))
	mux.Handle("GET /style.css", contentHandler(static, "style.css", "text/css; charset=utf-8"))
	mux.Handle("GET /app.js", contentHandler(static, "app.js", "application/javascript"))

	server := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("listening on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func handleChat(apiKey string) http.HandlerFunc {
	client := &http.Client{}

	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, openRouterURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		upstream.Header.Set("Authorization", "Bearer "+apiKey)
		upstream.Header.Set("Content-Type", "application/json")
		if referer := os.Getenv("OPENROUTER_HTTP_REFERER"); referer != "" {
			upstream.Header.Set("HTTP-Referer", referer)
		}
		if title := os.Getenv("OPENROUTER_APP_TITLE"); title != "" {
			upstream.Header.Set("X-Title", title)
		}

		resp, err := client.Do(upstream)
		if err != nil {
			log.Printf("openrouter request failed: %v", err)
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("streaming response failed: %v", err)
		}
	}
}

func contentHandler(static fs.FS, name, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(static, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
