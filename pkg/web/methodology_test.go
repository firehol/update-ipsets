package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func TestMethodologyIndexEndpointIsJSON(t *testing.T) {
	server := newWebHTTPTestServer(t, newHandler(&engine.Engine{}, Options{}, nil))

	status, headers, body := server.get(t, "/api/v1/methodology")
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", status, body)
	}
	if ct := headers.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected content-type: %q", ct)
	}

	var payload methodologyIndexPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, body)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected at least one methodology item")
	}
	if payload.Items[0].Slug == "" || payload.Items[0].Title == "" {
		t.Fatalf("unexpected methodology item: %+v", payload.Items[0])
	}
}

func TestMethodologyPageEndpointIsJSON(t *testing.T) {
	_, ordered := loadMethodologyPages()
	if len(ordered) == 0 {
		t.Fatal("expected embedded methodology pages")
	}
	slug := ordered[0].Slug
	server := newWebHTTPTestServer(t, newHandler(&engine.Engine{}, Options{}, nil))

	status, headers, body := server.get(t, "/api/v1/methodology/"+slug)
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", status, body)
	}
	if ct := headers.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected content-type: %q", ct)
	}

	var payload methodologyPagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid JSON: %v body=%s", err, body)
	}
	if payload.Slug != slug || payload.Title == "" || payload.Body == "" {
		t.Fatalf("unexpected methodology payload: %+v", payload)
	}
}

func TestMethodologyInsightSlugsPresent(t *testing.T) {
	_, ordered := loadMethodologyPages()
	if len(ordered) == 0 {
		t.Fatal("expected embedded methodology pages")
	}

	have := make([]string, 0, len(ordered))
	for _, page := range ordered {
		have = append(have, page.Slug)
	}

	want := []string{
		"bogon-present",
		"churn-high",
		"churn-low",
		"country-concentrated",
		"country-diverse",
		"cross-category-overlap",
		"currently-listed-age-p100",
		"currently-listed-age-p75",
		"independent",
		"infrastructure-present",
		"multiple-retention-policies",
		"observation-wall",
		"permanent-bans",
		"removed-age-p75",
		"single-country",
		"size-variation",
		"subset-of",
	}

	for _, slug := range want {
		if !slices.Contains(have, slug) {
			t.Fatalf("expected methodology page %q to be embedded", slug)
		}
	}
}
