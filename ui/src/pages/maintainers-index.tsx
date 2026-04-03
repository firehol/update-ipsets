import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { AccentBar } from "@/components/editorial/accent-bar";
import type { MaintainerIndexEntry } from "@/lib/api-types";
import { formatNum } from "@/lib/utils";
import { maintainersOptions } from "@/lib/queries/entities";

export function MaintainersIndexPage() {
  const query = useQuery(maintainersOptions());

  const maintainers = query.data?.maintainers ?? [];

  return (
    <div className="page-container py-20 md:py-24">
      <AccentBar />
      <div className="eyebrow mt-6 text-muted-foreground">Maintainers</div>
      <h1 className="display-title mt-4 text-foreground">
        Everyone who publishes a tracked feed.
      </h1>
      <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
        Aggregated from the public maintainer index across every tracked
        feed.
      </p>

      {query.isLoading ? (
        <div className="mt-16 py-24 text-center text-[13px] text-muted-foreground">
          Loading maintainers…
        </div>
      ) : query.isError ? (
        <div className="mt-16 border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
          Maintainer data is temporarily unavailable.
        </div>
      ) : maintainers.length === 0 ? (
        <div className="mt-16 border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
          No maintainers found.
        </div>
      ) : (
        <div className="mt-12 border border-border">
          <table className="w-full border-collapse text-[13px]">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="px-4 py-3 text-left eyebrow text-muted-foreground">
                  Maintainer
                </th>
                <th className="px-4 py-3 text-right eyebrow text-muted-foreground">
                  Feeds
                </th>
                <th className="px-4 py-3 text-right eyebrow text-muted-foreground">
                  IPs
                </th>
                <th className="px-4 py-3 text-right eyebrow text-muted-foreground">
                  Categories
                </th>
              </tr>
            </thead>
            <tbody>
              {maintainers.map((maintainer: MaintainerIndexEntry, idx) => (
                <tr
                  key={maintainer.slug}
                  className={
                    "border-b border-border transition hover:bg-muted/30 " +
                    (idx === maintainers.length - 1 ? "border-b-0" : "")
                  }
                >
                  <td className="px-4 py-3 align-middle">
                    <Link
                      to={`/maintainers/${maintainer.slug}`}
                      className="font-semibold text-foreground hover:text-primary"
                    >
                      {maintainer.name}
                    </Link>
                    {maintainer.url && (
                      <div className="mt-0.5 truncate text-[11px] text-muted-foreground">
                        {formatUrl(maintainer.url)}
                      </div>
                    )}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle font-medium text-foreground">
                    {formatNum(maintainer.feed_count)}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle text-muted-foreground">
                    {formatNum(maintainer.unique_ips)}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle text-muted-foreground">
                    {formatNum(maintainer.categories.length)}
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

function formatUrl(url: string): string {
  try {
    const parsed = new URL(url);
    return parsed.host.replace(/^www\./, "");
  } catch {
    return url;
  }
}
