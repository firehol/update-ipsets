import { useEffect, useMemo, useState } from "react";
import { Activity, Database, Search, ShieldCheck, Wrench } from "lucide-react";
import type { AdminFeed } from "@/lib/api-types";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from "@/components/ui/command";
import { feedHealth, healthLabel } from "@/lib/admin-format";

const PANEL_TARGETS = [
  {
    id: "admin-current-run-panel",
    label: "Runtime and queues",
    description: "Downloader, processing, and background work",
    icon: Activity,
  },
  {
    id: "admin-integrity-panel",
    label: "Pipeline integrity",
    description: "Settled feed-output integrity findings",
    icon: ShieldCheck,
  },
  {
    id: "admin-entity-integrity-panel",
    label: "Entity artifact integrity",
    description: "Country and ASN artifact health",
    icon: Wrench,
  },
  {
    id: "admin-feeds-table",
    label: "Feed inventory",
    description: "Search, filter, and inspect all feeds",
    icon: Database,
  },
];

export function AdminCommandPalette({
  feeds,
  onFeedClick,
}: {
  feeds: AdminFeed[];
  onFeedClick: (feed: AdminFeed) => void;
}) {
  const [open, setOpen] = useState(false);
  const commandFeeds = useMemo(() => {
    return [...feeds]
      .sort((a, b) => {
        const ah = feedHealth(a);
        const bh = feedHealth(b);
        if (ah !== bh) return ah.localeCompare(bh);
        return a.name.localeCompare(b.name);
      })
      .slice(0, 80);
  }, [feeds]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setOpen((current) => !current);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  const jumpToPanel = (id: string) => {
    setOpen(false);
    window.requestAnimationFrame(() => {
      document.getElementById(id)?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="inline-flex h-9 items-center gap-2 rounded-sm border border-border bg-card px-3 text-xs text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
      >
        <Search className="h-3.5 w-3.5" />
        Command
        <span className="rounded border border-border px-1.5 py-0.5 font-mono text-[10px]">
          Ctrl K
        </span>
      </button>
      <CommandDialog open={open} onOpenChange={setOpen}>
        <CommandInput placeholder="Jump to a panel or open a feed..." />
        <CommandList className="max-h-[520px]">
          <CommandEmpty>No matching admin command.</CommandEmpty>
          <CommandGroup heading="Panels">
            {PANEL_TARGETS.map((target) => {
              const Icon = target.icon;
              return (
                <CommandItem
                  key={target.id}
                  value={`${target.label} ${target.description}`}
                  onSelect={() => jumpToPanel(target.id)}
                >
                  <Icon className="text-muted-foreground" />
                  <span className="flex min-w-0 flex-col">
                    <span>{target.label}</span>
                    <span className="truncate text-xs text-muted-foreground">
                      {target.description}
                    </span>
                  </span>
                </CommandItem>
              );
            })}
          </CommandGroup>
          <CommandGroup heading="Feeds">
            {commandFeeds.map((feed) => (
              <CommandItem
                key={feed.name}
                value={`${feed.name} ${feed.category} ${feed.maintainer ?? ""} ${feed.last_error ?? ""}`}
                onSelect={() => {
                  setOpen(false);
                  onFeedClick(feed);
                }}
              >
                <span className="font-mono text-xs">{feed.name}</span>
                <span className="truncate text-xs text-muted-foreground">
                  {feed.category || feed.kind}
                </span>
                <CommandShortcut>{healthLabel(feedHealth(feed))}</CommandShortcut>
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </CommandDialog>
    </>
  );
}
