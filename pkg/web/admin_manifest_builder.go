package web

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
)

type feedManifestBuilder struct {
	name       string
	src        *config.Source
	cfg        *config.Config
	rt         engine.Runtime
	eng        *engine.Engine
	resp       ManifestResponse
	root       string
	baseDir    string
	webDir     string
	libDir     string
	isDatabase bool
}

func newFeedManifestBuilder(name string, src *config.Source, cfg *config.Config, rt engine.Runtime, eng *engine.Engine) *feedManifestBuilder {
	baseDir := rt.BaseDir
	webDir := rt.WebDir
	if webDir == "" {
		webDir = baseDir
	}
	isDatabase := src.HasUse(config.UseASN) || src.HasUse(config.UseGeoIP)
	return &feedManifestBuilder{
		name:       name,
		src:        src,
		cfg:        cfg,
		rt:         rt,
		eng:        eng,
		resp:       ManifestResponse{Feed: name, ProcessedDate: manifestProcessedDate(eng, cfg, name)},
		root:       daemonRoot(baseDir),
		baseDir:    baseDir,
		webDir:     webDir,
		libDir:     rt.LibDir,
		isDatabase: isDatabase,
	}
}

func (b *feedManifestBuilder) build() ManifestResponse {
	b.addBaseFiles()
	if !b.isDatabase {
		b.addWebSecondaryFiles()
	}
	b.addBinaryFiles()
	b.summarize()
	return b.resp
}

func manifestProcessedDate(eng *engine.Engine, cfg *config.Config, name string) int64 {
	for _, entry := range eng.EntriesSnapshotForConfig(cfg) {
		if entry.Name == name {
			return entry.ProcessedDate
		}
	}
	return 0
}

func (b *feedManifestBuilder) add(kind, provider, path string, required bool) {
	b.resp.Files = append(b.resp.Files, statManifestFile(ManifestFile{
		Rel:      relOrPath(b.root, path),
		Path:     path,
		Kind:     kind,
		Provider: provider,
		Required: required,
	}, b.resp.ProcessedDate))
}

func (b *feedManifestBuilder) addBaseFiles() {
	b.add("enabled", "", filepath.Join(b.baseDir, b.name+".enabled"), false)
	if b.isDatabase {
		b.add("provider_source", "", b.providerSourcePath(), true)
	} else {
		if b.hasRawSourceFile() {
			b.add("raw_source", "", filepath.Join(b.baseDir, b.name+".source"), false)
		}
		b.add("canonical", "", b.eng.FeedBodyPath(b.name), true)
	}
	b.add("setinfo", "", filepath.Join(b.baseDir, b.name+".setinfo"), false)
}

func (b *feedManifestBuilder) providerSourcePath() string {
	if b.src.HasUse(config.UseASN) {
		return filepath.Join(b.libDir, "asn", b.name, "source")
	}
	return filepath.Join(b.libDir, "geolocation", b.name+".source")
}

func (b *feedManifestBuilder) hasRawSourceFile() bool {
	return b.src.URL != "" &&
		b.src.Provenance != config.ProvenanceSecondaryRetention &&
		b.src.Provenance != config.ProvenanceSecondaryMerge
}

func (b *feedManifestBuilder) addWebSecondaryFiles() {
	for _, row := range []struct {
		kind   string
		suffix string
	}{
		{kind: "metadata", suffix: ".json"},
		{kind: "history", suffix: "_history.csv"},
		{kind: "changesets", suffix: "_changesets.csv"},
		{kind: "retention", suffix: "_retention.json"},
		{kind: "comparison", suffix: "_comparison.json"},
		{kind: "insights", suffix: "_insights.json"},
	} {
		b.add(row.kind, "", filepath.Join(b.webDir, b.name+row.suffix), true)
	}
	b.addProviderFanoutFiles()
}

func (b *feedManifestBuilder) addProviderFanoutFiles() {
	for _, provider := range b.cfg.SourcesWithUse(config.UseGeoIP) {
		b.add("geo", provider.Name, filepath.Join(b.webDir, b.name+"_"+provider.Name+".json"), true)
	}
	for _, provider := range b.cfg.SourcesWithUse(config.UseASN) {
		b.add("asn", provider.Name, filepath.Join(b.webDir, b.name+"_asn_"+provider.Name+".json"), true)
	}
	for _, provider := range b.cfg.SourcesWithUse(config.UseBogons) {
		b.add("bogons", provider.Name, filepath.Join(b.webDir, b.name+"_bogons_"+provider.Name+".json"), true)
	}
}

func (b *feedManifestBuilder) addBinaryFiles() {
	if b.libDir == "" || b.isDatabase {
		return
	}
	b.add("binary", "", filepath.Join(b.libDir, b.name, "latest"), true)
	b.addHistorySnapshots()
}

func (b *feedManifestBuilder) addHistorySnapshots() {
	rollupDir := filepath.Join(b.rt.HistoryDir, b.name)
	entries, err := os.ReadDir(rollupDir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".set") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	const maxRollups = 20
	for i, name := range names {
		if i >= maxRollups {
			break
		}
		b.add("history_snapshot", "", filepath.Join(rollupDir, name), false)
	}
}

func (b *feedManifestBuilder) summarize() {
	for i := range b.resp.Files {
		if b.resp.Files[i].Required {
			b.resp.Summary.Required++
		}
		if b.resp.Files[i].Exists {
			b.resp.Summary.Present++
		} else if b.resp.Files[i].Required {
			b.resp.Summary.Missing++
		}
		if b.resp.Files[i].Stale {
			b.resp.Summary.Stale++
		}
	}
	b.resp.Summary.Total = len(b.resp.Files)
}
