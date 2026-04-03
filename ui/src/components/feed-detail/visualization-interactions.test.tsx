import { expect, test } from "vitest";
import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { SectionASN } from "./section-asn";
import { SectionComparison } from "./section-comparison";
import { SectionGeo } from "./section-geo";
import { renderUI } from "@/test/render";
import { sampleFeedMetadata, sampleFeedSummary } from "@/test/fixtures";
import { server } from "@/test/msw-server";

test("ASN provider and view tabs switch visible attribution data by keyboard", async () => {
  server.use(
    http.get("/api/v1/sets/known_feed/asn", () =>
      HttpResponse.json([
        { name: "ip2asn", label: "ip2asn.com", type: "asn" },
        { name: "fixture_asn", label: "Fixture ASN", type: "asn" },
      ]),
    ),
    http.get("/api/v1/sets/known_feed/asn/ip2asn", () =>
      HttpResponse.json({
        provider: "ip2asn",
        feed_ips: 1200,
        attributed_ips: 1200,
        bogon_ips: 0,
        unknown_ips: 0,
        by_asn: [
          {
            asn: 13335,
            name: "Cloudflare, Inc.",
            count: 1200,
            percent: 100,
          },
        ],
      }),
    ),
    http.get("/api/v1/sets/known_feed/asn/fixture_asn", () =>
      HttpResponse.json({
        provider: "fixture_asn",
        feed_ips: 1200,
        attributed_ips: 400,
        bogon_ips: 0,
        unknown_ips: 800,
        by_asn: [
          {
            asn: 15169,
            name: "Google LLC",
            count: 400,
            percent: 33.33,
          },
        ],
      }),
    ),
  );

  const { user } = renderUI(<SectionASN feedName="known_feed" />);

  const primaryProvider = await screen.findByRole("tab", {
    name: "ip2asn.com",
  });
  const fixtureProvider = screen.getByRole("tab", { name: "Fixture ASN" });
  expect(primaryProvider).toHaveAttribute("aria-selected", "true");

  await user.tab();
  expect(primaryProvider).toHaveFocus();
  await user.tab();
  expect(fixtureProvider).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(fixtureProvider).toHaveAttribute("aria-selected", "true");

  await user.tab();
  expect(screen.getByRole("tab", { name: "Treemap" })).toHaveFocus();
  await user.tab();
  expect(screen.getByRole("tab", { name: "Bubble" })).toHaveFocus();
  await user.tab();
  const listTab = screen.getByRole("tab", { name: "List" });
  expect(listTab).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(listTab).toHaveAttribute("aria-selected", "true");
  expect(await screen.findByRole("link", { name: "AS15169" })).toBeVisible();
  expect(screen.getByText("Google LLC")).toBeVisible();
});

test("geo provider and view tabs switch visible country data by keyboard", async () => {
  server.use(
    http.get("/api/v1/sets/known_feed/countries", () =>
      HttpResponse.json([
        { name: "dbip", label: "DB-IP", type: "geoip" },
        { name: "fixture_geo", label: "Fixture Geo", type: "geoip" },
      ]),
    ),
    http.get("/api/v1/sets/known_feed/countries/dbip", () =>
      HttpResponse.json({
        total_mapped: 1200,
        countries: [{ code: "US", value: 1200 }],
      }),
    ),
    http.get("/api/v1/sets/known_feed/countries/fixture_geo", () =>
      HttpResponse.json({
        total_mapped: 350,
        countries: [{ code: "DE", value: 350 }],
      }),
    ),
    http.get("/api/v1/sets/known_feed/bogons", () =>
      HttpResponse.json([
        {
          name: "rfc_reserved",
          label: "RFC reserved",
          type: "bogons",
          authoritative: true,
        },
      ]),
    ),
    http.get("/api/v1/sets/known_feed/bogons/rfc_reserved", () =>
      HttpResponse.json({
        provider: "rfc_reserved",
        feed_ips: 1200,
        bogon_ips: 0,
        percent: 0,
        by_range: [],
      }),
    ),
  );

  const { user } = renderUI(
    <SectionGeo feedName="known_feed" feed={sampleFeedMetadata()} />,
  );

  const dbipProvider = await screen.findByRole("tab", { name: "DB-IP" });
  const fixtureProvider = screen.getByRole("tab", { name: "Fixture Geo" });
  expect(dbipProvider).toHaveAttribute("aria-selected", "true");

  await user.tab();
  expect(dbipProvider).toHaveFocus();
  await user.tab();
  expect(fixtureProvider).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(fixtureProvider).toHaveAttribute("aria-selected", "true");

  await user.tab();
  expect(screen.getByRole("tab", { name: "Map" })).toHaveFocus();
  await user.tab();
  const listTab = screen.getByRole("tab", { name: "List" });
  expect(listTab).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(listTab).toHaveAttribute("aria-selected", "true");
  expect(await screen.findByText("Germany")).toBeVisible();
  expect(screen.getByText("DE")).toBeVisible();
  expect(screen.getAllByText("350")[0]).toBeVisible();
});

test("comparison view tabs switch between list and graph views", async () => {
  server.use(
    http.get("/api/v1/sets", () =>
      HttpResponse.json([
        sampleFeedSummary({ name: "known_feed" }),
        sampleFeedSummary({
          name: "beta_malware",
          category: "malware_infrastructure",
        }),
      ]),
    ),
    http.get("/api/v1/sets/known_feed/compare", () =>
      HttpResponse.json([
        {
          name: "beta_malware",
          category: "malware_infrastructure",
          ips: 900,
          common: 150,
        },
      ]),
    ),
  );

  const { user } = renderUI(
    <SectionComparison
      feedName="known_feed"
      feedIPs={1200}
      feedHealthClass="healthy"
    />,
  );

  const listTab = await screen.findByRole("tab", { name: "List" });
  expect(listTab).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("link", { name: "beta_malware" })).toBeVisible();

  await user.tab();
  expect(listTab).toHaveFocus();
  await user.tab();
  const sankeyTab = screen.getByRole("tab", { name: "Sankey" });
  expect(sankeyTab).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(sankeyTab).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("link", { name: "beta_malware" })).toBeNull();

  await user.click(screen.getByRole("tab", { name: "Network" }));
  expect(screen.getByRole("tab", { name: "Network" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  await user.click(listTab);
  expect(listTab).toHaveAttribute("aria-selected", "true");
  expect(await screen.findByRole("link", { name: "beta_malware" }))
    .toBeVisible();
});
