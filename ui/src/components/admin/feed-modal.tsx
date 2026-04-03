import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import type { AdminFeed, IntegrityFinding } from "@/lib/api-types";
import { FeedModalDiagnostics } from "@/components/admin/feed-modal-diagnostics";
import { FeedModalHero } from "@/components/admin/feed-modal-hero";
import { FeedModalIdentity } from "@/components/admin/feed-modal-identity";
import { FeedModalManifest } from "@/components/admin/feed-modal-manifest";
import {
  FeedModalContent,
  FeedModalSchedule,
  FeedModalTimeline,
} from "@/components/admin/feed-modal-status-sections";
import { ModalSection } from "@/components/admin/feed-modal-primitives";

export function FeedModal({
  feed,
  integrityFinding,
  publicBaseURL,
  open,
  onClose,
}: {
  feed: AdminFeed | null;
  integrityFinding: IntegrityFinding | undefined;
  publicBaseURL?: string | null;
  open: boolean;
  onClose: () => void;
}) {
  if (!feed) return null;
  return (
    <Sheet modal={false} open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent
        side="right"
        hideOverlay
        aria-describedby={undefined}
        className="h-screen w-[min(1180px,96vw)] overflow-y-auto border-l border-border bg-background p-0 shadow-2xl sm:max-w-none"
      >
        <SheetTitle className="sr-only">{feed.name}</SheetTitle>
        <FeedModalBody
          feed={feed}
          integrityFinding={integrityFinding}
          publicBaseURL={publicBaseURL}
        />
      </SheetContent>
    </Sheet>
  );
}

function FeedModalBody({
  feed,
  integrityFinding,
  publicBaseURL,
}: {
  feed: AdminFeed;
  integrityFinding: IntegrityFinding | undefined;
  publicBaseURL?: string | null;
}) {
  return (
    <div className="divide-y divide-border">
      <FeedModalHero feed={feed} publicBaseURL={publicBaseURL} />
      <FeedModalIdentity feed={feed} />
      <FeedModalSchedule feed={feed} />
      <FeedModalTimeline feed={feed} />
      <FeedModalContent feed={feed} />
      <FeedModalManifest feed={feed} />
      <FeedModalDiagnostics feed={feed} integrityFinding={integrityFinding} />
    </div>
  );
}

export { ModalSection };
