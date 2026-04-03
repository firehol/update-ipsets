import { expect, test } from "vitest";
import { screen, within } from "@testing-library/react";
import { axe } from "vitest-axe";
import { Route, Routes } from "react-router-dom";
import { FeedDetailPage } from "./feed-detail";
import { renderUI } from "@/test/render";
import { feedDetailPageHandlers } from "@/test/page-scenarios";
import { server } from "@/test/msw-server";

test("shows a user-facing not-found state when the feed API returns 404", async () => {
  renderUI(
    <Routes>
      <Route path="/ipsets/:name" element={<FeedDetailPage />} />
    </Routes>,
    { route: "/ipsets/missing_feed" },
  );

  expect(
    await screen.findByRole("heading", { name: /feed not found/i }),
  ).toBeVisible();
  expect(screen.getByText("missing_feed")).toBeVisible();
  expect(screen.getByRole("link", { name: /back to the explorer/i })).toBeVisible();
});

test("loads a feed detail page through the published API surfaces", async () => {
  server.use(...feedDetailPageHandlers("known_feed"));

  const { container, user } = renderUI(
    <Routes>
      <Route path="/ipsets/:name" element={<FeedDetailPage />} />
    </Routes>,
    { route: "/ipsets/known_feed" },
  );

  expect(
    await screen.findByRole("heading", { name: /known_feed/i }),
  ).toBeVisible();
  expect(screen.getByRole("link", { name: /download known_feed.ipset/i }))
    .toBeVisible();

  expect((await screen.findAllByText("Known Test Feed")).length).toBeGreaterThan(0);
  expect(screen.getByText(/research-backed context for the test feed/i))
    .toBeVisible();
  expect(screen.getByText("How IPs get on and off this list")).toBeVisible();
  expect(screen.getByText(/maintainer describes hourly updates/i)).toBeVisible();
  expect(await screen.findByText("Critical infrastructure overlap"))
    .toBeVisible();
  expect(screen.getByText("Core public DNS")).toBeVisible();
  const overlapTable = (await screen.findAllByRole("table")).find((table) =>
    within(table).queryByRole("link", { name: "beta_malware" }),
  );
  if (!overlapTable) throw new Error("overlap table did not render beta_malware");
  const overlapFeedLink = within(overlapTable).getByRole("link", {
    name: "beta_malware",
  });
  await user.hover(overlapFeedLink);
  expect((await screen.findAllByText("Beta Malware Fixture")).length).toBeGreaterThan(0);
  expect(screen.getByText("Where the IPs come from")).toBeVisible();
  expect(screen.getByText("ip2asn.com")).toBeVisible();
  expect(screen.getByText("Where they live on the map")).toBeVisible();
  expect(screen.getByText("DB-IP")).toBeVisible();

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});
