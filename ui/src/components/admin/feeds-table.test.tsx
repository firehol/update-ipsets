import { expect, test } from "vitest";
import { screen, within } from "@testing-library/react";
import { axe } from "vitest-axe";
import { AdminPage } from "@/pages/admin";
import { renderUI } from "@/test/render";
import { adminPageHandlers } from "@/test/page-scenarios";
import { server } from "@/test/msw-server";

test("filters admin rows and opens the selected feed details", async () => {
  server.use(...adminPageHandlers());

  const { user, container } = renderUI(<AdminPage />, { route: "/admin" });

  expect(
    await screen.findByRole("heading", { name: /pipeline overview/i }),
  ).toBeVisible();
  await screen.findByRole("button", { name: /open beta_malware/i });
  expect(screen.getByRole("columnheader", { name: "Feed" }))
    .toHaveAttribute("aria-sort", "none");
  expect(screen.getByRole("button", { name: /sort by feed ascending/i }))
    .toBeVisible();

  await user.type(
    screen.getByRole("textbox", { name: /search admin feeds/i }),
    "beta",
  );

  expect(screen.queryByRole("button", { name: /open alpha_feed/i })).toBeNull();
  const row = screen.getByRole("button", { name: /open beta_malware/i });
  expect(row).toBeVisible();

  await user.click(row);

  const dialog = await screen.findByRole("dialog", { name: "beta_malware" });
  expect(within(dialog).getByText("Beta feed modal content used by the page-level admin test."))
    .toBeVisible();
  expect(within(dialog).getByText("Beta Maintainer")).toBeVisible();
  expect(within(dialog).getByText("File manifest")).toBeVisible();
  expect(within(dialog).getAllByText("beta_malware.ipset")[0]).toBeVisible();
  expect(within(dialog).getByText(/1\/1 present/i)).toBeVisible();

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});
