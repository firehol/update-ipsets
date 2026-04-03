import { expect, test } from "vitest";
import { screen } from "@testing-library/react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { axe } from "vitest-axe";
import { NotFoundPage } from "./not-found";
import { renderUI } from "@/test/render";

function LocationProbe() {
  const location = useLocation();
  return (
    <div role="status" aria-label="current route">
      {location.pathname}
      {location.hash}
    </div>
  );
}

test("catalog route redirects to the homepage explorer anchor", async () => {
  renderUI(
    <Routes>
      <Route path="/catalog" element={<Navigate to="/#explorer" replace />} />
      <Route path="/" element={<LocationProbe />} />
    </Routes>,
    { route: "/catalog" },
  );

  expect(await screen.findByRole("status", { name: /current route/i }))
    .toHaveTextContent("/#explorer");
});

test("not-found route gives users a homepage recovery link", async () => {
  const { container } = renderUI(
    <Routes>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>,
    { route: "/does-not-exist" },
  );

  expect(screen.getByRole("heading", { name: "404" })).toBeVisible();
  expect(screen.getByText(/That page does not exist/i)).toBeVisible();
  expect(
    screen.getByRole("link", { name: /Back to the homepage/i }),
  ).toHaveAttribute("href", "/");

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});
