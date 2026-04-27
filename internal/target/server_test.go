package target

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTargetCRUDFlow(t *testing.T) {
	srv := httptest.NewServer(New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Seed:   1,
		Auth:   true,
	}).Handler())
	defer srv.Close()

	client := srv.Client()
	token := login(t, client, srv.URL)

	createBody := bytes.NewBufferString(`{"name":"Ivan","email":"ivan@example.test"}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/users", createBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	var create struct {
		Data User `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&create); err != nil {
		t.Fatal(err)
	}
	if create.Data.ID == "" {
		t.Fatal("expected user id")
	}

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/users/"+create.Data.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", resp.StatusCode)
	}
}

func TestTargetUtilityEndpoints(t *testing.T) {
	srv := httptest.NewServer(New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Seed:   1,
		Auth:   false,
	}).Handler())
	defer srv.Close()

	start := time.Now()
	resp, err := srv.Client().Get(srv.URL + "/slow?ms=20")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("slow status = %d", resp.StatusCode)
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("/slow returned too fast")
	}

	resp, err = srv.Client().Get(srv.URL + "/flaky?rate=100")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("flaky status = %d", resp.StatusCode)
	}

	resp, err = srv.Client().Get(srv.URL + "/bytes?n=4kb")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 4*1024 {
		t.Fatalf("bytes len = %d", len(b))
	}
}

func login(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	resp, err := client.Post(baseURL+"/login", "application/json", bytes.NewBufferString(`{"username":"bench"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Token == "" {
		t.Fatal("expected token")
	}
	return out.Token
}
