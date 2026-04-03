import { expect, test } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { axe } from "vitest-axe";
import { http, HttpResponse, type HttpHandler } from "msw";
import { Toaster } from "@/components/ui/sonner";
import { IntegrityPanel } from "@/components/admin/integrity-panel";
import { EntityIntegrityPanel } from "@/components/admin/entity-integrity-panel";
import { AdminPage } from "./admin";
import { sampleAdminFeed } from "@/test/fixtures";
import { renderUI } from "@/test/render";
import {
  adminPageHandlers,
  adminWriteActionHandlers,
  integrityIssueHandlers,
} from "@/test/page-scenarios";
import { server } from "@/test/msw-server";

test("sends feed recheck, reprocess, and disable actions through the admin API", async () => {
  const requests: string[] = [];
  server.use(...adminPageHandlers(), ...adminWriteActionHandlers(requests));

  const { user, container } = renderUI(
    <>
      <AdminPage />
      <Toaster richColors closeButton />
    </>,
    { route: "/admin" },
  );

  await user.type(
    await screen.findByRole("textbox", { name: /search admin feeds/i }),
    "beta",
  );
  const row = screen.getByRole("button", { name: /open beta_malware/i });
  await user.click(row);

  const dialog = await screen.findByRole("dialog", { name: "beta_malware" });
  await user.click(within(dialog).getByRole("button", { name: "Recheck" }));
  expect(await screen.findByText("beta_malware: recheck scheduled"))
    .toBeVisible();

  await user.click(within(dialog).getByRole("button", { name: "Reprocess" }));
  expect(await screen.findByText("beta_malware: reprocess scheduled"))
    .toBeVisible();

  await user.click(within(dialog).getByRole("button", { name: "Disable" }));
  expect(await screen.findByText("beta_malware: disabled")).toBeVisible();

  await waitFor(() => {
    expect(requests).toEqual(
      expect.arrayContaining([
        "POST /api/v1/admin/feeds/beta_malware/recheck",
        "POST /api/v1/admin/feeds/beta_malware/reprocess",
        "POST /api/v1/admin/feeds/beta_malware/disable",
      ]),
    );
  });

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});

test("sends feed enable action through the admin API", async () => {
  const requests: string[] = [];
  server.use(
    ...adminPageHandlers([
      sampleAdminFeed({
        name: "disabled_feed",
        enabled: false,
        maintainer: "Disabled Maintainer",
      }),
    ]),
    ...adminWriteActionHandlers(requests),
  );

  const { user } = renderUI(
    <>
      <AdminPage />
      <Toaster richColors closeButton />
    </>,
    { route: "/admin" },
  );

  const row = await screen.findByRole("button", {
    name: /open disabled_feed/i,
  });
  await user.click(row);
  const dialog = await screen.findByRole("dialog", { name: "disabled_feed" });
  await user.click(within(dialog).getByRole("button", { name: "Enable" }));

  expect(await screen.findByText("disabled_feed: enabled")).toBeVisible();
  await waitFor(() => {
    expect(requests).toContain("POST /api/v1/admin/feeds/disabled_feed/enable");
  });
});

test("recovers feed integrity findings through the admin API", async () => {
  const requests: string[] = [];
  server.use(...integrityIssueHandlers(), ...adminWriteActionHandlers(requests));

  const { user } = renderUI(
    <>
      <IntegrityPanel />
      <Toaster richColors closeButton />
    </>,
  );

  expect(await screen.findByText(/published metadata is older/i)).toBeVisible();
  await user.click(screen.getByRole("button", { name: /recover all 1/i }));

  expect(await screen.findByText("Scheduled integrity recovery for 1 item(s)"))
    .toBeVisible();
  await waitFor(() => {
    expect(requests).toContain("POST /api/v1/admin/integrity/reprocess");
  });
});

test("expands integrity finding details from the keyboard", async () => {
  server.use(...integrityIssueHandlers());

  const { user, container } = renderUI(<IntegrityPanel />);

  expect(await screen.findByText(/published metadata is older/i)).toBeVisible();

  await user.tab();
  expect(screen.getByRole("checkbox", { name: /include archived feeds/i }))
    .toHaveFocus();
  await user.tab();
  expect(screen.getByRole("button", { name: "Re-check" })).toHaveFocus();
  await user.tab();
  expect(screen.getByRole("button", { name: /recover all 1/i })).toHaveFocus();
  await user.tab();

  const expand = screen.getByRole("button", {
    name: /expand beta_malware integrity finding/i,
  });
  expect(expand).toHaveFocus();
  await user.keyboard("{Enter}");

  expect(
    screen.getByRole("button", {
      name: /collapse beta_malware integrity finding/i,
    }),
  ).toHaveAttribute("aria-expanded", "true");
  expect(screen.getByText("Stale (1)")).toBeVisible();

  expect(
    await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    }),
  ).toHaveNoViolations();
});

test("queues an entity artifact rebuild through the admin API", async () => {
  const requests: string[] = [];
  server.use(...adminPageHandlers(), ...adminWriteActionHandlers(requests));

  const { user } = renderUI(
    <>
      <EntityIntegrityPanel />
      <Toaster richColors closeButton />
    </>,
  );

  await screen.findByText(/country and asn artifacts are current/i);
  await user.click(screen.getByRole("button", { name: "Rebuild All" }));
  await user.click(screen.getByRole("button", { name: "Confirm Full Rebuild" }));

  expect(await screen.findByText("Queued full country and ASN rebuild"))
    .toBeVisible();
  await waitFor(() => {
    expect(requests).toContain(
      "POST /api/v1/admin/integrity/entities/rebuild",
    );
  });
});

test("shows feed action failures without misleading success toasts", async () => {
  server.use(
    ...adminPageHandlers(),
    ...failedFeedActionHandlers({
      recheck: "backend refused recheck",
      reprocess: "backend refused reprocess",
      disable: "backend refused disable",
    }),
  );

  const { user } = renderUI(
    <>
      <AdminPage />
      <Toaster richColors closeButton />
    </>,
    { route: "/admin" },
  );

  await user.type(
    await screen.findByRole("textbox", { name: /search admin feeds/i }),
    "beta",
  );
  await user.click(screen.getByRole("button", { name: /open beta_malware/i }));

  const dialog = await screen.findByRole("dialog", { name: "beta_malware" });
  await user.click(within(dialog).getByRole("button", { name: "Recheck" }));
  expect(await screen.findByText("Recheck failed: backend refused recheck"))
    .toBeVisible();
  expect(screen.queryByText("beta_malware: recheck scheduled")).toBeNull();

  await user.click(within(dialog).getByRole("button", { name: "Reprocess" }));
  expect(await screen.findByText("Reprocess failed: backend refused reprocess"))
    .toBeVisible();
  expect(screen.queryByText("beta_malware: reprocess scheduled")).toBeNull();

  await user.click(within(dialog).getByRole("button", { name: "Disable" }));
  expect(await screen.findByText("Failed: backend refused disable"))
    .toBeVisible();
  expect(screen.queryByText("beta_malware: disabled")).toBeNull();
});

test("shows feed enable failure without a misleading success toast", async () => {
  server.use(
    ...adminPageHandlers([
      sampleAdminFeed({
        name: "disabled_feed",
        enabled: false,
        maintainer: "Disabled Maintainer",
      }),
    ]),
    ...failedFeedActionHandlers({ enable: "backend refused enable" }),
  );

  const { user } = renderUI(
    <>
      <AdminPage />
      <Toaster richColors closeButton />
    </>,
    { route: "/admin" },
  );

  await user.click(
    await screen.findByRole("button", { name: /open disabled_feed/i }),
  );
  const dialog = await screen.findByRole("dialog", { name: "disabled_feed" });
  await user.click(within(dialog).getByRole("button", { name: "Enable" }));

  expect(await screen.findByText("Failed: backend refused enable"))
    .toBeVisible();
  expect(screen.queryByText("disabled_feed: enabled")).toBeNull();
});

test("shows integrity recovery failure without a misleading success toast", async () => {
  server.use(
    ...integrityIssueHandlers(),
    http.post("/api/v1/admin/integrity/reprocess", () =>
      HttpResponse.json({ error: "backend refused recovery" }, { status: 500 }),
    ),
  );

  const { user } = renderUI(
    <>
      <IntegrityPanel />
      <Toaster richColors closeButton />
    </>,
  );

  expect(await screen.findByText(/published metadata is older/i)).toBeVisible();
  await user.click(screen.getByRole("button", { name: /recover all 1/i }));

  expect(await screen.findByText("Recovery failed: backend refused recovery"))
    .toBeVisible();
  expect(screen.queryByText(/Scheduled integrity recovery/)).toBeNull();
});

test("shows entity rebuild failure without a misleading success toast", async () => {
  server.use(
    ...adminPageHandlers(),
    http.post("/api/v1/admin/integrity/entities/rebuild", () =>
      HttpResponse.json({ error: "backend refused rebuild" }, { status: 500 }),
    ),
  );

  const { user } = renderUI(
    <>
      <EntityIntegrityPanel />
      <Toaster richColors closeButton />
    </>,
  );

  await screen.findByText(/country and asn artifacts are current/i);
  await user.click(screen.getByRole("button", { name: "Rebuild All" }));
  await user.click(screen.getByRole("button", { name: "Confirm Full Rebuild" }));

  expect(await screen.findByText("Entity rebuild failed: backend refused rebuild"))
    .toBeVisible();
  expect(screen.queryByText("Queued full country and ASN rebuild")).toBeNull();
});

function failedFeedActionHandlers(
  failures: Partial<Record<"recheck" | "reprocess" | "enable" | "disable", string>>,
): HttpHandler[] {
  return Object.entries(failures).map(([action, error]) =>
    http.post(`/api/v1/admin/feeds/:name/${action}`, () =>
      HttpResponse.json({ error }, { status: 500 }),
    ),
  );
}
