package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestHandleAddAllowsSpecificIP(t *testing.T) {
	dir := t.TempDir()
	server, err := NewServer(Config{
		AdminToken:   "secret",
		StatePath:    filepath.Join(dir, "state.json"),
		TraefikPath:  filepath.Join(dir, "whitelist.yml"),
		TempDuration: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	reqBody := bytes.NewBufferString(`{"type":"perm","ip":"203.0.113.10"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/add", reqBody)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	server.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	temporary, permanent, err := server.store.Info(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 || len(permanent) != 1 || permanent[0].IP != "203.0.113.10" {
		t.Fatalf("unexpected state: temporary=%#v permanent=%#v", temporary, permanent)
	}
}

func TestAPIRoutesRequireAuth(t *testing.T) {
	dir := t.TempDir()
	server, err := NewServer(Config{
		AdminToken:   "secret",
		StatePath:    filepath.Join(dir, "state.json"),
		TraefikPath:  filepath.Join(dir, "whitelist.yml"),
		TempDuration: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "info", method: http.MethodGet, path: "/api/info"},
		{name: "add", method: http.MethodPost, path: "/api/add", body: `{"type":"temp"}`},
		{name: "delete", method: http.MethodPost, path: "/api/delete", body: `{"ip":"203.0.113.10"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			resp := httptest.NewRecorder()

			server.Routes().ServeHTTP(resp, req)
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestAPIRoutesRejectWrongAuth(t *testing.T) {
	dir := t.TempDir()
	server, err := NewServer(Config{
		AdminToken:   "secret",
		StatePath:    filepath.Join(dir, "state.json"),
		TraefikPath:  filepath.Join(dir, "whitelist.yml"),
		TempDuration: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp := httptest.NewRecorder()

	server.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestHandleDeleteRemovesSpecificIP(t *testing.T) {
	dir := t.TempDir()
	server, err := NewServer(Config{
		AdminToken:   "secret",
		StatePath:    filepath.Join(dir, "state.json"),
		TraefikPath:  filepath.Join(dir, "whitelist.yml"),
		TempDuration: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.store.Add("203.0.113.10", "perm", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	reqBody := bytes.NewBufferString(`{"ip":"203.0.113.10"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/delete", reqBody)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	server.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	temporary, permanent, err := server.store.Info(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(temporary) != 0 || len(permanent) != 0 {
		t.Fatalf("unexpected state: temporary=%#v permanent=%#v", temporary, permanent)
	}
}

func TestHandleAddRejectsInvalidSpecificIP(t *testing.T) {
	dir := t.TempDir()
	server, err := NewServer(Config{
		AdminToken:   "secret",
		StatePath:    filepath.Join(dir, "state.json"),
		TraefikPath:  filepath.Join(dir, "whitelist.yml"),
		TempDuration: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	reqBody := bytes.NewBufferString(`{"type":"perm","ip":"not-an-ip"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/add", reqBody)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	server.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var payload apiResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error == nil || payload.Error.Code != "INVALID_IP" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestHandleDeleteRejectsMissingIP(t *testing.T) {
	dir := t.TempDir()
	server, err := NewServer(Config{
		AdminToken:   "secret",
		StatePath:    filepath.Join(dir, "state.json"),
		TraefikPath:  filepath.Join(dir, "whitelist.yml"),
		TempDuration: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	reqBody := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/delete", reqBody)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	server.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}
