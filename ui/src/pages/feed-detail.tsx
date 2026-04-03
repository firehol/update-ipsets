import { useParams, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import { feedOptions } from "@/lib/queries/feed-core";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { FeedHero } from "@/components/feed-detail/hero";
import { SectionEditorial } from "@/components/feed-detail/section-editorial";
import { SectionStatus } from "@/components/feed-detail/section-status";
import { SectionMethod } from "@/components/feed-detail/section-method";
import { SectionListing } from "@/components/feed-detail/section-listing";
import { SectionReputation } from "@/components/feed-detail/section-reputation";
import { SectionSourcesConsulted } from "@/components/feed-detail/section-sources";
import { SectionASN } from "@/components/feed-detail/section-asn";
import { SectionGeo } from "@/components/feed-detail/section-geo";
import { SectionBogons } from "@/components/feed-detail/section-bogons";
import { SectionCriticalInfrastructure } from "@/components/feed-detail/section-critical-infrastructure";
import { SectionBehavior } from "@/components/feed-detail/section-behavior";
import { SectionRetention } from "@/components/feed-detail/section-retention";
import { SectionComparison } from "@/components/feed-detail/section-comparison";
import { SectionInsights } from "@/components/feed-detail/section-insights";
import { SectionSpecs } from "@/components/feed-detail/section-specs";
import { SectionErrorBoundary } from "@/components/feed-detail/section-error-boundary";
import { IPSearchSurface } from "@/components/ip-search/ip-search-surface";

/**
 * Feed detail page. Loads the feed metadata first; every section component
 * pulls its own data with TanStack Query and renders independently. The
 * page composition mirrors the existing detail page section ordering so
 * users moving between the old and the new UI never lose orientation.
 */
export function FeedDetailPage() {
  const { name } = useParams<{ name: string }>();
  const feedName = name ?? "";

  const feedQuery = useQuery({
    ...feedOptions(feedName),
    enabled: feedName !== "",
  });

  if (feedQuery.isLoading) {
    return (
      <div className="container py-10">
        <Skeleton className="h-12 w-64" />
        <Skeleton className="mt-4 h-6 w-96" />
        <Skeleton className="mt-8 h-96 w-full" />
      </div>
    );
  }

  if (feedQuery.isError || !feedQuery.data) {
    return (
      <div className="container py-24 text-center">
        <h1 className="font-display text-3xl font-bold">Feed not found</h1>
        <p className="mt-2 text-muted-foreground">
          No feed named <span className="font-mono">{feedName}</span> in the
          catalog.
        </p>
        <Button asChild className="mt-6">
          <Link to="/#explorer">
            <ChevronLeft className="mr-2 h-4 w-4" />
            Back to the explorer
          </Link>
        </Button>
      </div>
    );
  }

  const feed = feedQuery.data;

  return (
    <div>
      <div className="container pt-4">
        <Button asChild variant="ghost" size="sm">
          <Link to="/#explorer">
            <ChevronLeft className="mr-1 h-4 w-4" />
            Explore
          </Link>
        </Button>
      </div>
      <FeedHero feed={feed} />
      <div className="page-container">
        <section className="pt-8">
          <IPSearchSurface
            scope={{ kind: "feed", feedName }}
            variant="section"
            eyebrow="In-feed lookup"
            title="Search one IP inside this feed."
            description="Use the same search surface here when you already know which feed you want to inspect."
            placeholder={`Search ${feed.name}`}
          />
        </section>
        {/* Editorial opening (long_description as a magazine article) is
            the first thing after the hero. Insights follow with the
            deterministic facts the engine derived. Method is the
            fact-card explanation of how the feed is built. Chart
            sections follow in their previous order, then Listing rules
            (which feels closer to the "operator action" surface), then
            Overlap and Specs, and finally the quiet reputation footer. */}
        <SectionErrorBoundary name="Editorial">
          <SectionEditorial feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Status">
          <SectionStatus feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Insights">
          <SectionInsights feedName={feedName} category={feed.category} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Method">
          <SectionMethod feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Critical Infrastructure">
          <SectionCriticalInfrastructure
            feedName={feedName}
            family={feed.ipv}
            feedIPs={feed.ips}
            category={feed.category}
            isReferenceFeed={feed.used_for?.includes("critical_infrastructure") ?? false}
            isProviderContext={feed.used_for?.includes("provider_context") ?? false}
          />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="AS Composition">
          <SectionASN feedName={feedName} category={feed.category} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Geographic Coverage">
          <SectionGeo feedName={feedName} feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Bogons">
          <SectionBogons feedName={feedName} feedIPs={feed.ips} category={feed.category} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Behavior">
          <SectionBehavior feedName={feedName} feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Retention">
          <SectionRetention feedName={feedName} category={feed.category} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Listing">
          <SectionListing feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Overlap">
          <SectionComparison
            feedName={feedName}
            feedIPs={feed.ips}
            feedHealthClass={feed.health.class}
            category={feed.category}
          />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Specs">
          <SectionSpecs feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Reputation">
          <SectionReputation feed={feed} />
        </SectionErrorBoundary>
        <SectionErrorBoundary name="Sources">
          <SectionSourcesConsulted feed={feed} />
        </SectionErrorBoundary>
      </div>
    </div>
  );
}
