#!/usr/bin/env node

import { gzipSync } from "node:zlib";
import { pathToFileURL, fileURLToPath } from "node:url";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const defaultRootDir = path.resolve(scriptDir, "..");
const defaultConfigPath = path.join(defaultRootDir, "bundle-budget.config.mjs");

export function formatBytes(bytes) {
  if (!Number.isFinite(bytes)) {
    return "n/a";
  }
  if (Math.abs(bytes) < 1024) {
    return `${bytes} B`;
  }
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

export async function loadBudgetConfig(configPath = defaultConfigPath) {
  const imported = await import(pathToFileURL(path.resolve(configPath)));
  return imported.default ?? imported;
}

export async function checkBundleBudget(config, options = {}) {
  const rootDir = path.resolve(options.rootDir ?? defaultRootDir);
  const assetsDir = path.resolve(rootDir, config.assetsDir ?? "dist/assets");
  const files = await readAssetFiles(assetsDir);
  const warnAt = Number.isFinite(config.warnAt) ? config.warnAt : 0.9;

  const results = (config.budgets ?? []).map((budget) =>
    evaluateBudget(budget, files, warnAt),
  );
  const failures = results.filter((result) => result.status === "fail");
  const warnings = results.filter((result) => result.status === "warn");

  return {
    ok: failures.length === 0,
    results,
    failures,
    warnings,
  };
}

function evaluateBudget(budget, files, warnAt) {
  const patterns = (budget.patterns ?? []).map(parseChunkPattern);
  const matches = files.filter((file) =>
    patterns.some((pattern) => matchesChunkPattern(file.relative, pattern)),
  );
  const bytes = sum(matches, "bytes");
  const gzipBytes = sum(matches, "gzipBytes");
  const required = budget.required ?? true;
  const missing = required && matches.length === 0;
  const overBytes =
    Number.isFinite(budget.maxBytes) && bytes > budget.maxBytes;
  const overGzip =
    Number.isFinite(budget.maxGzipBytes) && gzipBytes > budget.maxGzipBytes;
  const nearBytes =
    Number.isFinite(budget.maxBytes) && bytes > budget.maxBytes * warnAt;
  const nearGzip =
    Number.isFinite(budget.maxGzipBytes) &&
    gzipBytes > budget.maxGzipBytes * warnAt;

  let status = "ok";
  if (missing || overBytes || overGzip) {
    status = "fail";
  } else if (nearBytes || nearGzip) {
    status = "warn";
  }

  return {
    name: budget.name,
    status,
    files: matches.map((file) => file.relative).sort(),
    bytes,
    gzipBytes,
    maxBytes: budget.maxBytes,
    maxGzipBytes: budget.maxGzipBytes,
    missing,
    overBytes,
    overGzip,
  };
}

const CHUNK_PATTERN_RE = /^\^([A-Za-z0-9_-]+)-\[\\w-\]\+\\\.(js|css)\$$/;

function parseChunkPattern(pattern) {
  const match = CHUNK_PATTERN_RE.exec(String(pattern));
  if (!match) {
    throw new Error(`unsupported bundle budget pattern: ${pattern}`);
  }
  return { prefix: match[1], extension: match[2] };
}

function matchesChunkPattern(fileName, pattern) {
  const prefix = `${pattern.prefix}-`;
  const suffix = `.${pattern.extension}`;
  if (!fileName.startsWith(prefix) || !fileName.endsWith(suffix)) {
    return false;
  }
  const chunk = fileName.slice(prefix.length, -suffix.length);
  return chunk.length > 0 && [...chunk].every(isChunkNameChar);
}

function isChunkNameChar(char) {
  return (
    (char >= "a" && char <= "z") ||
    (char >= "A" && char <= "Z") ||
    (char >= "0" && char <= "9") ||
    char === "_" ||
    char === "-"
  );
}

async function readAssetFiles(assetsDir) {
  const entries = await readdir(assetsDir, {
    recursive: true,
    withFileTypes: true,
  });
  const files = [];
  for (const entry of entries) {
    if (!entry.isFile()) {
      continue;
    }
    const absolute = path.join(entry.parentPath ?? assetsDir, entry.name);
    const relative = path.relative(assetsDir, absolute).split(path.sep).join("/");
    const body = await readFile(absolute);
    files.push({
      relative,
      bytes: body.length,
      gzipBytes: gzipSync(body).length,
    });
  }
  return files;
}

function sum(files, field) {
  return files.reduce((total, file) => total + file[field], 0);
}

export function formatReport(report) {
  const lines = ["Bundle budget report:"];
  for (const result of report.results) {
    const status = result.status.toUpperCase().padEnd(4);
    const raw = `${formatBytes(result.bytes)} / ${formatBytes(result.maxBytes)}`;
    const gzip = `${formatBytes(result.gzipBytes)} / ${formatBytes(
      result.maxGzipBytes,
    )}`;
    const files = result.files.length > 0 ? result.files.join(", ") : "no files";
    lines.push(`- ${status} ${result.name}: raw ${raw}, gzip ${gzip}`);
    lines.push(`       ${files}`);
  }
  return lines.join("\n");
}

async function main() {
  const configPath = process.argv[2]
    ? path.resolve(process.argv[2])
    : defaultConfigPath;
  const config = await loadBudgetConfig(configPath);
  const report = await checkBundleBudget(config, { rootDir: defaultRootDir });
  console.log(formatReport(report));
  if (!report.ok) {
    process.exitCode = 1;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });
}
