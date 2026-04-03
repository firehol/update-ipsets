import { expect, test } from "vitest";
import { screen, within } from "@testing-library/react";
import { HomeExplorer } from "./home-explorer";
import {
  sampleCategories,
  sampleFeedSummary,
} from "@/test/fixtures";
import { renderUI } from "@/test/render";

test("filters visible feeds and keeps the result set when switching views", async () => {
  const feeds = [
    sampleFeedSummary({ name: "alpha_feed", maintainer: "Alpha Maintainer" }),
    sampleFeedSummary({
      name: "beta_malware",
      category: "malware_infrastructure",
      maintainer: "Beta Maintainer",
      official_name: "Beta Malware Fixture",
      short_description: "Research-backed context for the beta fixture.",
    }),
  ];

  const { user } = renderUI(
    <HomeExplorer feeds={feeds} categories={sampleCategories} loading={false} />,
  );

  expect(screen.getByRole("link", { name: "alpha_feed" })).toBeVisible();
  expect(screen.getByRole("link", { name: "beta_malware" })).toBeVisible();

  await user.type(screen.getByRole("searchbox", { name: /filter feeds/i }), "beta");

  expect(screen.queryByRole("link", { name: "alpha_feed" })).toBeNull();
  expect(screen.getByRole("link", { name: "beta_malware" })).toBeVisible();
  expect(screen.getByText(/showing/i)).toHaveTextContent("Showing 1 of 2 feeds");

  await user.click(screen.getByRole("button", { name: "Table" }));

  const table = await screen.findByRole("table");
  const betaTableLink = within(table).getByRole("link", { name: "beta_malware" });
  expect(betaTableLink).toBeVisible();
  expect(within(table).queryByRole("link", { name: "alpha_feed" })).toBeNull();

  await user.hover(betaTableLink);
  expect((await screen.findAllByText("Beta Malware Fixture")).length).toBeGreaterThan(0);
});
