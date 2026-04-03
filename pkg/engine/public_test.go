package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestSetDataRejectsUnexpectedRawFilePath(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))
	now := time.Now().UTC().Unix()
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.File = "../secret"
	entry.FrequencyMinutes = 60
	entry.SourceDate = now
	entry.ProcessedDate = now
	entry.CheckedDate = now

	if _, _, err := eng.SetData("sample"); err == nil || !strings.Contains(err.Error(), "unexpected materialized file") {
		t.Fatalf("SetData err = %v, want unexpected materialized file", err)
	}
}

func TestSetDataReadsOnlyExactRawFeedFile(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))
	body := []byte("1.2.3.4\n")
	if err := os.WriteFile(filepath.Join(baseDir, "sample.ipset"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Unix()
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.File = "sample.ipset"
	entry.FrequencyMinutes = 60
	entry.SourceDate = now
	entry.ProcessedDate = now
	entry.CheckedDate = now

	got, _, err := eng.SetData("sample")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("SetData body = %q, want %q", got, body)
	}
}

func TestPublicComposeRejectsNonRedistributableExclude(t *testing.T) {
	redistributable := false
	cfg := config.New()
	cfg.Sources["public"] = &config.Source{Name: "public", Frequency: 60}
	cfg.Sources["private"] = &config.Source{Name: "private", Frequency: 60, Redistributable: &redistributable}
	eng := newEngineFixture(t, withConfig(cfg))
	now := time.Now().UTC().Unix()
	entry := eng.state.Entry("public")
	entry.Name = "public"
	entry.File = "public.ipset"
	entry.FrequencyMinutes = 60
	entry.SourceDate = now
	entry.ProcessedDate = now
	entry.CheckedDate = now

	_, err := eng.PublicCompose(t.Context(), []string{"public"}, []string{"private"}, "cidr")
	if err == nil || !strings.Contains(err.Error(), `set "private" is not redistributable`) {
		t.Fatalf("PublicCompose err = %v, want private not redistributable", err)
	}
}

func TestPublicComposeRejectsUnsafeTextFallbackPath(t *testing.T) {
	cfg := config.New()
	cfg.Sources["sample"] = &config.Source{Name: "sample", Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg))
	now := time.Now().UTC().Unix()
	entry := eng.state.Entry("sample")
	entry.Name = "sample"
	entry.File = "../secret"
	entry.FrequencyMinutes = 60
	entry.SourceDate = now
	entry.ProcessedDate = now
	entry.CheckedDate = now

	_, err := eng.PublicCompose(t.Context(), []string{"sample"}, nil, "cidr")
	if err == nil || !strings.Contains(err.Error(), "unexpected materialized file") {
		t.Fatalf("PublicCompose err = %v, want unexpected materialized file", err)
	}
}

func TestComposeRejectsTooManyIncludes(t *testing.T) {
	cfg := config.New()
	eng := newEngineFixture(t, withConfig(cfg))
	names := make([]string, composeMaxInclude+1)
	for i := range names {
		cfg.Sources[fmt.Sprintf("feed_%d", i)] = &config.Source{Name: fmt.Sprintf("feed_%d", i), Frequency: 60}
		names[i] = fmt.Sprintf("feed_%d", i)
	}
	_, err := eng.Compose(t.Context(), names, nil, "cidr")
	if err == nil || !strings.Contains(err.Error(), "too many include sets") {
		t.Fatalf("Compose err = %v, want too many include sets", err)
	}
}

func TestComposeRejectsTooManyExcludes(t *testing.T) {
	cfg := config.New()
	cfg.Sources["alpha"] = &config.Source{Name: "alpha", Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg))
	excludes := make([]string, composeMaxExclude+1)
	for i := range excludes {
		excludes[i] = fmt.Sprintf("excl_%d", i)
	}
	_, err := eng.Compose(t.Context(), []string{"alpha"}, excludes, "cidr")
	if err == nil || !strings.Contains(err.Error(), "too many exclude sets") {
		t.Fatalf("Compose err = %v, want too many exclude sets", err)
	}
}

func TestComposeRespectsContextCancellation(t *testing.T) {
	cfg := config.New()
	cfg.Sources["alpha"] = &config.Source{Name: "alpha", Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := eng.Compose(ctx, []string{"alpha"}, nil, "cidr")
	if err == nil {
		t.Fatal("Compose with cancelled context should fail")
	}
}

func TestComposeRejectsUnsupportedFormat(t *testing.T) {
	baseDir := t.TempDir()
	cfg := config.New()
	cfg.Sources["alpha"] = &config.Source{Name: "alpha", Frequency: 60}
	eng := newEngineFixture(t, withConfig(cfg), withRuntime(func(rt *Runtime) {
		rt.BaseDir = baseDir
	}))
	body := []byte("1.2.3.4/32\n")
	if err := os.WriteFile(filepath.Join(baseDir, "alpha.ipset"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Unix()
	entry := eng.state.Entry("alpha")
	entry.Name = "alpha"
	entry.File = "alpha.ipset"
	entry.FrequencyMinutes = 60
	entry.SourceDate = now
	entry.ProcessedDate = now
	entry.CheckedDate = now
	_, err := eng.Compose(t.Context(), []string{"alpha"}, nil, "xml")
	if err == nil || !strings.Contains(err.Error(), "unsupported compose format") {
		t.Fatalf("Compose err = %v, want unsupported format", err)
	}
}

func TestLimitedWriterStopsAtLimit(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, limit: 16}
	_, err := lw.Write([]byte("0123456789"))
	if err != nil {
		t.Fatalf("first write should succeed: %v", err)
	}
	_, err = lw.Write([]byte("0123456789"))
	if err == nil {
		t.Fatal("second write should exceed limit")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %v, want too large", err)
	}
}

func TestCollectIterCancelsMidIteration(t *testing.T) {
	set := iprange.New("test")
	for i := uint32(0); i < 100_000; i++ {
		lo := i*512 + 1
		_ = set.AddRange(iprange.Range{Lo: lo, Hi: lo})
	}
	set.Optimize()

	iter := set.Iter()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := collectIter(ctx, "cancel_test", iter)
	if err == nil {
		t.Fatal("collectIter with cancelled context should fail")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("error = %v, want cancelled", err)
	}
}

func TestIsServerErrorClassifiesCorrectly(t *testing.T) {
	clientErr := fmt.Errorf("unknown set %q", "bad")
	if IsServerError(clientErr) {
		t.Fatal("client error should not be classified as server error")
	}

	serverErr := wrapServerError(fmt.Errorf("I/O error reading set"))
	if !IsServerError(serverErr) {
		t.Fatal("wrapped I/O error should be classified as server error")
	}

	wrappedServerErr := fmt.Errorf("compose include: %w", serverErr)
	if !IsServerError(wrappedServerErr) {
		t.Fatal("wrapped server error should propagate through error chain")
	}
}
