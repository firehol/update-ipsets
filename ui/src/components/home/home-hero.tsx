import { AccentBar } from "@/components/editorial/accent-bar";
import { formatNum } from "@/lib/utils";

export function HomeHero({
  loading,
  trackedFeeds,
  maintainers,
  categoryCount,
}: {
  loading: boolean;
  trackedFeeds: number;
  maintainers: number;
  categoryCount: number;
}) {
  return (
    <section className="bg-display py-20 text-display-fg md:py-28">
      <div className="page-container">
        <AccentBar />
        <div className="eyebrow mt-6 text-display-muted">
          A public observatory of IP threat feeds
        </div>

        <h1 className="mt-6 max-w-[18ch] font-display text-[2.9rem] font-bold leading-[0.92] tracking-normal text-display-fg md:text-[4.4rem] xl:text-[5.8rem]">
          All Cybercrime
          <br />
          IP Feeds
        </h1>

        <p className="mt-8 max-w-[62ch] text-[17px] leading-relaxed text-display-muted md:text-[19px]">
          FireHOL IP Lists tracks every public IP threat feed it can find,
          normalizes them into one schema, and measures freshness, change,
          provenance, uniqueness, and coverage — so operators, researchers, and
          maintainers can evaluate every feed on the same terms.
        </p>

        <div className="mt-10 flex items-center gap-6 md:mt-12">
          <a
            href="#explorer"
            className="inline-flex items-center gap-3 border-b-2 border-primary pb-1 text-[15px] font-medium tracking-tight text-display-fg transition hover:border-primary/70 hover:text-primary"
          >
            Explore all feeds
            <span aria-hidden="true">↓</span>
          </a>
          <a
            href="#ip-lookup"
            className="text-[15px] font-medium tracking-tight text-display-muted transition hover:text-display-fg"
          >
            Look up an IP
          </a>
        </div>

        <div className="mt-16 grid grid-cols-1 gap-px overflow-hidden border border-display-border bg-display-border sm:grid-cols-2 md:mt-20 md:grid-cols-3">
          <HeroStat
            label="Tracked feeds"
            value={loading ? "—" : formatNum(trackedFeeds)}
            accent
          />
          <HeroStat
            label="Maintainers"
            value={loading ? "—" : formatNum(maintainers)}
          />
          <HeroStat
            label="Threat categories"
            value={loading ? "—" : formatNum(categoryCount)}
          />
        </div>
      </div>
    </section>
  );
}

function HeroStat({
  label,
  value,
  accent = false,
}: {
  label: string;
  value: string;
  accent?: boolean;
}) {
  return (
    <div className="bg-display px-2 py-10 md:py-12">
      <div className="eyebrow text-display-muted">{label}</div>
      <div
        className={
          "num display-title mt-4 " +
          (accent ? "text-primary" : "text-display-fg")
        }
      >
        {value}
      </div>
    </div>
  );
}
