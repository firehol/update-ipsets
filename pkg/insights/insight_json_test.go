package insights

import (
	"encoding/json"
	"testing"
)

func TestSectionJSONRoundTrip(t *testing.T) {
	input := SectionRetention
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var output Section
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if output != input {
		t.Fatalf("round-trip mismatch: got %v want %v", output, input)
	}
}

func TestSectionJSONRejectsUnknownValue(t *testing.T) {
	var output Section
	if err := json.Unmarshal([]byte(`"not-a-real-section"`), &output); err == nil {
		t.Fatal("expected unknown section to fail")
	}
}
