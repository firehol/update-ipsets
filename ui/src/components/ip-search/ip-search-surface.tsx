import { useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Link,
  useLocation,
  useNavigate,
  useSearchParams,
} from "react-router-dom";
import { Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { ipSearchOptions } from "@/lib/queries/search";
import { IPSearchResults } from "./ip-search-results";

type SearchScope = { kind: "global" } | { kind: "feed"; feedName: string };

type SearchVariant = "hero" | "section" | "header";

export function IPSearchSurface({
  scope,
  variant,
  eyebrow,
  title,
  description,
  placeholder,
  syncToUrl = false,
  syncHash,
  showClear = false,
  initialValue = "",
}: {
  scope: SearchScope;
  variant: SearchVariant;
  eyebrow?: string;
  title?: string;
  description?: string;
  placeholder?: string;
  syncToUrl?: boolean;
  syncHash?: string;
  showClear?: boolean;
  initialValue?: string;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const urlValue = syncToUrl ? (searchParams.get("ip") ?? "") : "";
  const [draftState, setDraftState] = useState(() => ({
    urlValue,
    value: urlValue || initialValue,
    touched: false,
  }));
  const [submitted, setSubmitted] = useState(
    syncToUrl ? "" : initialValue.trim(),
  );
  const initialSeed = initialValue.trim();
  const seededDraft =
    !draftState.touched && urlValue.trim() === "" ? initialSeed : "";
  const draft =
    syncToUrl && draftState.urlValue !== urlValue
      ? urlValue
      : draftState.value || seededDraft;

  const submittedIP = syncToUrl ? urlValue.trim() : submitted.trim();

  const query = useQuery({
    ...ipSearchOptions(submittedIP, scope, true),
    enabled: submittedIP !== "",
  });

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const ip = draft.trim();
    if (syncToUrl) {
      const next = new URLSearchParams(searchParams);
      if (ip) next.set("ip", ip);
      else next.delete("ip");
      navigate({
        pathname: location.pathname,
        hash: syncHash ? `#${syncHash}` : location.hash,
        search: next.toString() ? `?${next.toString()}` : "",
      });
      return;
    }
    if (!ip) {
      setSubmitted("");
      return;
    }
    setSubmitted(ip);
  };

  const clearSearch = () => {
    setDraftState({ urlValue: "", value: "", touched: true });
    if (syncToUrl) {
      const next = new URLSearchParams(searchParams);
      next.delete("ip");
      navigate({
        pathname: location.pathname,
        hash: syncHash ? `#${syncHash}` : location.hash,
        search: next.toString() ? `?${next.toString()}` : "",
      });
      return;
    }
    setSubmitted("");
  };

  const isDisplay = variant === "hero" || variant === "header";
  const hasResults = submittedIP !== "";

  return (
    <div className={rootClass(variant)}>
      {(eyebrow || title || description) && variant !== "header" && (
        <div className="space-y-2">
          {eyebrow && (
            <div
              className={cn(
                "eyebrow",
                isDisplay ? "text-display-muted" : "text-muted-foreground",
              )}
            >
              {eyebrow}
            </div>
          )}
          {title && (
            <h2
              className={cn(
                "text-2xl font-semibold tracking-tight",
                isDisplay ? "text-display-fg" : "text-foreground",
              )}
            >
              {title}
            </h2>
          )}
          {description && (
            <p
              className={cn(
                "max-w-[62ch] text-sm leading-relaxed",
                isDisplay ? "text-display-muted" : "text-muted-foreground",
              )}
            >
              {description}
            </p>
          )}
        </div>
      )}

      <form
        onSubmit={onSubmit}
        className={cn(
          "grid gap-3",
          variant === "header" && "flex items-center gap-2",
        )}
      >
        <label
          className={cn("relative block", variant === "header" && "min-w-0 flex-1")}
        >
          <Search
            className={cn(
              "pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2",
              variant === "header" ? "h-3.5 w-3.5" : "h-4 w-4",
              isDisplay ? "text-display-muted" : "text-muted-foreground",
            )}
          />
          <input
            type="search"
            inputMode="numeric"
            aria-label={
              scope.kind === "global"
                ? "Search IP address across all feeds"
                : `Search IP address inside ${scope.feedName}`
            }
            placeholder={placeholder ?? "Search any IPv4 address"}
            value={draft}
            onChange={(event) =>
              setDraftState({
                urlValue,
                value: event.target.value,
                touched: true,
              })
            }
            className={inputClass(variant)}
          />
        </label>
        <div className={actionRowClass(variant)}>
          <button type="submit" className={submitClass(variant)}>
            Search IP
          </button>
          {showClear && hasResults && (
            <button
              type="button"
              onClick={clearSearch}
              className={secondaryActionClass(variant)}
            >
              Clear
            </button>
          )}
          {variant === "hero" && scope.kind === "global" && (
            <Link to="/#explorer" className={secondaryActionClass(variant)}>
              Browse feeds
            </Link>
          )}
          {variant === "section" && scope.kind === "feed" && hasResults && (
            <Link
              to={`/?ip=${encodeURIComponent(submittedIP)}`}
              className={secondaryActionClass(variant)}
            >
              Search all feeds
            </Link>
          )}
        </div>
      </form>

      {variant === "header"
        ? hasResults && (
            <div className={headerResultsClass()}>
              <IPSearchResults
                ip={submittedIP}
                scope={scope.kind}
                result={query.data}
                loading={query.isLoading}
                error={
                  query.error instanceof Error ? query.error.message : undefined
                }
                variant={variant}
              />
            </div>
          )
        : hasResults && (
            <div className="pt-2">
              <IPSearchResults
                ip={submittedIP}
                scope={scope.kind}
                result={query.data}
                loading={query.isLoading}
                error={
                  query.error instanceof Error ? query.error.message : undefined
                }
                variant={variant}
              />
            </div>
          )}
    </div>
  );
}

function rootClass(variant: SearchVariant) {
  return cn(
    "relative grid gap-4",
    variant === "hero" &&
      "border border-display-border bg-white/[0.03] px-5 py-5 backdrop-blur-sm",
    variant === "section" && "border border-border bg-card px-5 py-5",
    variant === "header" && "relative w-full min-w-0",
  );
}

function inputClass(variant: SearchVariant) {
  const isDisplay = variant === "hero" || variant === "header";
  return cn(
    "w-full border pl-10 pr-4 focus:outline-none",
    variant === "header"
      ? "h-9 rounded-sm text-[13px]"
      : "h-12 rounded-md text-sm",
    isDisplay
      ? "border-display-border bg-white/[0.05] text-display-fg placeholder:text-display-muted focus:border-primary/60"
      : "border-border bg-background text-foreground placeholder:text-muted-foreground focus:border-primary/60",
  );
}

function actionRowClass(variant: SearchVariant) {
  return cn(
    "flex gap-3",
    variant === "header" && "shrink-0 flex-nowrap justify-end gap-2",
    variant === "hero" && "sm:flex-row",
    variant !== "header" && "flex-col sm:flex-row",
  );
}

function submitClass(variant: SearchVariant) {
  return cn(
    "inline-flex items-center justify-center whitespace-nowrap font-semibold transition-colors",
    variant === "header"
      ? "h-9 rounded-sm px-4 text-xs"
      : "h-11 rounded-md px-5 text-sm",
    "bg-primary text-primary-foreground hover:bg-primary/90",
  );
}

function secondaryActionClass(variant: SearchVariant) {
  const isDisplay = variant === "hero" || variant === "header";
  return cn(
    "inline-flex items-center justify-center whitespace-nowrap border font-semibold transition-colors",
    variant === "header"
      ? "h-9 rounded-sm px-4 text-xs"
      : "h-11 rounded-md px-5 text-sm",
    isDisplay
      ? "border-display-border text-display-fg hover:border-primary/60 hover:text-primary"
      : "border-border text-foreground hover:border-primary hover:text-primary",
  );
}

function headerResultsClass() {
  return "absolute right-0 top-full z-30 mt-2 w-[28rem] max-w-[calc(100vw-2rem)] border border-display-border bg-display px-4 py-4 shadow-2xl";
}
