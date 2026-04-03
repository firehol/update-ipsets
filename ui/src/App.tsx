import { lazy, Suspense, type ReactNode } from "react";
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/components/theme-provider";
import { Layout } from "@/components/layout";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  RouteErrorBoundary,
  RouteLoadingFallback,
} from "@/components/route-error-boundary";
import { queryClient } from "@/lib/query-client";

const HomePage = lazy(() =>
  import("@/pages/home").then((mod) => ({ default: mod.HomePage })),
);
const FeedDetailPage = lazy(() =>
  import("@/pages/feed-detail").then((mod) => ({
    default: mod.FeedDetailPage,
  })),
);
const MethodologyPage = lazy(() =>
  import("@/pages/methodology").then((mod) => ({
    default: mod.MethodologyPage,
  })),
);
const CountriesIndexPage = lazy(() =>
  import("@/pages/countries-index").then((mod) => ({
    default: mod.CountriesIndexPage,
  })),
);
const CountryDetailPage = lazy(() =>
  import("@/pages/country-detail").then((mod) => ({
    default: mod.CountryDetailPage,
  })),
);
const ASNsIndexPage = lazy(() =>
  import("@/pages/asns-index").then((mod) => ({
    default: mod.ASNsIndexPage,
  })),
);
const ASNDetailPage = lazy(() =>
  import("@/pages/asn-detail").then((mod) => ({
    default: mod.ASNDetailPage,
  })),
);
const MaintainersIndexPage = lazy(() =>
  import("@/pages/maintainers-index").then((mod) => ({
    default: mod.MaintainersIndexPage,
  })),
);
const MaintainerDetailPage = lazy(() =>
  import("@/pages/maintainer-detail").then((mod) => ({
    default: mod.MaintainerDetailPage,
  })),
);
const AdminPage = lazy(() =>
  import("@/pages/admin").then((mod) => ({ default: mod.AdminPage })),
);
const AdminLayout = lazy(() =>
  import("@/components/admin/admin-layout").then((mod) => ({
    default: mod.AdminLayout,
  })),
);
const NotFoundPage = lazy(() =>
  import("@/pages/not-found").then((mod) => ({
    default: mod.NotFoundPage,
  })),
);

/**
 * Root component. Wires up the top-level providers (TanStack Query for
 * server state, ThemeProvider for dark/light, TooltipProvider so every
 * `<HoverTip>` in the app shares one hover-delay budget, BrowserRouter
 * for navigation) and declares the route table. Every public route
 * renders inside <Layout>; the admin SPA mounts under its own outlet so
 * it can use a different chrome.
 *
 * delayDuration is set to 400ms — a hair under half a second feels
 * snappy without flashing tooltips at the user during fast cursor
 * sweeps. Same value is used for both public site and admin so the
 * hover budget feels uniform across the whole app.
 */
export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <TooltipProvider delayDuration={400} skipDelayDuration={200}>
          <BrowserRouter>
            <RouteRuntimeBoundary>
              <Routes>
                {/* Admin routes use their own layout — full-width,
                  no public chrome, dedicated operator header. */}
                <Route element={<AdminLayout />}>
                  <Route path="/admin/*" element={<AdminPage />} />
                </Route>
                {/* Public catalog routes use the editorial Layout
                  with site header, footer, and feed sidebar. */}
                <Route element={<Layout />}>
                  <Route path="/" element={<HomePage />} />
                  <Route
                    path="/catalog"
                    element={<Navigate to="/#explorer" replace />}
                  />
                  <Route path="/ipsets/:name" element={<FeedDetailPage />} />
                  <Route path="/countries" element={<CountriesIndexPage />} />
                  <Route
                    path="/countries/:code"
                    element={<CountryDetailPage />}
                  />
                  <Route path="/asns" element={<ASNsIndexPage />} />
                  <Route path="/asns/:asn" element={<ASNDetailPage />} />
                  <Route path="/maintainers" element={<MaintainersIndexPage />} />
                  <Route
                    path="/maintainers/:slug"
                    element={<MaintainerDetailPage />}
                  />
                  <Route
                    path="/methodology/:slug"
                    element={<MethodologyPage />}
                  />
                  <Route path="/methodology" element={<MethodologyPage />} />
                  <Route path="*" element={<NotFoundPage />} />
                </Route>
              </Routes>
            </RouteRuntimeBoundary>
          </BrowserRouter>
        </TooltipProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

function RouteRuntimeBoundary({ children }: { children: ReactNode }) {
  const location = useLocation();
  // Key on pathname (not location.key) so that query/hash-only updates
  // — e.g. the explorer search input syncing ?q= on every keystroke —
  // don't remount the whole subtree and blur focused inputs.
  return (
    <RouteErrorBoundary key={location.pathname}>
      <Suspense fallback={<RouteLoadingFallback />}>{children}</Suspense>
    </RouteErrorBoundary>
  );
}
