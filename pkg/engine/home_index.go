package engine

// CountryIndexEntry is one row in the countries index payload.
type CountryIndexEntry struct {
	Code          string `json:"code"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// CountryIndexPayload is the response shape for /api/v1/countries.
type CountryIndexPayload struct {
	Provider  HomeSummaryProvider `json:"provider"`
	Countries []CountryIndexEntry `json:"countries"`
}

// ASNIndexEntry is one row in the ASN index payload.
type ASNIndexEntry struct {
	ASN           uint32 `json:"asn"`
	Name          string `json:"name,omitempty"`
	FeedCount     int    `json:"feed_count"`
	AttributedIPs uint64 `json:"attributed_ips"`
}

// ASNIndexPayload is the response shape for /api/v1/asns.
type ASNIndexPayload struct {
	Provider HomeSummaryProvider `json:"provider"`
	ASNs     []ASNIndexEntry     `json:"asns"`
}
