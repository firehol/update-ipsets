import { expect, test } from "vitest";
import { screen } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import { axe } from "vitest-axe";
import { http, HttpResponse } from "msw";
import { MethodologyPage } from "./methodology";
import { renderUI } from "@/test/render";
import { server } from "@/test/msw-server";

test("methodology index explains interpretation, not implementation paths", async () => {
  server.use(
    http.get("/api/v1/methodology", () =>
      HttpResponse.json({
        items: [
          {
            slug: "feed-health",
            title: "Feed health",
            summary: "How to read feed health classes.",
          },
        ],
      }),
    ),
  );

  const { container } = renderUI(<MethodologyPage />, {
    route: "/methodology",
  });

  expect(await screen.findByRole("heading", { name: "Methodology" }))
    .toBeVisible();
  expect(
    screen.getByText(/what each signal means, how to interpret it/i),
  ).toBeVisible();
  expect(screen.queryByText(/source code paths/i)).toBeNull();
  expect(screen.getByRole("link", { name: "Feed health" })).toBeVisible();

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});

test("methodology detail renders sanitized public content", async () => {
  server.use(
    http.get("/api/v1/methodology/feed-health", () =>
      HttpResponse.json({
        slug: "feed-health",
        title: "Feed health",
        summary: "How to read feed health classes.",
        body: "<p>Healthy means the feed is current.</p><script>window.bad=true</script>",
      }),
    ),
  );

  const { container } = renderUI(
    <Routes>
      <Route path="/methodology/:slug" element={<MethodologyPage />} />
      <Route path="/methodology" element={<MethodologyPage />} />
    </Routes>,
    { route: "/methodology/feed-health" },
  );

  expect(await screen.findByRole("heading", { name: "Feed health" }))
    .toBeVisible();
  expect(screen.getByText(/Healthy means the feed is current/i)).toBeVisible();
  expect(
    screen.getByRole("link", { name: /Methodology index/i }),
  ).toHaveAttribute("href", "/methodology");
  expect(screen.queryByText(/window\.bad/i, { ignore: false })).toBeNull();

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});
