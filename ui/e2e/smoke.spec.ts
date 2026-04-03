import { expect, type Locator, test } from "@playwright/test";
import { installApiFixtures } from "./api-fixtures";

test.beforeEach(async ({ page }) => {
  await installApiFixtures(page);
});

test("homepage renders and searches an IP through the production bundle", async ({
  page,
}) => {
  await page.goto("/");

  await expect(
    page.getByRole("heading", { level: 1, name: /All Cybercrime IP Feeds/i }),
  ).toBeVisible();

  const lookup = page.locator("#ip-lookup");
  await expect(lookup.getByText("Search any IPv4 address.")).toBeVisible();
  await lookup
    .getByRole("searchbox", { name: /search ip address across all feeds/i })
    .fill("1.1.1.1");
  await lookup.getByRole("button", { name: "Search IP" }).click();

  await expect(page).toHaveURL(/ip=1\.1\.1\.1/);
  await expect(lookup.getByText("Feeds matched")).toBeVisible();
  await expect(lookup.getByText("Cloudflare, Inc.")).toBeVisible();
  await expect(lookup.getByRole("link", { name: "alpha_feed" })).toBeVisible();
});

test("homepage treemap tiles are keyboard reachable links", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "Treemap" }).click();
  await expect(page.getByRole("img", { name: "Feed size treemap" }))
    .toBeVisible();

  const tile = page.getByRole("link", {
    name: "Open alpha_feed feed details",
  });
  for (let i = 0; i < 120; i++) {
    await page.keyboard.press("Tab");
    if (await tile.evaluate((element) => element === document.activeElement)) {
      break;
    }
  }
  await expect(tile).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/ipsets\/alpha_feed$/);
});

test("feed detail renders real chart surfaces in a browser", async ({ page }) => {
  await page.goto("/ipsets/known_feed");

  await expect(
    page.getByRole("heading", { level: 1, name: "known_feed" }),
  ).toBeVisible();
  await expect(page.getByText("Core public DNS")).toBeVisible();

  const behavior = page.locator("section", {
    has: page.getByRole("heading", {
      name: "How the list moves over time",
    }),
  });
  await expect(behavior.getByText("IP count evolution")).toBeVisible();

  const chart = behavior.getByLabel("IP count evolution chart");
  await expect(chart).toBeVisible();
  const box = await chart.boundingBox();
  expect(box?.width ?? 0).toBeGreaterThan(100);
  expect(box?.height ?? 0).toBeGreaterThan(100);

  const asnSection = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Where the IPs come from" }),
  });
  await asnSection.getByRole("tab", { name: "Bubble" }).click();
  await expectSvgSurface(
    asnSection.getByRole("img", { name: "ASN distribution bubble chart" }),
    "circle",
  );
  await asnSection.getByRole("tab", { name: "List" }).click();
  await expect(asnSection.getByRole("link", { name: "AS13335" })).toBeVisible();

  const geoSection = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Where they live on the map" }),
  });
  await expectSvgSurface(
    geoSection.getByRole("img", { name: "Country distribution map" }),
    'path[aria-label="Open United States country detail"]',
  );

  const overlapSection = page.locator("section").filter({
    has: page.getByRole("heading", { name: "Where else these IPs appear" }),
  });
  await overlapSection.getByRole("tab", { name: "Sankey" }).click();
  await expectSvgSurface(
    overlapSection.getByRole("img", { name: "Overlap sankey" }),
    "path, rect",
  );
  await overlapSection.getByRole("tab", { name: "Network" }).click();
  await expectSvgSurface(
    overlapSection.getByRole("img", { name: "Overlap network graph" }),
    "circle, line",
  );
});

test("admin feed drawer has real browser focus behavior", async ({ page }) => {
  await page.goto("/admin");

  const row = page.getByRole("button", { name: /open beta_malware/i });
  await expect(row).toBeVisible();
  await row.click();

  const dialog = page.getByRole("dialog", { name: "beta_malware" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Close" })).toBeFocused();

  await page.keyboard.press("Tab");
  await expect(dialog.getByRole("button", { name: "Recheck" })).toBeFocused();
});

test("country detail page renders provider-backed entity context", async ({
  page,
}) => {
  await page.goto("/countries/US");

  await expect(
    page.getByRole("heading", { level: 1, name: /United States/i }),
  ).toBeVisible();
  await expect(page.getByText("DB-IP").first()).toBeVisible();
  await expect(page.getByText("ip2asn.com").first()).toBeVisible();
  await expect(page.getByRole("link", { name: "alpha_feed" })).toBeVisible();
  await expect(page.getByRole("link", { name: "AS13335" })).toBeVisible();
});

async function expectSvgSurface(svg: Locator, markSelector: string) {
  await expect(svg).toBeVisible();
  const box = await svg.boundingBox();
  expect(box?.width ?? 0).toBeGreaterThan(100);
  expect(box?.height ?? 0).toBeGreaterThan(100);
  expect(await svg.locator(markSelector).count()).toBeGreaterThan(0);
}
