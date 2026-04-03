import { expect, test } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { axe } from "vitest-axe";
import { HomePage } from "./home";
import { renderUI } from "@/test/render";
import { homePageHandlers } from "@/test/page-scenarios";
import { server } from "@/test/msw-server";

test("loads homepage data through queries and preserves filtering when switching explorer views", async () => {
  server.use(...homePageHandlers());

  const { user, container } = renderUI(<HomePage />);

  expect(await screen.findByRole("link", { name: "alpha_feed" }))
    .toBeVisible();
  expect(screen.getByRole("link", { name: "beta_malware" })).toBeVisible();
  expect(screen.getByText("Critical references")).toBeVisible();
  expect(screen.getByText("Critical overlap")).toBeVisible();

  await user.type(
    screen.getByRole("searchbox", { name: /filter feeds/i }),
    "beta",
  );

  expect(screen.queryByRole("link", { name: "alpha_feed" })).toBeNull();
  expect(screen.getByRole("link", { name: "beta_malware" })).toBeVisible();
  expect(screen.getByText(/showing/i)).toHaveTextContent("Showing 1 of 2 feeds");

  await user.click(screen.getByRole("button", { name: "Table" }));

  const table = await screen.findByRole("table");
  expect(within(table).getByRole("link", { name: "beta_malware" }))
    .toBeVisible();
  expect(within(table).queryByRole("link", { name: "alpha_feed" }))
    .toBeNull();

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});

test("hydrates the IP lookup from the URL and clears the visible result", async () => {
  server.use(...homePageHandlers());

  const { user } = renderUI(<HomePage />, { route: "/?ip=1.1.1.1" });

  expect(
    screen.getByRole("searchbox", {
      name: /search ip address across all feeds/i,
    }),
  ).toHaveValue("1.1.1.1");

  expect(await screen.findByText("Cloudflare, Inc.")).toBeVisible();
  expect(screen.getByText(/via ip2asn\.com/i)).toBeVisible();

  await user.click(screen.getByRole("button", { name: "Clear" }));

  await waitFor(() => {
    expect(screen.queryByText("Cloudflare, Inc.")).toBeNull();
  });
  expect(
    screen.getByRole("searchbox", {
      name: /search ip address across all feeds/i,
    }),
  ).toHaveValue("");
});

test("seeds the IP lookup from the detected client IP when no URL IP is present", async () => {
  server.use(...homePageHandlers());

  renderUI(<HomePage />);

  expect(
    await screen.findByText(/detected from your connection/i),
  ).toBeVisible();
  expect(screen.getByText("203.0.113.10")).toBeVisible();
  await waitFor(() => {
    expect(
      screen.getByRole("searchbox", {
        name: /search ip address across all feeds/i,
      }),
    ).toHaveValue("203.0.113.10");
  });
});
