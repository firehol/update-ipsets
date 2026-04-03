import { expect, test } from "vitest";
import {
  applyFilters,
  applySort,
  readExplorerState,
  writeExplorerState,
} from "./explorer-state";
import { sampleFeedSummary, sampleHealth } from "@/test/fixtures";

test("round-trips URL state for critical filters and non-default health", () => {
  const state = readExplorerState(
    new URLSearchParams(
      "q=beta&category=intrusion&health=all&critical=hard&critical_overlap=soft&view=table&sort=name",
    ),
  );

  expect(state.q).toBe("beta");
  expect([...state.categories]).toEqual(["intrusion"]);
  expect([...state.health]).toEqual([]);
  expect([...state.criticalReference]).toEqual(["hard"]);
  expect([...state.criticalOverlap]).toEqual(["soft"]);
  expect(state.view).toBe("table");
  expect(state.sort).toBe("name");

  const written = writeExplorerState(new URLSearchParams(), state);
  expect(written.get("q")).toBe("beta");
  expect(written.get("category")).toBe("intrusion");
  expect(written.get("health")).toBe("all");
  expect(written.get("critical")).toBe("hard");
  expect(written.get("critical_overlap")).toBe("soft");
  expect(written.get("view")).toBe("table");
  expect(written.get("sort")).toBe("name");
});

test("filters critical reference and overlap tiers from typed feed fields", () => {
  const feeds = [
    sampleFeedSummary({
      name: "critical_dns_core",
      critical: { tier: "hard", role: "public_dns" },
    }),
    sampleFeedSummary({
      name: "consumer_feed",
      critical_overlap_tiers: ["soft"],
    }),
    sampleFeedSummary({
      name: "ordinary_feed",
    }),
  ];

  const hardReference = readExplorerState(new URLSearchParams("critical=hard"));
  expect(applyFilters(feeds, hardReference).map((feed) => feed.name)).toEqual([
    "critical_dns_core",
  ]);

  const softOverlap = readExplorerState(
    new URLSearchParams("critical_overlap=soft"),
  );
  expect(applyFilters(feeds, softOverlap).map((feed) => feed.name)).toEqual([
    "consumer_feed",
  ]);
});

test("keeps archived feeds out of the default health view until explicitly included", () => {
  const feeds = [
    sampleFeedSummary({ name: "healthy_feed", health: sampleHealth("healthy") }),
    sampleFeedSummary({ name: "archived_feed", health: sampleHealth("archived") }),
  ];

  const defaults = readExplorerState(new URLSearchParams());
  expect(applyFilters(feeds, defaults).map((feed) => feed.name)).toEqual([
    "healthy_feed",
  ]);

  const allHealth = readExplorerState(new URLSearchParams("health=all"));
  expect(applyFilters(feeds, allHealth).map((feed) => feed.name)).toEqual([
    "healthy_feed",
    "archived_feed",
  ]);
});

test("free-text search includes researched feed context", () => {
  const feeds = [
    sampleFeedSummary({
      name: "alpha_feed",
      official_name: "Alpha Fixture",
      short_description: "Research-backed scanner context.",
    }),
    sampleFeedSummary({ name: "beta_feed" }),
  ];

  const byOfficialName = readExplorerState(new URLSearchParams("q=fixture"));
  expect(applyFilters(feeds, byOfficialName).map((feed) => feed.name)).toEqual([
    "alpha_feed",
  ]);

  const byShortDescription = readExplorerState(new URLSearchParams("q=scanner"));
  expect(applyFilters(feeds, byShortDescription).map((feed) => feed.name)).toEqual([
    "alpha_feed",
  ]);
});

test("sorts coverage and maintainer views with deterministic tie breakers", () => {
  const feeds = [
    sampleFeedSummary({
      name: "charlie",
      maintainer: "Zeta Maintainer",
      unique_ips: 20,
    }),
    sampleFeedSummary({
      name: "alpha",
      maintainer: "Alpha Maintainer",
      unique_ips: 50,
    }),
    sampleFeedSummary({
      name: "beta",
      maintainer: "Alpha Maintainer",
      unique_ips: 50,
    }),
  ];

  expect(applySort(feeds, "coverage").map((feed) => feed.name)).toEqual([
    "alpha",
    "beta",
    "charlie",
  ]);
  expect(applySort(feeds, "maintainer").map((feed) => feed.name)).toEqual([
    "alpha",
    "beta",
    "charlie",
  ]);
});
