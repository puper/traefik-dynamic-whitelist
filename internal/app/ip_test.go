package app

import (
	"net/http"
	"net/netip"
	"testing"
)

func TestClientIPUsesRemoteAddrWhenProxyIsNotTrusted(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	got, err := ClientIP(req, []string{"X-Forwarded-For"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPUsesConfiguredHeaderForTrustedProxy(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	got, err := ClientIP(req, []string{"X-Forwarded-For"}, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "203.0.113.10" {
		t.Fatalf("got %q", got)
	}
}
