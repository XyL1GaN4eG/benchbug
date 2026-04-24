package target

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	mrand "math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Options struct {
	Logger      *slog.Logger
	Seed        int64
	Auth        bool
	MaxBytes    int
	LogRequests bool
}

type Server struct {
	log      *slog.Logger
	auth     bool
	maxBytes int
	logReqs  bool

	usersMu sync.RWMutex
	users   map[string]User
	nextID  atomic.Int64

	randMu sync.Mutex
	rand   *mrand.Rand

	tokens sync.Map
}

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 8 << 20
	}
	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Server{
		log:      opts.Logger,
		auth:     opts.Auth,
		maxBytes: opts.MaxBytes,
		logReqs:  opts.LogRequests,
		users:    map[string]User{},
		rand:     mrand.New(mrand.NewSource(seed)),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("GET /users", s.listUsers)
	mux.HandleFunc("POST /users", s.createUser)
	mux.HandleFunc("GET /users/{id}", s.getUser)
	mux.HandleFunc("GET /slow", s.slow)
	mux.HandleFunc("GET /flaky", s.flaky)
	mux.HandleFunc("GET /bytes", s.bytes)
	if s.logReqs {
		return requestLog(s.log, mux)
	}
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now().UTC()})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req)
	if req.Username == "" {
		req.Username = "benchbug"
	}
	token := "tok_" + randomHex(12)
	s.tokens.Store(token, req.Username)
	w.Header().Set("X-Auth-Token", token)
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  req.Username,
	})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	limit := clampInt(queryInt(r, "limit", 20), 1, 1000)
	s.usersMu.RLock()
	defer s.usersMu.RUnlock()
	users := make([]User, 0, min(limit, len(s.users)))
	for _, u := range s.users {
		if len(users) >= limit {
			break
		}
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users, "count": len(users)})
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if req.Name == "" {
		req.Name = "User"
	}
	if req.Email == "" {
		req.Email = strings.ToLower(req.Name) + "@example.test"
	}
	id := strconv.FormatInt(s.nextID.Add(1), 10)
	u := User{
		ID:        id,
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now().UTC(),
	}
	s.usersMu.Lock()
	s.users[id] = u
	s.usersMu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"data": u})
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	id := r.PathValue("id")
	s.usersMu.RLock()
	u, ok := s.users[id]
	s.usersMu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": u})
}

func (s *Server) slow(w http.ResponseWriter, r *http.Request) {
	ms := clampInt(queryInt(r, "ms", 250), 0, 30_000)
	if err := sleepContext(r.Context(), time.Duration(ms)*time.Millisecond); err != nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slept_ms": ms})
}

func (s *Server) flaky(w http.ResponseWriter, r *http.Request) {
	rate := clampInt(queryInt(r, "rate", 10), 0, 100)
	s.randMu.Lock()
	hit := s.rand.Intn(100) < rate
	s.randMu.Unlock()
	if hit {
		writeError(w, http.StatusInternalServerError, "flaky_failure", fmt.Sprintf("rate=%d", rate))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rate": rate})
}

func (s *Server) bytes(w http.ResponseWriter, r *http.Request) {
	n := clampInt(queryBytes(r, "n", 1024), 0, s.maxBytes)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(n))
	buf := make([]byte, min(n, 32*1024))
	for i := range buf {
		buf[i] = byte('a' + i%26)
	}
	remaining := n
	for remaining > 0 {
		chunk := min(remaining, len(buf))
		if _, err := w.Write(buf[:chunk]); err != nil {
			return
		}
		remaining -= chunk
	}
}

func (s *Server) authorized(w http.ResponseWriter, r *http.Request) bool {
	if !s.auth {
		return true
	}
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return false
	}
	if _, ok := s.tokens.Load(token); !ok {
		writeError(w, http.StatusForbidden, "forbidden", "unknown token")
		return false
	}
	return true
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func queryBytes(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	if raw == "" {
		return fallback
	}
	mult := 1
	switch {
	case strings.HasSuffix(raw, "kb"):
		mult = 1024
		raw = strings.TrimSuffix(raw, "kb")
	case strings.HasSuffix(raw, "mb"):
		mult = 1024 * 1024
		raw = strings.TrimSuffix(raw, "mb")
	case strings.HasSuffix(raw, "b"):
		raw = strings.TrimSuffix(raw, "b")
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > math.MaxInt/mult {
		return fallback
	}
	return n * mult
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func requestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration", time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func Serve(ctx context.Context, addr string, opts Options) error {
	s := &http.Server{
		Addr:              addr,
		Handler:           New(opts).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
