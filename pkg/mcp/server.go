package mcp

import (
	"net/http"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type FeedCatalog interface {
	FindFeeds(filters FeedFilters) ([]FeedHit, error)
}

type MarkdownStore interface {
	ReadMarkdown(entityType, name string) ([]byte, error)
}

type Server struct {
	handler       http.Handler
	catalog       FeedCatalog
	markdown      MarkdownStore
	filterOptions FeedFilterOptions
}

func NewServer(catalog FeedCatalog, markdown MarkdownStore) *Server {
	s := &Server{
		catalog:       catalog,
		markdown:      markdown,
		filterOptions: feedFilterOptionsFromCatalog(catalog),
	}

	mcpServer := server.NewMCPServer(
		"iplists.firehol.org",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	mcpServer.AddTools(
		server.ServerTool{
			Tool:    s.findFeedsTool(),
			Handler: s.handleFindFeeds,
		},
		server.ServerTool{
			Tool:    s.fetchAnalysisTool(),
			Handler: s.handleFetchAnalysis,
		},
	)

	s.handler = server.NewStreamableHTTPServer(mcpServer,
		server.WithHeartbeatInterval(30*time.Second),
	)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func readOnly() mcpgo.ToolOption {
	return mcpgo.WithReadOnlyHintAnnotation(true)
}

func optionalEnumString(name, description string, values []string) mcpgo.ToolOption {
	opts := []mcpgo.PropertyOption{
		mcpgo.Description(description),
	}
	if len(values) > 0 {
		opts = append(opts, mcpgo.Enum(values...))
	}
	return mcpgo.WithString(name, opts...)
}

func (s *Server) findFeedsTool() mcpgo.Tool {
	options := s.filterOptions
	return mcpgo.NewTool("find_feeds",
		readOnly(),
		mcpgo.WithDescription(`Discover threat intelligence IP blocklists (also called IP feeds or IP sets) by searching and filtering public feed metadata. Each feed is a curated list of IP addresses associated with a specific threat type (malware, spam, attacks, etc.) maintained by security researchers and organizations. Returns matching feeds as markdown: a compact table for structured columns, followed by per-feed maintainer and description sections. All filters are optional and can be combined.

Common use cases:
- Search for feeds by feed name, maintainer, or description text (e.g. search="ssh brute force")
- Find feeds for a specific threat type (e.g. category="anonymizers" for Tor/proxy lists)
- Find feeds from a specific maintainer (e.g. maintainer="firehol" or maintainer="spamhaus")
- Find primary (original source) vs derived (merged/retained) feeds
- Find healthy, actively maintained feeds
- Find feeds above a certain size (e.g. size_min=10000 for feeds with 10k+ IPs)`),
		mcpgo.WithString("search",
			mcpgo.Description("Case-insensitive full-text search across feed names, maintainer names, and feed descriptions. Multiple terms must all match."),
		),
		optionalEnumString("category",
			"Threat category the feed covers. Values come from the active public feed catalog.",
			options.Categories),
		optionalEnumString("maintainer",
			"Feed maintainer name from the active public feed catalog. Exact enum values are preferred; filtering remains case-insensitive.",
			options.Maintainers),
		optionalEnumString("provenance",
			"Origin of the feed data: primary, secondary_upstream, secondary_merge, or secondary_retention.",
			options.Provenance),
		optionalEnumString("health",
			"Operational health of the feed.",
			options.Health),
		optionalEnumString("freshness",
			"Time since the feed was last updated.",
			options.Freshness),
		optionalEnumString("cadence",
			"How often the feed is typically updated.",
			options.Cadence),
		optionalEnumString("uniqueness",
			"How much of this feed's IPs are unique to it compared to other feeds.",
			options.Uniqueness),
		optionalEnumString("license",
			"License type from the active public feed catalog.",
			options.Licenses),
		optionalEnumString("redistributable",
			"Whether the feed data can be freely redistributed.",
			options.Redistributable),
		optionalEnumString("critical",
			"Critical infrastructure reference tier.",
			options.Critical),
		mcpgo.WithNumber("size_min",
			mcpgo.Description("Minimum number of unique IP addresses in the feed"),
		),
		mcpgo.WithNumber("size_max",
			mcpgo.Description("Maximum number of unique IP addresses in the feed"),
		),
	)
}

func (s *Server) fetchAnalysisTool() mcpgo.Tool {
	return mcpgo.NewTool("fetch_analysis",
		readOnly(),
		mcpgo.WithDescription(`Fetch a detailed markdown analysis page for a specific feed, country, ASN, or maintainer from iplists.firehol.org. The page includes statistics, historical trends, size over time, geographic distribution, overlap with other feeds, and other analytical data.

Use find_feeds first to discover available feeds, then use this tool to get the full analysis for any entity.

Entity types:
- "feed": analysis of a specific IP blocklist (size, history, overlaps, geography, retention)
- "country": which feeds cover IPs in a given country
- "asn": which feeds cover IPs in a given Autonomous System Number
- "maintainer": overview of all feeds from a specific maintainer`),
		mcpgo.WithString("type",
			mcpgo.Description("Type of entity to analyze"),
			mcpgo.Required(),
			mcpgo.Enum("feed", "country", "asn", "maintainer"),
		),
		mcpgo.WithString("name",
			mcpgo.Description(`Entity identifier. Format depends on type:
- feed: the feed name as returned by find_feeds (e.g. "firehol_level1", "tor_exits", "spamhaus_drop")
- country: ISO 3166-1 alpha-2 country code (e.g. "US", "CN", "DE", "BR")
- asn: ASN number with or without "AS" prefix (e.g. "13335" or "AS13335" for Cloudflare)
- maintainer: maintainer slug (e.g. "firehol", "spamhaus", "emergingthreats")`),
			mcpgo.Required(),
		),
	)
}
