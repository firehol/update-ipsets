package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type webHTTPTestServer struct {
	server *httptest.Server
	client *http.Client
}

func newWebHTTPTestServer(t *testing.T, handler http.Handler) *webHTTPTestServer {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &webHTTPTestServer{
		server: server,
		client: server.Client(),
	}
}

func (s *webHTTPTestServer) do(t *testing.T, method, path string, configure func(*http.Request)) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, s.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if configure != nil {
		configure(req)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, body
}

func (s *webHTTPTestServer) get(t *testing.T, path string) (int, http.Header, []byte) {
	t.Helper()
	return s.do(t, http.MethodGet, path, nil)
}

func (s *webHTTPTestServer) getJSON(t *testing.T, path string, out any) (int, http.Header) {
	t.Helper()
	status, headers, body := s.get(t, path)
	decodeTestJSONInto(t, body, out)
	return status, headers
}
