import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { sanitizeHtml } from "@/lib/safe-html";
import {
  methodologyOptions,
  methodologyPageOptions,
} from "@/lib/queries/methodology";

export function MethodologyPage() {
  const { slug } = useParams<{ slug: string }>();

  const indexQuery = useQuery({
    ...methodologyOptions(),
    enabled: !slug,
  });

  const pageQuery = useQuery({
    ...methodologyPageOptions(slug ?? ""),
    enabled: !!slug,
  });

  const query = slug ? pageQuery : indexQuery;

  return (
    <div className="container py-10">
      <div className="mb-4">
        {slug && (
          <Button asChild variant="ghost" size="sm">
            <Link to="/methodology">
              <ChevronLeft className="mr-1 h-4 w-4" />
              Methodology index
            </Link>
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="pt-6">
          {query.isLoading && <Skeleton className="h-72 w-full" />}
          {query.isError && (
            <p className="py-10 text-center text-sm text-muted-foreground">
              Could not load methodology page.
            </p>
          )}
          {!query.isLoading && !query.isError && !slug && indexQuery.data && (
            <div className="space-y-6">
              <header className="space-y-3">
                <h1 className="text-3xl font-semibold tracking-tight text-foreground">
                  Methodology
                </h1>
                <p className="max-w-[68ch] text-[15px] leading-7 text-muted-foreground">
                  Every number, headline, and deterministic signal on the site
                  has a documented methodology. These pages explain what each
                  signal means, how to interpret it, and where its limits or
                  false-positive risks are.
                </p>
              </header>

              <div className="divide-y divide-border border-y border-border">
                {indexQuery.data.items.map((item) => (
                  <article key={item.slug} className="py-4">
                    <Link
                      to={`/methodology/${item.slug}`}
                      className="text-lg font-semibold text-foreground hover:text-primary"
                    >
                      {item.title}
                    </Link>
                    {item.summary && (
                      <p className="mt-1 text-sm leading-6 text-muted-foreground">
                        {item.summary}
                      </p>
                    )}
                  </article>
                ))}
              </div>
            </div>
          )}
          {!query.isLoading && !query.isError && slug && pageQuery.data && (
            <article className="space-y-4">
              <header className="space-y-3">
                <h1 className="text-3xl font-semibold tracking-tight text-foreground">
                  {pageQuery.data.title}
                </h1>
                {pageQuery.data.summary && (
                  <p className="max-w-[68ch] text-[15px] leading-7 text-muted-foreground">
                    {pageQuery.data.summary}
                  </p>
                )}
              </header>
              <div
                className="prose prose-slate max-w-none dark:prose-invert"
                dangerouslySetInnerHTML={{
                  __html: sanitizeHtml(pageQuery.data.body),
                }}
              />
            </article>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
