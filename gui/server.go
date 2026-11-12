package gui

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed web
var webFS embed.FS

type app struct {
	token    string
	version  string
	lastSeen atomic.Int64
	// debug keeps the WebView2 context menu and devtools available and prints
	// the session URL; enabled with FOCUSD_GUI_DEBUG=1.
	debug bool
}

func startServer(a *app) (string, func(), error) {
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		return "", nil, err
	}
	indexHTML, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return "", nil, err
	}
	// The token is injected once; the browser navigates to it with ?t= and the
	// API then requires it on every request. Keeping it out of a separate file
	// means an unauthenticated local process cannot read it from disk.
	page := strings.ReplaceAll(string(indexHTML), "__FOCUSD_TOKEN__", a.token)

	static := http.FileServer(http.FS(assets))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", a.handleAPI)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if !a.authorizePage(w, r) {
				http.Error(w, "Open the DeepFocus window to use this page.", http.StatusForbidden)
				return
			}
			a.touch()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			setSecurityHeaders(w)
			_, _ = w.Write([]byte(page))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/favicon.ico" {
			w.Header().Set("Cache-Control", "public, max-age=3600")
			setSecurityHeaders(w)
			static.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("gui: server stopped: %v", err)
		}
	}()

	url := fmt.Sprintf("http://%s/?t=%s", ln.Addr().String(), a.token)
	shutdown := func() {
		_ = srv.Close()
	}
	return url, shutdown, nil
}

func (a *app) authorizePage(w http.ResponseWriter, r *http.Request) bool {
	token := r.URL.Query().Get("t")
	if token == "" {
		if c, err := r.Cookie("focusd_session"); err == nil {
			token = c.Value
		}
	}
	if !secureEqual(token, a.token) {
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "focusd_session",
		Value:    a.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return true
}

func secureEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
			"script-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
}

func (a *app) handleAPI(w http.ResponseWriter, r *http.Request) {
	a.touch()
	if !secureEqual(r.Header.Get("X-Focusd-Token"), a.token) {
		writeError(w, http.StatusForbidden, "unauthorized")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
		writeError(w, http.StatusForbidden, "bad origin")
		return
	}

	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("gui: panic in %s %s: %v", r.Method, r.URL.Path, rec)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
	}()

	var err error
	path := r.URL.Path
	switch {
	case path == "/api/bootstrap" && r.Method == http.MethodGet:
		err = a.bootstrap(w, r)
	case path == "/api/overview" && r.Method == http.MethodGet:
		err = a.overview(w, r)
	case path == "/api/stats" && r.Method == http.MethodGet:
		err = a.stats(w, r)
	case path == "/api/sessions" && r.Method == http.MethodGet:
		err = a.sessions(w, r)
	case path == "/api/limits" && r.Method == http.MethodGet:
		err = a.limits(w, r)
	case path == "/api/settings" && r.Method == http.MethodGet:
		err = a.settings(w, r)
	case path == "/api/pomodoro" && r.Method == http.MethodGet:
		err = a.pomodoro(w, r)
	case path == "/api/actions" && r.Method == http.MethodPost:
		err = a.action(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		log.Printf("gui: %s %s: %v", r.Method, path, err)
		if ce, ok := actionError(err); ok {
			writeError(w, http.StatusBadRequest, ce.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func sameOrigin(origin, host string) bool {
	origin = strings.TrimSuffix(origin, "/")
	return origin == "http://"+host || origin == "https://"+host
}

func writeJSON(w http.ResponseWriter, v interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	setSecurityHeaders(w)
	return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func decodeJSON(r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}
