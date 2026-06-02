import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL, fileURLToPath } from "node:url";

import { ESLint } from "eslint";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const uiRoot = path.resolve(scriptDir, "..");
const repoRoot = path.resolve(uiRoot, "..");
const rootConfigPath = path.join(repoRoot, "eslint.config.mjs");

test("root ESLint bridge exports a flat config array", async () => {
  const imported = await import(pathToFileURL(rootConfigPath));
  const config = imported.default;

  assert.ok(Array.isArray(config));
  assert.ok(config.length > 1);
  for (const entry of config) {
    assert.equal(typeof entry, "object");
    assert.notEqual(entry, null);
  }
});

test("root ESLint bridge resolves representative UI and root files", async () => {
  const eslint = new ESLint({
    cwd: repoRoot,
    ignore: false,
    overrideConfigFile: rootConfigPath,
  });

  for (const filePath of [
    "ui/src/root-eslint-bridge-fixture.ts",
    "ui/src/root-eslint-bridge-fixture.tsx",
    "root-eslint-bridge-fixture.js",
    "root-eslint-bridge-fixture.mjs",
  ]) {
    const config = await eslint.calculateConfigForFile(
      path.join(repoRoot, filePath),
    );

    assert.ok(config, `${filePath} should receive an ESLint config`);
  }
});

test("root ESLint bridge applies the UI TypeScript rules", async () => {
  const eslint = new ESLint({
    cwd: repoRoot,
    ignore: false,
    overrideConfigFile: rootConfigPath,
  });

  const [result] = await eslint.lintText(
    "const unusedValue: string = 'unused';\n",
    {
      filePath: path.join(repoRoot, "ui/src/root-eslint-bridge-fixture.ts"),
    },
  );

  assert.equal(result.fatalErrorCount, 0);
  assert.ok(
    result.messages.some(
      (message) => message.ruleId === "@typescript-eslint/no-unused-vars",
    ),
    "expected the bridged UI config to apply TypeScript lint rules",
  );
});

test("root ESLint bridge parses JavaScript and MJS files without fatal errors", async () => {
  const eslint = new ESLint({
    cwd: repoRoot,
    ignore: false,
    overrideConfigFile: rootConfigPath,
  });

  const samples = [
    {
      filePath: "root-eslint-bridge-fixture.js",
      code: "const value = 1;\nvoid value;\n",
    },
    {
      filePath: "root-eslint-bridge-fixture.mjs",
      code: "export const value = 1;\n",
    },
  ];

  for (const sample of samples) {
    const [result] = await eslint.lintText(sample.code, {
      filePath: path.join(repoRoot, sample.filePath),
    });

    assert.equal(result.fatalErrorCount, 0, sample.filePath);
  }
});

test("root ESLint bridge applies Node script rules", async () => {
  const eslint = new ESLint({
    cwd: repoRoot,
    ignore: false,
    overrideConfigFile: rootConfigPath,
  });

  const [result] = await eslint.lintText(
    "const unusedValue = process.cwd();\nconsole.log('ready');\n",
    {
      filePath: path.join(repoRoot, "scripts/root-eslint-bridge-fixture.mjs"),
    },
  );

  assert.equal(result.fatalErrorCount, 0);
  assert.ok(
    result.messages.some((message) => message.ruleId === "no-unused-vars"),
    "expected the root config to apply Node script lint rules",
  );
});
