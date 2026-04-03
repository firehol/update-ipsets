package web

import (
	"encoding/json"
	"testing"
)

type criticalProviderPayload struct {
	Provider struct {
		Name string `json:"name"`
	} `json:"provider"`
}

func decodeTestJSON[T any](t *testing.T, body []byte) T {
	t.Helper()
	var out T
	decodeTestJSONInto(t, body, &out)
	return out
}

func decodeTestJSONInto(t *testing.T, body []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("decode JSON body: %v\nbody=%s", err, body)
	}
}

func findAdminFeed(feeds []adminFeed, name string) (adminFeed, bool) {
	for _, feed := range feeds {
		if feed.Name == name {
			return feed, true
		}
	}
	return adminFeed{}, false
}
