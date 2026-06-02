import { expect, test } from "vitest";
import { screen } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import { axe } from "vitest-axe";
import { ASNDetailPage } from "./asn-detail";
import { ASNsIndexPage } from "./asns-index";
import { CountriesIndexPage } from "./countries-index";
import { CountryDetailPage } from "./country-detail";
import { MaintainerDetailPage } from "./maintainer-detail";
import { MaintainersIndexPage } from "./maintainers-index";
import { renderUI } from "@/test/render";
import {
  entityIndexHandlers,
  entityPageHandlers,
} from "@/test/page-scenarios";
import { server } from "@/test/msw-server";

async function expectNoA11yViolations(container: HTMLElement) {
  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
}

test("loads the countries index from the public country API", async () => {
  server.use(...entityIndexHandlers());

  const { container } = renderUI(
    <Routes>
      <Route path="/countries" element={<CountriesIndexPage />} />
    </Routes>,
    { route: "/countries" },
  );

  expect(
    await screen.findByRole("heading", {
      name: /Every country currently attributed by public feeds/i,
    }),
  ).toBeVisible();
  const countryLink = await screen.findByRole("link", { name: /United States/i });
  expect(screen.getByText(/DB-IP/i)).toBeVisible();
  expect(countryLink).toHaveAttribute("href", "/countries/US");
  expect(screen.getByText("US")).toBeVisible();
  expect(screen.getByText("1,200")).toBeVisible();

  await expectNoA11yViolations(container);
});

test("loads the ASN index from the public ASN API", async () => {
  server.use(...entityIndexHandlers());

  const { container } = renderUI(
    <Routes>
      <Route path="/asns" element={<ASNsIndexPage />} />
    </Routes>,
    { route: "/asns" },
  );

  expect(
    await screen.findByRole("heading", {
      name: /Every ASN currently attributed by public feeds/i,
    }),
  ).toBeVisible();
  const asnLink = await screen.findByRole("link", { name: "AS13335" });
  expect(
    screen.getByText("Aggregated from the active public ASN-attribution provider", {
      exact: false,
    }),
  ).toHaveTextContent("ip2asn.com");
  expect(asnLink).toHaveAttribute("href", "/asns/13335");
  expect(screen.getByText("Cloudflare, Inc.")).toBeVisible();
  expect(screen.getByText("1,200")).toBeVisible();

  await expectNoA11yViolations(container);
});

test("loads the maintainer index from the public maintainer API", async () => {
  server.use(...entityIndexHandlers());

  const { container } = renderUI(
    <Routes>
      <Route path="/maintainers" element={<MaintainersIndexPage />} />
    </Routes>,
    { route: "/maintainers" },
  );

  expect(
    await screen.findByRole("heading", {
      name: "Everyone who publishes a tracked feed.",
    }),
  ).toBeVisible();
  expect(await screen.findByRole("link", { name: "Alpha Maintainer" }))
    .toHaveAttribute("href", "/maintainers/alpha-maintainer");
  expect(screen.getByText("example.invalid")).toBeVisible();
  expect(screen.getByText("1,200")).toBeVisible();

  await expectNoA11yViolations(container);
});

test("loads a country detail page through the configured provider payloads", async () => {
  server.use(...entityPageHandlers());

  const { container } = renderUI(
    <Routes>
      <Route path="/countries/:code" element={<CountryDetailPage />} />
    </Routes>,
    { route: "/countries/US" },
  );

  expect(
    await screen.findByRole("heading", { name: /United States/i }),
  ).toBeVisible();
  expect(screen.getAllByText("DB-IP")[0]).toBeVisible();
  expect(screen.getAllByText("ip2asn.com")[0]).toBeVisible();
  expect(screen.getByRole("link", { name: "alpha_feed" })).toBeVisible();
  expect(screen.getByRole("link", { name: "AS13335" })).toBeVisible();

  await expectNoA11yViolations(container);
});

test("loads an ASN detail page with country and feed context", async () => {
  server.use(...entityPageHandlers());

  const { container } = renderUI(
    <Routes>
      <Route path="/asns/:asn" element={<ASNDetailPage />} />
    </Routes>,
    { route: "/asns/13335" },
  );

  expect(await screen.findByRole("heading", { level: 1, name: /AS13335/i }))
    .toBeVisible();
  expect(screen.getAllByText("Cloudflare, Inc.")[0]).toBeVisible();
  expect(screen.getAllByText("ip2asn.com")[0]).toBeVisible();
  expect(screen.getByRole("link", { name: /United States/i })).toBeVisible();
  expect(screen.getByRole("link", { name: "alpha_feed" })).toBeVisible();

  await expectNoA11yViolations(container);
});

test("loads a maintainer detail page grouped by public category", async () => {
  server.use(...entityPageHandlers());

  const { container } = renderUI(
    <Routes>
      <Route path="/maintainers/:slug" element={<MaintainerDetailPage />} />
    </Routes>,
    { route: "/maintainers/alpha-maintainer" },
  );

  expect(
    await screen.findByRole("heading", { name: "Alpha Maintainer" }),
  ).toBeVisible();
  expect(screen.getByRole("link", { name: "https://example.invalid/alpha ↗" }))
    .toBeVisible();
  expect(screen.getByRole("link", { name: "alpha_feed" })).toBeVisible();
  expect(screen.getByText("Intrusion")).toBeVisible();

  await expectNoA11yViolations(container);
});
