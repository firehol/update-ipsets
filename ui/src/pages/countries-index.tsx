import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { AccentBar } from "@/components/editorial/accent-bar";
import type { CountryIndexEntry } from "@/lib/api-types";
import { formatCountryLabel } from "@/lib/countries";
import { formatNum } from "@/lib/utils";
import { countriesOptions } from "@/lib/queries/entities";

export function CountriesIndexPage() {
  const query = useQuery(countriesOptions());

  const countries = query.data?.countries ?? [];

  return (
    <div className="page-container py-20 md:py-24">
      <AccentBar />
      <div className="eyebrow mt-6 text-muted-foreground">Countries</div>
      <h1 className="display-title mt-4 text-foreground">
        Every country currently attributed by public feeds.
      </h1>
      <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
        Aggregated from the active public country-attribution provider
        {query.data?.provider.label ? `: ${query.data.provider.label}.` : "."}
      </p>

      {query.isLoading ? (
        <div className="mt-16 py-24 text-center text-[13px] text-muted-foreground">
          Loading countries…
        </div>
      ) : query.isError ? (
        <div className="mt-16 border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
          Country index data is temporarily unavailable.
        </div>
      ) : countries.length === 0 ? (
        <div className="mt-16 border border-dashed border-border py-24 text-center text-[13px] text-muted-foreground">
          No countries found.
        </div>
      ) : (
        <div className="mt-12 border border-border">
          <table className="w-full border-collapse text-[13px]">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="px-4 py-3 text-left eyebrow text-muted-foreground">
                  Country
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
              {countries.map((country: CountryIndexEntry, idx) => (
                <tr
                  key={country.code}
                  className={
                    "border-b border-border transition hover:bg-muted/30 " +
                    (idx === countries.length - 1 ? "border-b-0" : "")
                  }
                >
                  <td className="px-4 py-3 align-middle">
                    <Link
                      to={`/countries/${country.code}`}
                      className="font-semibold text-foreground hover:text-primary"
                    >
                      {formatCountryLabel(country.code)}
                    </Link>
                    <div className="mt-0.5 font-mono text-[11px] text-muted-foreground">
                      {country.code}
                    </div>
                  </td>
                  <td className="num px-4 py-3 text-right align-middle font-medium text-foreground">
                    {formatNum(country.feed_count)}
                  </td>
                  <td className="num px-4 py-3 text-right align-middle text-muted-foreground">
                    {formatNum(country.attributed_ips)}
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
