package engine

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	sitemapXMLNS   = "http://www.sitemaps.org/schemas/sitemap/0.9"
	maxSitemapURLs = 45000
)

type sitemapURLEntry struct {
	Loc string `xml:"loc"`
}

type sitemapURLSet struct {
	XMLName xml.Name          `xml:"urlset"`
	XMLNS   string            `xml:"xmlns,attr"`
	URLs    []sitemapURLEntry `xml:"url"`
}

type sitemapIndexEntry struct {
	Loc string `xml:"loc"`
}

type sitemapIndex struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	XMLNS    string              `xml:"xmlns,attr"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

func (e *Engine) writePublicMetadataFiles(outDir string, outputNames []string) ([]string, error) {
	return e.writePublicMetadataFilesWithSnapshot(e.operationSnapshot(), outDir, outputNames)
}

func (e *Engine) writePublicMetadataFilesWithSnapshot(snap operationSnapshot, outDir string, outputNames []string) ([]string, error) {
	siteBase := publicSiteBaseURLForRuntime(snap.runtime)
	feedPrefix := publicFeedURLPrefixForRuntime(snap.runtime, siteBase)
	files, err := e.writeSitemapFilesWithSnapshot(snap, outDir, siteBase, feedPrefix, outputNames)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(outDir, "robots.txt"), []byte(renderRobotsTXT(siteBase)), generatedFileMode); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(outDir, "llms.txt"), []byte(renderLLMSTXT(siteBase, feedPrefix, outputNames)), generatedFileMode); err != nil {
		return nil, err
	}
	files = append(files, "robots.txt", "llms.txt")
	return files, nil
}

func (e *Engine) writeSitemapFiles(outDir, siteBase, feedPrefix string, outputNames []string) ([]string, error) {
	return e.writeSitemapFilesWithSnapshot(e.operationSnapshot(), outDir, siteBase, feedPrefix, outputNames)
}

func (e *Engine) writeSitemapFilesWithSnapshot(snap operationSnapshot, outDir, siteBase, feedPrefix string, outputNames []string) ([]string, error) {
	const indexName = "sitemap.xml"
	files := []string{indexName}
	if siteBase == "" {
		payload, err := marshalSitemapIndex(nil)
		if err != nil {
			return nil, err
		}
		if err := writeFileAtomic(filepath.Join(outDir, indexName), payload, generatedFileMode); err != nil {
			return nil, err
		}
		if err := removeStaleSitemapShards(outDir, files); err != nil {
			return nil, err
		}
		return files, nil
	}

	shards := []struct {
		name string
		urls []string
	}{
		{name: "sitemap-pages.xml", urls: publicPageSitemapURLs(siteBase)},
		{name: "sitemap-feeds.xml", urls: publicFeedSitemapURLs(feedPrefix, outputNames)},
		{name: "sitemap-countries.xml", urls: e.publicCountrySitemapURLsWithSnapshot(snap, siteBase, outDir)},
		{name: "sitemap-maintainers.xml", urls: e.publicMaintainerSitemapURLsWithSnapshot(snap, siteBase)},
	}
	for _, shard := range shards {
		if err := writeSitemapURLSet(filepath.Join(outDir, shard.name), shard.urls); err != nil {
			return nil, err
		}
		files = append(files, shard.name)
	}
	for i, urls := range chunkStrings(e.publicASNSitemapURLsWithSnapshot(snap, siteBase, outDir), maxSitemapURLs) {
		name := fmt.Sprintf("sitemap-asns-%04d.xml", i+1)
		if err := writeSitemapURLSet(filepath.Join(outDir, name), urls); err != nil {
			return nil, err
		}
		files = append(files, name)
	}

	indexEntries := make([]string, 0, len(files)-1)
	for _, name := range files[1:] {
		indexEntries = append(indexEntries, joinPublicURL(siteBase, name))
	}
	payload, err := marshalSitemapIndex(indexEntries)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(filepath.Join(outDir, indexName), payload, generatedFileMode); err != nil {
		return nil, err
	}
	if err := removeStaleSitemapShards(outDir, files); err != nil {
		return nil, err
	}
	return files, nil
}

func publicPageSitemapURLs(siteBase string) []string {
	urls := make([]string, 0, 5)
	for _, path := range []string{"", "countries", "asns", "maintainers", "methodology"} {
		urls = append(urls, joinPublicURL(siteBase, path))
	}
	return urls
}

func publicFeedSitemapURLs(feedPrefix string, outputNames []string) []string {
	if feedPrefix == "" {
		return nil
	}
	urls := make([]string, 0, len(outputNames))
	for _, name := range outputNames {
		urls = append(urls, joinPublicURL(feedPrefix, name))
	}
	slices.Sort(urls)
	return urls
}

func (e *Engine) publicCountrySitemapURLs(siteBase, outDir string) []string {
	return e.publicCountrySitemapURLsWithSnapshot(e.operationSnapshot(), siteBase, outDir)
}

func (e *Engine) publicCountrySitemapURLsWithSnapshot(snap operationSnapshot, siteBase, outDir string) []string {
	if index := e.loadCountryIndexForSitemapWithSnapshot(snap, outDir); index != nil {
		return countrySitemapURLsFromIndex(siteBase, index)
	}
	index, err := e.buildCountryIndexWithSnapshot(snap, newEntityOutputViewWithRuntime(e, snap.runtime, outDir))
	if err != nil || index == nil {
		return nil
	}
	return countrySitemapURLsFromIndex(siteBase, index)
}

func countrySitemapURLsFromIndex(siteBase string, index *CountryIndexPayload) []string {
	if index == nil {
		return nil
	}
	urls := make([]string, 0, len(index.Countries))
	for _, country := range index.Countries {
		code := strings.ToUpper(strings.TrimSpace(country.Code))
		if code == "" {
			continue
		}
		urls = append(urls, joinPublicURL(siteBase, "countries/"+code))
	}
	slices.Sort(urls)
	return urls
}

func (e *Engine) publicASNSitemapURLs(siteBase, outDir string) []string {
	return e.publicASNSitemapURLsWithSnapshot(e.operationSnapshot(), siteBase, outDir)
}

func (e *Engine) publicASNSitemapURLsWithSnapshot(snap operationSnapshot, siteBase, outDir string) []string {
	if index := e.loadASNIndexForSitemapWithSnapshot(snap, outDir); index != nil {
		return asnSitemapURLsFromIndex(siteBase, index)
	}
	index, err := e.buildASNIndexWithSnapshot(snap, newEntityOutputViewWithRuntime(e, snap.runtime, outDir))
	if err != nil || index == nil {
		return nil
	}
	return asnSitemapURLsFromIndex(siteBase, index)
}

func asnSitemapURLsFromIndex(siteBase string, index *ASNIndexPayload) []string {
	if index == nil {
		return nil
	}
	urls := make([]string, 0, len(index.ASNs))
	for _, asn := range index.ASNs {
		if asn.ASN == 0 {
			continue
		}
		urls = append(urls, joinPublicURL(siteBase, fmt.Sprintf("asns/%d", asn.ASN)))
	}
	slices.Sort(urls)
	return urls
}

func (e *Engine) loadCountryIndexForSitemap(outDir string) *CountryIndexPayload {
	return e.loadCountryIndexForSitemapWithSnapshot(e.operationSnapshot(), outDir)
}

func (e *Engine) loadCountryIndexForSitemapWithSnapshot(snap operationSnapshot, outDir string) *CountryIndexPayload {
	for _, candidate := range sitemapIndexCandidatePaths(outDir, outputDirForRuntime(snap.runtime), e.publicCountryIndexRelPath()) {
		data, err := readFileInRoot(candidate.rootDir, candidate.rel)
		if err != nil {
			continue
		}
		var payload CountryIndexPayload
		if err := json.Unmarshal(data, &payload); err == nil {
			return &payload
		}
	}
	return nil
}

func (e *Engine) loadASNIndexForSitemap(outDir string) *ASNIndexPayload {
	return e.loadASNIndexForSitemapWithSnapshot(e.operationSnapshot(), outDir)
}

func (e *Engine) loadASNIndexForSitemapWithSnapshot(snap operationSnapshot, outDir string) *ASNIndexPayload {
	for _, candidate := range sitemapIndexCandidatePaths(outDir, outputDirForRuntime(snap.runtime), e.publicASNIndexRelPath()) {
		data, err := readFileInRoot(candidate.rootDir, candidate.rel)
		if err != nil {
			continue
		}
		var payload ASNIndexPayload
		if err := json.Unmarshal(data, &payload); err == nil {
			return &payload
		}
	}
	return nil
}

func sitemapIndexCandidatePaths(stageDir, liveDir, rel string) []rootedCandidatePath {
	paths := make([]rootedCandidatePath, 0, 2)
	seen := map[string]struct{}{}
	for _, dir := range []string{stageDir, liveDir} {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		key := filepath.Join(dir, rel)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, rootedCandidatePath{rootDir: dir, rel: rel})
	}
	return paths
}

func (e *Engine) publicMaintainerSitemapURLs(siteBase string) []string {
	return e.publicMaintainerSitemapURLsWithSnapshot(e.operationSnapshot(), siteBase)
}

func (e *Engine) publicMaintainerSitemapURLsWithSnapshot(snap operationSnapshot, siteBase string) []string {
	index, err := e.MaintainerIndexWithSnapshot(snap, nil)
	if err != nil || index == nil {
		return nil
	}
	urls := make([]string, 0, len(index.Maintainers))
	for _, maintainer := range index.Maintainers {
		slug := strings.TrimSpace(maintainer.Slug)
		if slug == "" {
			continue
		}
		urls = append(urls, joinPublicURL(siteBase, "maintainers/"+slug))
	}
	slices.Sort(urls)
	return urls
}

func writeSitemapURLSet(path string, urls []string) error {
	payload, err := marshalSitemapURLSet(urls)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, payload, generatedFileMode)
}

func marshalSitemapURLSet(urls []string) ([]byte, error) {
	entries := make([]sitemapURLEntry, 0, len(urls))
	for _, u := range urls {
		if strings.TrimSpace(u) == "" {
			continue
		}
		entries = append(entries, sitemapURLEntry{Loc: u})
	}
	payload, err := xml.MarshalIndent(sitemapURLSet{XMLNS: sitemapXMLNS, URLs: entries}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(payload, '\n')...), nil
}

func marshalSitemapIndex(urls []string) ([]byte, error) {
	entries := make([]sitemapIndexEntry, 0, len(urls))
	for _, u := range urls {
		if strings.TrimSpace(u) == "" {
			continue
		}
		entries = append(entries, sitemapIndexEntry{Loc: u})
	}
	payload, err := xml.MarshalIndent(sitemapIndex{XMLNS: sitemapXMLNS, Sitemaps: entries}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(payload, '\n')...), nil
}

func chunkStrings(items []string, size int) [][]string {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	chunks := make([][]string, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func removeStaleSitemapShards(outDir string, generated []string) error {
	stale, err := staleSitemapShardNames(outDir, generated)
	if err != nil {
		return err
	}
	for _, name := range stale {
		path := filepath.Join(outDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func staleSitemapShardNames(outDir string, generated []string) ([]string, error) {
	keep := make(map[string]struct{}, len(generated))
	for _, name := range generated {
		keep[name] = struct{}{}
	}
	matches, err := filepath.Glob(filepath.Join(outDir, "sitemap-*.xml"))
	if err != nil {
		return nil, err
	}
	stale := make([]string, 0)
	for _, path := range matches {
		name := filepath.Base(path)
		if _, ok := keep[name]; ok {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		stale = append(stale, name)
	}
	slices.Sort(stale)
	return stale, nil
}

func renderRobotsTXT(siteBase string) string {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, path := range []string{
		"/api/v1/search",
		"/api/v1/query",
		"/api/v1/compose",
		"/api/v1/client-ip",
		"/api/v1/sets/*/search",
		"/api/v1/ipsets/*/search",
	} {
		b.WriteString("Disallow: ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	b.WriteString("Allow: /\n")
	if siteBase != "" {
		b.WriteString("Sitemap: ")
		b.WriteString(joinPublicURL(siteBase, "sitemap.xml"))
		b.WriteString("\n")
	}
	return b.String()
}

func renderLLMSTXT(siteBase, feedPrefix string, outputNames []string) string {
	link := func(path string) string {
		return joinPublicURL(siteBase, path)
	}

	var b strings.Builder
	b.WriteString("# FireHOL IP Lists\n\n")
	b.WriteString("> Public cybercrime IP feed observatory for discovering, comparing, and consuming maintained IP blocklists.\n\n")
	b.WriteString("This file is a concise, public-only map of the site for AI agents. It links to human pages, public APIs, methodology, and feed artifacts. It does not describe private or operator-only surfaces.\n\n")
	b.WriteString("## Primary Pages\n\n")
	b.WriteString("- [Homepage and feed explorer](" + link("") + "): Main public surface for IP lookup and feed discovery.\n")
	b.WriteString("- [Countries](" + link("countries") + "): Country index for public feed matches.\n")
	b.WriteString("- [ASNs](" + link("asns") + "): ASN index for public feed matches.\n")
	b.WriteString("- [Maintainers](" + link("maintainers") + "): Maintainer index for public feeds.\n")
	b.WriteString("- [Methodology](" + link("methodology") + "): Explanations of feed metrics, health, overlap, retention, geography, ASN attribution, and insights.\n\n")
	b.WriteString("## Public APIs\n\n")
	b.WriteString("- [Service status](" + link("api/v1/status") + "): High-level public service state.\n")
	b.WriteString("- [Categories](" + link("api/v1/categories") + "): Public category registry.\n")
	b.WriteString("- [Feed catalog](" + link("api/v1/sets") + "): Public feed inventory.\n")
	b.WriteString("- [Global IP search](" + link("api/v1/search?ip=1.1.1.1") + "): Query public feed membership for one IP address.\n")
	b.WriteString("- [Countries API](" + link("api/v1/countries") + "): Published country summaries.\n")
	b.WriteString("- [ASNs API](" + link("api/v1/asns") + "): Published ASN summaries.\n")
	b.WriteString("- [Maintainers API](" + link("api/v1/maintainers") + "): Public maintainer summaries.\n")
	b.WriteString("- [Methodology API](" + link("api/v1/methodology") + "): Machine-readable methodology index.\n")
	if len(outputNames) > 0 {
		b.WriteString("- [Compose API example](" + link("api/v1/compose?include="+url.QueryEscape(outputNames[0])+"&format=single") + "): Public feed composition endpoint using include/exclude query parameters.\n")
	}
	b.WriteString("\n")
	b.WriteString("## Feed Surfaces\n\n")
	b.WriteString("- [Legacy feed catalog JSON](" + link("all-ipsets.json") + "): Bash-compatible public feed catalog.\n")
	b.WriteString("- [Public feed API index](" + link("api/v1/sets") + "): Canonical API entry point for feed metadata.\n")
	if len(outputNames) > 0 && feedPrefix != "" {
		name := outputNames[0]
		b.WriteString("- [Example feed detail](" + joinPublicURL(feedPrefix, name) + "): Example public feed page; use the feed catalog for the full list.\n")
	}
	b.WriteString("\n")
	b.WriteString("## Optional\n\n")
	b.WriteString("- [Sitemap](" + link("sitemap.xml") + "): XML sitemap for public pages.\n")
	b.WriteString("- [robots.txt](" + link("robots.txt") + "): Crawler policy and sitemap pointer.\n")
	return b.String()
}

// jsonMarshalTabIndent produces JSON with tab indentation matching the
// bash script's printf-based output.
func jsonMarshalTabIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "\t")
}

func millis(seconds int64) int64 {
	if seconds <= 0 {
		return 0
	}
	return seconds * 1000
}
