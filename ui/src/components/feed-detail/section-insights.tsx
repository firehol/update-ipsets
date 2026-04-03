import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  Clock3,
  Layers,
  Lightbulb,
  Network,
  Sparkles,
  TrendingUp,
  type LucideIcon,
} from "lucide-react";
import { insightsOptions } from "@/lib/queries/feed";
import { useCategoryAccent } from "@/lib/categories";
import type { InsightSection } from "@/lib/api-types";
import { DetailSection } from "./section";

const SECTION_ICON: Record<InsightSection, LucideIcon> = {
  overview: Lightbulb,
  composition: Layers,
  retention: Clock3,
  trends: TrendingUp,
  relationships: Network,
  freshness: Activity,
};

/**
 * Insights section — deterministic facts the engine derives from the
 * feed's data. Each insight gets a small lucide icon for its section
 * kind and a side rail tinted by the feed's category, so the cards
 * read as a family rather than a flat list.
 */
export function SectionInsights({
  feedName,
  category,
}: {
  feedName: string;
  category: string | null | undefined;
}) {
  const insightsQuery = useQuery({
    ...insightsOptions(feedName),
    retry: false,
  });
  const accent = useCategoryAccent(category);

  const insights = insightsQuery.data ?? [];
  if (insightsQuery.isError && (insightsQuery.error as { status?: number })?.status === 404) {
    return null;
  }
  if (!insightsQuery.isLoading && insights.length === 0) return null;

  return (
    <DetailSection
      eyebrow="Insights"
      title="What the data says"
      lede="Deterministic facts the engine derives from this feed's data."
      icon={Sparkles}
      accentColor={accent}
    >
      {insightsQuery.isLoading && <div className="h-32 animate-pulse bg-muted/40" />}
      {insights.length > 0 && (
        <div className="grid grid-cols-1 gap-x-8 gap-y-6 md:grid-cols-2">
          {insights.map((ins) => {
            const Icon = SECTION_ICON[ins.section] ?? Sparkles;
            return (
              <div
                key={ins.code}
                className="border-l-[3px] border-primary py-2 pl-5"
                style={accent ? { borderColor: accent } : undefined}
              >
                <div className="flex items-center gap-2">
                  <Icon
                    className="h-3.5 w-3.5"
                    style={accent ? { color: accent } : undefined}
                    aria-hidden="true"
                  />
                  <div className="eyebrow text-muted-foreground">
                    {ins.section.replace(/_/g, " ")}
                  </div>
                </div>
                <p className="mt-2 text-[17px] leading-snug text-foreground">{ins.headline}</p>
              </div>
            );
          })}
        </div>
      )}
    </DetailSection>
  );
}
