package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/scheduler"
)

func TestSurfaceHandlerModesRegisterExpectedSurfaces(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	eng, _ := testHandler(t, Options{EnableAll: true})
	runner := scheduler.New(eng, true, nil)
	opts := Options{EnableAll: true}

	shared := newWebHTTPTestServer(t, newHandler(eng, opts, runner))
	publicOnly := newWebHTTPTestServer(t, newPublicHandler(eng, opts, runner))
	adminOnly := newWebHTTPTestServer(t, newAdminHandler(eng, opts, runner))

	assertRouteStatus(t, shared, "/healthz", "", http.StatusOK)
	assertRouteStatus(t, shared, "/api/v1/admin/status", "admin", http.StatusOK)
	assertRouteStatus(t, publicOnly, "/healthz", "", http.StatusOK)
	assertRouteStatus(t, publicOnly, "/admin", "admin", http.StatusNotFound)
	assertRouteStatus(t, publicOnly, "/api/v1/admin/status", "admin", http.StatusNotFound)
	assertRouteStatus(t, adminOnly, "/healthz", "", http.StatusNotFound)
	assertRouteStatus(t, adminOnly, "/api/v1/status", "", http.StatusNotFound)
	assertRouteStatus(t, adminOnly, "/admin", "admin", http.StatusOK)
}

func assertRouteStatus(t *testing.T, server *webHTTPTestServer, path, user string, want int) {
	t.Helper()

	status, _, _ := server.do(t, http.MethodGet, path, func(req *http.Request) {
		if user != "" {
			req.SetBasicAuth(user, "secret")
		}
	})
	if status != want {
		t.Fatalf("%s status = %d, want %d", path, status, want)
	}
}

func TestRouteMethodContracts(t *testing.T) {
	_, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	server := newWebHTTPTestServer(t, handler)

	status, _, _ := server.do(t, http.MethodGet, "/api/v1/status", nil)
	if status != http.StatusOK {
		t.Fatalf("GET public status = %d, want 200", status)
	}

	status, headers, _ := server.do(t, http.MethodPost, "/api/v1/status", nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("POST public status = %d, want 405", status)
	}
	assertAllowContains(t, headers, http.MethodGet)
	assertAllowContains(t, headers, http.MethodHead)

	status, headers, _ = server.do(t, http.MethodGet, "/api/v1/admin/run", nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("GET admin run = %d, want 405", status)
	}
	assertAllowExactly(t, headers, http.MethodPost)

	status, _, _ = server.do(t, http.MethodPost, "/api/v1/admin/run", nil)
	if status != http.StatusAccepted {
		t.Fatalf("POST admin run = %d, want 202", status)
	}

	status, headers, _ = server.do(t, http.MethodGet, "/api/v1/admin/feeds/sample/recheck", nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("GET admin feed recheck = %d, want 405", status)
	}
	assertAllowExactly(t, headers, http.MethodPost)

	status, headers, _ = server.do(t, http.MethodOptions, "/api/v1/status", nil)
	if status != http.StatusNoContent {
		t.Fatalf("OPTIONS public status = %d, want 204", status)
	}
	if got := headers.Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("CORS allow methods = %q, want GET, OPTIONS", got)
	}
}

func TestMCPAndAdminCORSContracts(t *testing.T) {
	_, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	server := newWebHTTPTestServer(t, handler)

	status, headers, _ := server.do(t, http.MethodOptions, "/mcp", func(req *http.Request) {
		req.Header.Set("Origin", "https://client.example")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "content-type,mcp-session-id,mcp-protocol-version,last-event-id")
	})
	if status != http.StatusNoContent {
		t.Fatalf("OPTIONS /mcp status = %d, want 204", status)
	}
	if got := headers.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("MCP CORS origin = %q, want *", got)
	}
	if got := headers.Get("Access-Control-Allow-Methods"); got != "GET, POST, DELETE, OPTIONS" {
		t.Fatalf("MCP CORS methods = %q, want GET, POST, DELETE, OPTIONS", got)
	}
	for _, want := range []string{"Content-Type", "Mcp-Session-Id", "MCP-Protocol-Version", "Last-Event-ID"} {
		assertCommaHeaderContains(t, headers, "Access-Control-Allow-Headers", want)
	}
	assertCommaHeaderContains(t, headers, "Access-Control-Expose-Headers", "Mcp-Session-Id")

	status, headers, _ = server.do(t, http.MethodOptions, "/api/v1/status", func(req *http.Request) {
		req.Header.Set("Origin", "https://client.example")
	})
	if status != http.StatusNoContent {
		t.Fatalf("OPTIONS public status = %d, want 204", status)
	}
	if got := headers.Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("public CORS methods = %q, want GET, OPTIONS", got)
	}
	if got := headers.Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Fatalf("public CORS headers = %q, want Content-Type", got)
	}
	if got := headers.Get("Access-Control-Expose-Headers"); got != "" {
		t.Fatalf("public CORS expose headers = %q, want empty", got)
	}

	status, headers, _ = server.do(t, http.MethodOptions, "/api/v1/admin/status", func(req *http.Request) {
		req.Header.Set("Origin", "https://client.example")
	})
	if status != http.StatusNoContent {
		t.Fatalf("OPTIONS admin status = %d, want 204", status)
	}
	for _, name := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Expose-Headers"} {
		if got := headers.Get(name); got != "" {
			t.Fatalf("admin %s = %q, want empty", name, got)
		}
	}
}

func TestAdminReadRoutesAllowHEAD(t *testing.T) {
	opts := Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	}
	_, handler := testHandler(t, opts)
	server := newWebHTTPTestServer(t, handler)

	for _, path := range []string{
		"/api/v1/admin/status",
		"/api/v1/admin/feeds",
		"/api/v1/admin/feeds/sample",
		"/api/v1/admin/feeds/sample/manifest",
		"/api/v1/admin/schedule",
		"/api/v1/admin/integrity",
		"/api/v1/admin/integrity/entities",
	} {
		status, _, _ := server.do(t, http.MethodGet, path, nil)
		headStatus, _, _ := server.do(t, http.MethodHead, path, nil)
		if headStatus != status {
			t.Fatalf("HEAD %s status = %d, want GET status %d", path, headStatus, status)
		}
	}

	_, artifactHandler := testHandlerWithArtifactCatalog(t, opts)
	artifactServer := newWebHTTPTestServer(t, artifactHandler)
	status, _, _ := artifactServer.do(t, http.MethodGet, "/api/v1/admin/artifacts/dronebl", nil)
	headStatus, _, _ := artifactServer.do(t, http.MethodHead, "/api/v1/admin/artifacts/dronebl", nil)
	if headStatus != status {
		t.Fatalf("HEAD admin artifact detail status = %d, want GET status %d", headStatus, status)
	}
}

func TestAdminActionRoutesRejectHEAD(t *testing.T) {
	_, handler := testHandler(t, Options{
		EnableAll:                 true,
		AdminAuthMode:             AdminAuthModeDisabled,
		AllowUnauthenticatedAdmin: true,
	})
	server := newWebHTTPTestServer(t, handler)

	for _, path := range []string{
		"/api/v1/admin/run",
		"/api/v1/admin/feeds/sample/recheck",
		"/api/v1/admin/feeds/sample/reprocess",
		"/api/v1/admin/integrity/entities/rebuild",
		"/api/v1/admin/integrity/reprocess",
	} {
		status, headers, _ := server.do(t, http.MethodHead, path, nil)
		if status != http.StatusMethodNotAllowed {
			t.Fatalf("HEAD %s status = %d, want 405", path, status)
		}
		assertAllowExactly(t, headers, http.MethodPost)
	}
}

func assertAllowExactly(t *testing.T, headers http.Header, want string) {
	t.Helper()
	if got := headers.Get("Allow"); got != want {
		t.Fatalf("Allow = %q, want %q", got, want)
	}
}

func assertAllowContains(t *testing.T, headers http.Header, want string) {
	t.Helper()
	for _, method := range strings.Split(headers.Get("Allow"), ",") {
		if strings.TrimSpace(method) == want {
			return
		}
	}
	t.Fatalf("Allow = %q, want it to contain %q", headers.Get("Allow"), want)
}

func assertCommaHeaderContains(t *testing.T, headers http.Header, name, want string) {
	t.Helper()
	for _, token := range strings.Split(headers.Get(name), ",") {
		if strings.EqualFold(strings.TrimSpace(token), want) {
			return
		}
	}
	t.Fatalf("%s = %q, want it to contain %q", name, headers.Get(name), want)
}
