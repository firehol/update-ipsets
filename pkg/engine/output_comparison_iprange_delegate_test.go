package engine

import (
	"os"
	"strings"
	"testing"
)

func TestComparisonPairsDelegateExactBatchToIPrange(t *testing.T) {
	data, err := os.ReadFile("output_comparison.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "iprange.CompareSourcePairs") {
		t.Fatal("metadata comparison must delegate exact candidate batches to pkg/iprange CompareSourcePairs")
	}
	if strings.Contains(source, "iprange.OverlapCountIterContext") {
		t.Fatal("metadata comparison must not run engine-local pairwise overlap loops")
	}
}
