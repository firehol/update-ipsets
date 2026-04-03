export function formatRunPhase(phase: string | undefined): string {
  switch (phase) {
    case "preflight":
      return "Preflight";
    case "prefetch":
      return "Prefetch";
    case "sources":
      return "Sources";
    case "geoip":
      return "GeoIP";
    case "bogons":
      return "Bogons";
    case "critical_infrastructure":
      return "Critical infrastructure";
    case "asn":
      return "ASN";
    case "entities":
      return "Entities";
    case "metadata":
      return "Metadata";
    case "insights":
      return "Insights";
    case "publish":
      return "Publish";
    default:
      return "—";
  }
}

export function describeRunPhase(phase: string | undefined): string {
  switch (phase) {
    case "preflight":
      return "Preparing directories, cleanup, and pre-run inputs before the batch starts.";
    case "prefetch":
      return "Probing and downloading feed inputs before feed processing begins.";
    case "sources":
      return "Processing feed sources and injecting dependent derivatives into the same batch.";
    case "geoip":
      return "Refreshing geolocation provider data and per-feed country comparison files.";
    case "bogons":
      return "Rebuilding bogon provider data and per-feed bogon comparison files.";
    case "critical_infrastructure":
      return "Rebuilding critical infrastructure reference-feed overlap files.";
    case "asn":
      return "Refreshing ASN provider data and per-feed ASN comparison files.";
    case "entities":
      return "Precomputing per-feed country/ASN entity sidecars for changed feeds.";
    case "metadata":
      return "Writing per-feed metadata, overlap, and related secondary JSON files.";
    case "insights":
      return "Recomputing derived insights from the per-feed secondary data.";
    case "publish":
      return "Publishing staged web outputs and syncing generated files to disk.";
    default:
      return "No batch phase is active.";
  }
}
