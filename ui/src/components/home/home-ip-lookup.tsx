import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { AccentBar } from "@/components/editorial/accent-bar";
import { IPSearchSurface } from "@/components/ip-search/ip-search-surface";
import { clientIPOptions } from "@/lib/queries/home";

export function HomeIPLookup() {
  const [searchParams] = useSearchParams();
  const hasURLIP = (searchParams.get("ip") ?? "").trim() !== "";
  const clientIPQuery = useQuery({
    ...clientIPOptions(),
    enabled: !hasURLIP,
    staleTime: 5 * 60 * 1000,
  });

  return (
    <section
      id="ip-lookup"
      className="border-t border-border bg-background py-24 md:py-28"
    >
      <div className="page-container">
        <AccentBar />
        <div className="eyebrow mt-6 text-muted-foreground">Look up an IP</div>
        <h2 className="display-subtitle mt-4 text-foreground">
          Search any IPv4 address.
        </h2>
        <p className="lede mt-5 max-w-[62ch] text-muted-foreground">
          Every tracked feed is searched at once. Country and autonomous system
          context come from the currently configured geo and ASN providers.
          Results are shareable via the URL.
        </p>
        {!hasURLIP && clientIPQuery.data?.ip && (
          <p className="mt-3 text-[13px] text-muted-foreground">
            Detected from your connection:{" "}
            <span className="font-mono text-foreground">
              {clientIPQuery.data.ip}
            </span>
          </p>
        )}

        <div className="mt-10">
          <IPSearchSurface
            scope={{ kind: "global" }}
            variant="section"
            placeholder="e.g. 1.1.1.1"
            initialValue={!hasURLIP ? (clientIPQuery.data?.ip ?? "") : ""}
            syncToUrl
            syncHash="ip-lookup"
            showClear
          />
        </div>
      </div>
    </section>
  );
}
