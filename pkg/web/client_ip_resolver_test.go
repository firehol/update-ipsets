package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPResolverDefaultUsesRemoteAddr(t *testing.T) {
	r := &clientIPResolver{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "192.0.2.1")

	if got := r.clientIP(req); got != "10.0.0.1" {
		t.Fatalf("default resolver should use RemoteAddr, got %q", got)
	}
}

func TestClientIPResolverTrustProxy(t *testing.T) {
	r := &clientIPResolver{trustProxy: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 70.0.0.1")
	req.Header.Set("X-Real-IP", "192.0.2.1")

	if got := r.clientIP(req); got != "203.0.113.1" {
		t.Fatalf("should use first X-Forwarded-For entry, got %q", got)
	}
}

func TestClientIPResolverTrustProxyFallsBackToXRealIP(t *testing.T) {
	r := &clientIPResolver{trustProxy: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "192.0.2.1")

	if got := r.clientIP(req); got != "192.0.2.1" {
		t.Fatalf("should use X-Real-IP when no X-Forwarded-For, got %q", got)
	}
}

func TestClientIPResolverTrustProxyFallsBackToRemoteAddr(t *testing.T) {
	r := &clientIPResolver{trustProxy: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	if got := r.clientIP(req); got != "10.0.0.1" {
		t.Fatalf("should use RemoteAddr when no proxy headers, got %q", got)
	}
}

func TestClientIPResolverTrustCloudflare(t *testing.T) {
	r := &clientIPResolver{trustCloudflare: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	if got := r.clientIP(req); got != "198.51.100.1" {
		t.Fatalf("should use CF-Connecting-IP, got %q", got)
	}
}

func TestClientIPResolverTrustCloudflareFallsBackToRemoteAddr(t *testing.T) {
	r := &clientIPResolver{trustCloudflare: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	if got := r.clientIP(req); got != "10.0.0.1" {
		t.Fatalf("should ignore X-Forwarded-For when only CF trusted, got %q", got)
	}
}

func TestClientIPResolverBothTrusted(t *testing.T) {
	r := &clientIPResolver{trustProxy: true, trustCloudflare: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	if got := r.clientIP(req); got != "198.51.100.1" {
		t.Fatalf("CF-Connecting-IP should take precedence over X-Forwarded-For, got %q", got)
	}
}

func TestClientIPResolverMultiHopXForwardedFor(t *testing.T) {
	r := &clientIPResolver{trustProxy: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 70.0.0.1, 60.0.0.1")

	if got := r.clientIP(req); got != "203.0.113.1" {
		t.Fatalf("should use first entry in multi-hop X-Forwarded-For, got %q", got)
	}
}

func TestClientIPResolverMalformedHeadersIgnored(t *testing.T) {
	r := &clientIPResolver{trustProxy: true, trustCloudflare: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "not-an-ip")
	req.Header.Set("X-Forwarded-For", "also-not-ip")
	req.Header.Set("X-Real-IP", "")

	if got := r.clientIP(req); got != "10.0.0.1" {
		t.Fatalf("should fall back to RemoteAddr on malformed headers, got %q", got)
	}
}

func TestClientIPResolverSpoofedHeadersIgnoredByDefault(t *testing.T) {
	r := &clientIPResolver{}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	req.Header.Set("X-Forwarded-For", "5.6.7.8")
	req.Header.Set("X-Real-IP", "9.10.11.12")

	if got := r.clientIP(req); got != "10.0.0.1" {
		t.Fatalf("spoofed headers should be ignored without trust config, got %q", got)
	}
}

func TestClientIPResolverProxyIgnoresCFHeader(t *testing.T) {
	r := &clientIPResolver{trustProxy: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	if got := r.clientIP(req); got != "203.0.113.1" {
		t.Fatalf("CF header should be ignored when only proxy trusted, got %q", got)
	}
}

func TestClientIPResolverCloudflareIgnoresProxyHeaders(t *testing.T) {
	r := &clientIPResolver{trustCloudflare: true}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "192.0.2.1")

	if got := r.clientIP(req); got != "198.51.100.1" {
		t.Fatalf("should use CF header and ignore proxy headers, got %q", got)
	}
}
