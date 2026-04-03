import { expect, test } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { axe } from "vitest-axe";
import { IPSearchSurface } from "./ip-search-surface";
import { sampleSearchResult } from "@/test/fixtures";
import { renderUI } from "@/test/render";
import { server } from "@/test/msw-server";

test("searches an IP through the backend boundary and shows matching feeds", async () => {
  const requested = new URLSearchParams();
  server.use(
    http.get("/api/v1/search", ({ request }) => {
      const url = new URL(request.url);
      for (const [key, value] of url.searchParams) requested.set(key, value);
      return HttpResponse.json(sampleSearchResult);
    }),
  );

  const { user, container } = renderUI(
    <IPSearchSurface
      scope={{ kind: "global" }}
      variant="hero"
      placeholder="e.g. 1.1.1.1"
    />,
  );

  await user.type(
    screen.getByRole("searchbox", {
      name: /search ip address across all feeds/i,
    }),
    "1.1.1.1",
  );
  await user.click(screen.getByRole("button", { name: /search ip/i }));

  expect(
    await screen.findByText(/1 tracked feed currently match 1\.1\.1\.1/i),
  ).toBeVisible();
  expect(screen.getByRole("link", { name: /alpha_feed/i })).toBeVisible();

  await waitFor(() => expect(requested.get("ip")).toBe("1.1.1.1"));
  expect(requested.get("details")).toBe("true");
  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});
