package web

import (
	"net/http"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func TestClientIPAPIEndpointReturnsRemoteAddrByDefault(t *testing.T) {
	server := newWebHTTPTestServer(t, newHandler(&engine.Engine{}, Options{}, nil))

	status, _, body := server.do(t, http.MethodGet, "/api/v1/client-ip", func(req *http.Request) {
		req.Header.Set("CF-Connecting-IP", "198.51.100.7")
	})

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var payload struct {
		IP string `json:"ip"`
	}
	decodeTestJSONInto(t, body, &payload)
	if payload.IP != "127.0.0.1" {
		t.Fatalf("ip = %q, want 127.0.0.1 (RemoteAddr, CF header ignored)", payload.IP)
	}
}

func TestClientIPAPIEndpointReturnsForwardedIPv4WhenTrusted(t *testing.T) {
	server := newWebHTTPTestServer(t, newHandler(&engine.Engine{}, Options{TrustCloudflareHeaders: true}, nil))

	status, _, body := server.do(t, http.MethodGet, "/api/v1/client-ip", func(req *http.Request) {
		req.Header.Set("CF-Connecting-IP", "198.51.100.7")
	})

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var payload struct {
		IP string `json:"ip"`
	}
	decodeTestJSONInto(t, body, &payload)
	if payload.IP != "198.51.100.7" {
		t.Fatalf("ip = %q, want 198.51.100.7", payload.IP)
	}
}
