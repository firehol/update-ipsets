import { expect, test, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { CurrentRunPanel } from "./current-run";
import { sampleAdminFeed, sampleAdminStatus } from "@/test/fixtures";
import { renderUI } from "@/test/render";

test("shows active processing progress with work size, completion, and rate", () => {
  const feed = sampleAdminFeed({ name: "dronebl_anonymizers" });
  const status = sampleAdminStatus({
    engine: {
      running: true,
      current_phase: "sources",
      active_operations: [
        {
          operation: "retention.reconcile_cohorts",
          phase: "sources",
          feed: "dronebl_anonymizers",
          stage: "scan",
          unit: "files",
          current: 4,
          total: 10,
          completion_pct: 40,
          rate_per_second: 2.5,
          started_at: "2026-06-19T19:11:20Z",
        },
      ],
    },
    queues: {
      processing_active: [
        {
          name: "dronebl_anonymizers",
          reason: "scheduled_due",
          started_at: "2026-06-19T19:11:20Z",
        },
      ],
    },
  });

  renderUI(
    <CurrentRunPanel status={status} feeds={[feed]} onFeedClick={vi.fn()} />,
  );

  expect(screen.getByText("dronebl_anonymizers")).toBeVisible();
  expect(screen.getByText("Scanning retention cohorts")).toBeVisible();
  expect(screen.getByText("40%")).toBeVisible();
  expect(screen.getByText("4 / 10 files")).toBeVisible();
  expect(screen.getByText("2.5 files/s")).toBeVisible();
  expect(
    screen.getByRole("progressbar", {
      name: /scanning retention cohorts progress/i,
    }),
  ).toHaveAttribute("aria-valuenow", "40");
});

test("shows phase-level progress when no feed-specific operation is active", () => {
  const status = sampleAdminStatus({
    engine: {
      running: true,
      current_phase: "metadata",
      active_operations: [
        {
          operation: "metadata.write_per_feed_outputs",
          phase: "metadata",
          stage: "write",
          unit: "feeds",
          current: 5,
          total: 10,
          completion_pct: 50,
          rate_per_second: 1.25,
          started_at: "2026-06-20T10:00:00Z",
        },
      ],
    },
    queues: {
      processing_active: [],
    },
  });

  renderUI(
    <CurrentRunPanel status={status} feeds={[]} onFeedClick={vi.fn()} />,
  );

  expect(screen.getByText("Metadata")).toBeVisible();
  expect(screen.getByText("Writing feed metadata")).toBeVisible();
  expect(screen.getByText("50%")).toBeVisible();
  expect(screen.getByText("5 / 10 feeds")).toBeVisible();
  expect(screen.getByText("1.3 feeds/s")).toBeVisible();
  expect(
    screen.getByRole("progressbar", {
      name: /writing feed metadata progress/i,
    }),
  ).toHaveAttribute("aria-valuenow", "50");
  expect(
    screen.queryByText(/without per-feed queue entries/i),
  ).not.toBeInTheDocument();
});

test("shows whole processing batch and phase plan", () => {
  const feeds = [
    sampleAdminFeed({ name: "stopforumspam" }),
    sampleAdminFeed({ name: "dronebl_anonymizers" }),
    sampleAdminFeed({ name: "firehol_level1", kind: "merge" }),
  ];
  const status = sampleAdminStatus({
    engine: {
      running: true,
      current_phase: "sources",
      current_batch: {
        total: 3,
        completed: 1,
        active: 1,
        pending: 1,
        names: ["stopforumspam", "dronebl_anonymizers", "firehol_level1"],
        completed_names: ["stopforumspam"],
        active_names: ["dronebl_anonymizers"],
        pending_names: ["firehol_level1"],
        source_total: 2,
        source_completed: 1,
        merge_total: 1,
        merge_completed: 0,
      },
      phase_plan: {
        phases: ["preflight", "sources", "metadata", "publish"],
        current: "sources",
        current_position: 2,
        total: 4,
        final: true,
      },
    },
    queues: {
      processing_active: [
        {
          name: "dronebl_anonymizers",
          reason: "scheduled_due",
          started_at: "2026-06-20T10:00:00Z",
        },
      ],
    },
  });

  renderUI(
    <CurrentRunPanel status={status} feeds={feeds} onFeedClick={vi.fn()} />,
  );

  expect(screen.getByText("1 done · 1 active · 1 pending")).toBeVisible();
  expect(
    screen.getByText("stopforumspam, dronebl_anonymizers, firehol_level1"),
  ).toBeVisible();
  expect(screen.getByText("2/4 · Sources")).toBeVisible();
  expect(screen.getByText(/sources 1\/2/i)).toBeVisible();
  expect(screen.getByText(/merges 0\/1/i)).toBeVisible();
});

test("shows deferred processing as blocked waiting work", () => {
  const feeds = [
    sampleAdminFeed({ name: "alpha_feed" }),
    sampleAdminFeed({ name: "beta_feed" }),
  ];
  const status = sampleAdminStatus({
    queues: {
      processing_waiting: [
        {
          name: "alpha_feed",
          reason: "scheduled_due",
          queued_at: "2026-06-20T10:00:00Z",
        },
      ],
      processing_deferred: [
        {
          name: "beta_feed",
          reason: "manual_reprocess",
          queued_at: "2026-06-20T10:01:00Z",
        },
      ],
    },
  });

  renderUI(
    <CurrentRunPanel status={status} feeds={feeds} onFeedClick={vi.fn()} />,
  );

  expect(screen.getByText("2 feeds")).toBeVisible();
  expect(screen.getByText("alpha_feed")).toBeVisible();
  expect(screen.getByText("beta_feed")).toBeVisible();
  expect(
    screen.getByText(
      /blocked by active processing batch; rerun after it finishes/i,
    ),
  ).toBeVisible();
  expect(screen.queryByText(/\+1 pending/i)).not.toBeInTheDocument();
});

test("keeps live queue feed lists in fixed tiles with full-height scroll bodies", () => {
  const feedNames = Array.from(
    { length: 6 },
    (_, index) => `queued_feed_${index + 1}`,
  );
  const feeds = feedNames.map((name) => sampleAdminFeed({ name }));
  const waitingItems = feedNames.map((name, index) => ({
    name,
    reason: "scheduled_due",
    queued_at: `2026-06-20T10:0${index}:00Z`,
  }));
  const activeItems = feedNames.map((name, index) => ({
    name,
    reason: "scheduled_due",
    started_at: `2026-06-20T10:0${index}:00Z`,
  }));
  const status = sampleAdminStatus({
    engine: { running: true, current_phase: "sources" },
    queues: {
      download_waiting: waitingItems,
      download_active: activeItems,
      processing_waiting: waitingItems,
      processing_active: activeItems,
    },
  });

  renderUI(
    <CurrentRunPanel status={status} feeds={feeds} onFeedClick={vi.fn()} />,
  );

  for (const [tileName, regionName] of [
    ["Waiting To Be Downloaded tile", "Waiting To Be Downloaded queue"],
    ["Being Downloaded Now tile", "Being Downloaded Now queue"],
    ["Waiting To Be Processed tile", "Waiting To Be Processed queue"],
    ["Being Processed Now tile", "Being Processed Now queue"],
  ] as const) {
    const tile = screen.getByRole("group", { name: tileName });
    const queueRegion = within(tile).getByRole("region", {
      name: regionName,
    });
    expect(tile).toHaveClass(
      "h-[13.5rem]",
      "grid-rows-[auto_minmax(0,1fr)]",
    );
    expect(queueRegion).toHaveClass("min-h-0", "overflow-y-auto");
    expect(within(queueRegion).getAllByRole("listitem")).toHaveLength(6);
  }
});
