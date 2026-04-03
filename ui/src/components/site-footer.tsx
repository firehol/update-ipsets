import { Link } from "react-router-dom";

/**
 * Editorial footer. Dark surface, thin top border, restrained type.
 * Mirrors the header chrome so the page feels framed top and bottom.
 */
export function SiteFooter() {
  return (
    <footer className="mt-32 border-t border-display-border bg-display py-16 text-display-muted">
      <div className="page-container">
        <div className="grid gap-10 md:grid-cols-3">
          <div>
            <div className="font-display text-[19px] font-bold text-display-fg">FireHOL</div>
            <div className="mt-1 text-[11px] uppercase tracking-[0.18em] text-display-muted">
              IP Lists
            </div>
            <p className="mt-6 max-w-[36ch] text-[13px] leading-relaxed">
              Threat-intelligence feeds analysed for evolution, geographic coverage,
              retention, comparisons and overlaps.
            </p>
          </div>
          <div>
            <div className="eyebrow text-display-muted">Data providers</div>
            <ul className="mt-4 space-y-2 text-[13px]">
              <li>MaxMind GeoLite2</li>
              <li>IPDeny &middot; IP2Location &middot; IPIP &middot; DB-IP</li>
              <li>iptoasn &middot; CAIDA RouteViews</li>
            </ul>
          </div>
          <div>
            <div className="eyebrow text-display-muted">Project</div>
            <ul className="mt-4 space-y-2 text-[13px]">
              <li>
                <Link className="hover:text-display-fg" to="/">
                  Home
                </Link>
              </li>
              <li>
                <Link className="hover:text-display-fg" to="/#explorer">
                  Explore feeds
                </Link>
              </li>
              <li>
                <Link className="hover:text-display-fg" to="/maintainers">
                  Maintainers
                </Link>
              </li>
              <li>
                <Link className="hover:text-display-fg" to="/countries">
                  Countries
                </Link>
              </li>
              <li>
                <Link className="hover:text-display-fg" to="/asns">
                  ASNs
                </Link>
              </li>
              <li>
                <Link className="hover:text-display-fg" to="/methodology">
                  Methodology
                </Link>
              </li>
              <li>
                <a className="hover:text-display-fg" href="https://github.com/firehol/firehol">
                  github.com/firehol/firehol
                </a>
              </li>
              <li>
                <a className="hover:text-display-fg" href="https://github.com/firehol/iprange">
                  github.com/firehol/iprange
                </a>
              </li>
              <li>
                <a className="hover:text-display-fg" href="mailto:costa@tsaousis.gr">
                  costa@tsaousis.gr
                </a>
              </li>
            </ul>
          </div>
        </div>
        <div className="mt-12 border-t border-display-border pt-6 text-[12px] text-display-muted">
          &copy; 2015&ndash;2026 Costa Tsaousis, for FireHOL. IP lists are property of their
          maintainers.
        </div>
      </div>
    </footer>
  );
}
