import { Outlet, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Activity, ExternalLink } from "lucide-react";
import { publicSiteURL } from "@/lib/public-url";
import { Toaster } from "@/components/ui/sonner";
import { ThemeToggle } from "@/components/theme-toggle";
import { adminStatusOptions } from "@/lib/queries/admin";

/**
 * Admin-only layout. Distinct from the public Layout (which
 * wraps the editorial catalog pages) because the operator
 * console needs:
 *
 *   - full viewport width (no 1280px editorial cap)
 *   - no site footer (operators don't need the "About the
 *     blocklists" prose when they're debugging a failing feed)
 *   - minimal header that fits the density of the feeds table
 *   - a persistent daemon-health dot so the operator knows
 *     immediately if the API is unreachable
 *
 * The previous admin UI used public chrome around operator content.
 * This layout owns the admin chrome end-to-end so there is no
 * misalignment.
 */
export function AdminLayout() {
  const statusQuery = useQuery({
    ...adminStatusOptions(),
    retry: false,
  });

  const reachable = !statusQuery.error;
  const running = statusQuery.data?.engine.running ?? false;
  const publicHref = publicSiteURL(
    statusQuery.data ? (statusQuery.data.public_base_url ?? null) : undefined,
    "",
  );

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <header className="sticky top-0 z-40 flex h-14 items-center gap-4 border-b border-border bg-background/95 px-6 backdrop-blur">
        <div className="flex items-center gap-3">
          <span
            className={
              reachable
                ? running
                  ? "text-status-healthy"
                  : "text-foreground/60"
                : "text-destructive"
            }
            aria-label={
              reachable ? (running ? "running" : "idle") : "unreachable"
            }
          >
            <Activity className="h-4 w-4" />
          </span>
          <Link
            to="/admin"
            className="font-semibold tracking-tight text-foreground"
          >
            update-ipsets
          </Link>
          <span className="text-xs text-muted-foreground">operator</span>
        </div>
        <nav className="flex items-center gap-4 text-sm">
          <Link
            to="/admin"
            className="text-muted-foreground transition-colors hover:text-foreground"
          >
            Dashboard
          </Link>
        </nav>
        <div className="ml-auto flex items-center gap-3">
          {publicHref && (
            <a
              href={publicHref}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
            >
              Public site
              <ExternalLink className="h-3 w-3" />
            </a>
          )}
          <ThemeToggle />
        </div>
      </header>

      <main className="flex-1">
        <Outlet />
      </main>

      <Toaster richColors closeButton />
    </div>
  );
}
