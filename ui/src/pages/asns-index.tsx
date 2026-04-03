import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { AccentBar } from "@/components/editorial/accent-bar";
import type { ASNIndexEntry } from "@/lib/api-types";
import { asnsOptions } from "@/lib/queries/entities";
import { formatNum } from "@/lib/utils";

export function ASNsIndexPage() {
  const query = useQuery(asnsOptions());

  const asns = query.data?.asns ?? [];

  return (
    <div className="page-container py-20 md:py-24">
      <AccentBar />
      <div className="eyebrow mt-6 text-muted-foreground">Autonomous Systems</div>
      <h1 className="display-title mt-4 text-foreground">
        Every ASN currently attributed by public feeds.
      </h1>
      <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
        Aggregated from the active public ASN-attribution provider
        {query.data?.provider.label ? `: ${query.data.provider.label}.` : "."}
      </p>

      {query.isLoading ? (
        <div className="mt-16 py-24 text-center text-[13px] text-muted-foreground">
          Loading ASNs…
        </div>
      ) : query.isError ? (
        <div className="mt-16 border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
          ASN index data is temporarily unavailable.
        </div>
      ) : asns.length === 0 ? (
        <div className="mt-16 border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
          No ASNs found.
        </div>
      ) : (
        <div className="mt-12 border border-border">
          <table className="w-full border-collapse text-[13px]">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="px-4 py-3 text-left eyebrow text-muted-foreground">
                  ASN
                </th>
                <th className="px-4 py-3 text-right eyebrow text-muted-foreground">
                  Feeds
                </th>
                <th className="px-4 py-3 text-right eyebrow text-muted-foreground">
                  Attributed IPs
                </th>
              </tr>
            </thead>
            <tbody>
              {asns.map((row: ASNIndexEntry, idx) => (
                <tr
                  key={row.asn}
                  className={
                    "border-b border-border transition hover:bg-muted/30 " +
                    (idx === asns.length - 1 ? "border-b-0" : "")
                  }
                >
                  <td className="px-4 py-3 align-middle">
                    <Link
                      to={`/asns/${row.asn}`}
                      className="font-semibold text-foreground hover:text-primary"
                    >
                      AS{row.asn}
                    </Link>
                    {row.name && (
                      <div className="mt-0.5 text-[11px] text-muted-foreground">
                        {row.name}
                      </div>
                    )}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle font-medium text-foreground">
                    {formatNum(row.feed_count)}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle text-muted-foreground">
                    {formatNum(row.attributed_ips)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
