import { expect, test, vi } from "vitest";
import { screen } from "@testing-library/react";
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
});
