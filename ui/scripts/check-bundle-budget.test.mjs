import assert from "node:assert/strict";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import {
  checkBundleBudget,
  formatBytes,
} from "./check-bundle-budget.mjs";

test("passes grouped chunks matched by stable prefixes", async () => {
  const root = await makeFixture();
  try {
    await writeAsset(root, "admin-a1.js", "console.log('admin');");
    await writeAsset(root, "admin-layout-b2.js", "console.log('layout');");

    const report = await checkBundleBudget(
      {
        assetsDir: "dist/assets",
        budgets: [
          {
            name: "admin route",
            patterns: ["^admin-[\\w-]+\\.js$", "^admin-layout-[\\w-]+\\.js$"],
            maxBytes: 1024,
            maxGzipBytes: 1024,
          },
        ],
      },
      { rootDir: root },
    );

    assert.equal(report.ok, true);
    assert.equal(report.results[0].status, "ok");
    assert.deepEqual(report.results[0].files, [
      "admin-a1.js",
      "admin-layout-b2.js",
    ]);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("fails when a required budget has no matching chunk", async () => {
  const root = await makeFixture();
  try {
    await writeAsset(root, "index-a1.js", "console.log('shell');");

    const report = await checkBundleBudget(
      {
        assetsDir: "dist/assets",
        budgets: [
          {
            name: "feed detail route",
            patterns: ["^feed-detail-[\\w-]+\\.js$"],
            maxBytes: 1024,
            maxGzipBytes: 1024,
          },
        ],
      },
      { rootDir: root },
    );

    assert.equal(report.ok, false);
    assert.equal(report.results[0].status, "fail");
    assert.equal(report.results[0].missing, true);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("fails when a matched chunk exceeds raw or gzip limits", async () => {
  const root = await makeFixture();
  try {
    await writeAsset(root, "index-a1.js", "x".repeat(2048));

    const report = await checkBundleBudget(
      {
        assetsDir: "dist/assets",
        budgets: [
          {
            name: "public shell",
            patterns: ["^index-[\\w-]+\\.js$"],
            maxBytes: 512,
            maxGzipBytes: 1,
          },
        ],
      },
      { rootDir: root },
    );

    assert.equal(report.ok, false);
    assert.equal(report.results[0].status, "fail");
    assert.equal(report.results[0].overBytes, true);
    assert.equal(report.results[0].overGzip, true);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("formats byte counts for report output", () => {
  assert.equal(formatBytes(512), "512 B");
  assert.equal(formatBytes(1536), "1.5 KiB");
});

async function makeFixture() {
  const root = await mkdtemp(path.join(os.tmpdir(), "bundle-budget-"));
  await mkdir(path.join(root, "dist", "assets"), { recursive: true });
  return root;
}

async function writeAsset(root, name, body) {
  await writeFile(path.join(root, "dist", "assets", name), body);
}
