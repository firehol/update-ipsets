#!/usr/bin/env node
import { mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";

const [, , sourceArg = "docs", destArg = "wiki", wikiBaseArg] = process.argv;
const sourceDir = path.resolve(sourceArg);
const destDir = path.resolve(destArg);
const wikiBaseURL = normalizeWikiBaseURL(
  wikiBaseArg ?? process.env.WIKI_BASE_URL ?? defaultWikiBaseURL(),
);

function toPosix(value) {
  return value.split(path.sep).join(path.posix.sep);
}

async function walkMarkdown(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walkMarkdown(fullPath)));
      continue;
    }

    if (entry.isFile() && entry.name.endsWith(".md")) {
      files.push(fullPath);
    }
  }

  return files.sort();
}

async function cleanDestination(dir) {
  await mkdir(dir, { recursive: true });

  const entries = await readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (entry.name === ".git") {
      continue;
    }
    await rm(path.join(dir, entry.name), { recursive: true, force: true });
  }
}

function outputNameFor(relPath, basenameCounts) {
  if (relPath === "Home.md" || relPath === "_Sidebar.md") {
    return relPath;
  }

  const basename = path.posix.basename(relPath);
  if (basenameCounts.get(basename) === 1) {
    return basename;
  }

  const withoutExt = relPath.slice(0, -".md".length);
  return `${withoutExt.split("/").join("-")}.md`;
}

function hasScheme(target) {
  return /^[a-z][a-z0-9+.-]*:/i.test(target);
}

function defaultWikiBaseURL() {
  return `https://github.com/${process.env.GITHUB_REPOSITORY ?? "firehol/update-ipsets"}/wiki`;
}

function normalizeWikiBaseURL(value) {
  const trimmed = value.trim().replace(/\/+$/, "");
  if (!trimmed) {
    throw new Error("wiki base URL cannot be empty");
  }
  if (hasScheme(trimmed)) {
    return trimmed;
  }
  return `https://github.com${trimmed.startsWith("/") ? "" : "/"}${trimmed}`;
}

function splitTarget(target) {
  const hash = target.indexOf("#");
  if (hash === -1) {
    return { pathname: target, anchor: "" };
  }

  return {
    pathname: target.slice(0, hash),
    anchor: target.slice(hash),
  };
}

function resolveDocsLink(currentRel, target) {
  if (
    !target ||
    target.startsWith("#") ||
    target.startsWith("/") ||
    target.startsWith("?") ||
    hasScheme(target)
  ) {
    return null;
  }

  const { pathname, anchor } = splitTarget(target);
  if (!pathname || pathname.includes(" ")) {
    return null;
  }

  const ext = path.posix.extname(pathname);
  const docsPath = ext === "" ? `${pathname}.md` : pathname;
  if (ext !== "" && ext !== ".md") {
    return null;
  }

  const currentDir = path.posix.dirname(currentRel);
  const resolved = path.posix.normalize(path.posix.join(currentDir, docsPath));
  if (resolved.startsWith("../") || resolved === "..") {
    return null;
  }

  return { resolved, anchor };
}

function wikiTarget(output, anchor) {
  const slug = output.slice(0, -".md".length);
  const pagePath = slug === "Home" ? wikiBaseURL : `${wikiBaseURL}/${slug}`;
  return `${pagePath}${anchor}`;
}

function rewriteMarkdownLinks(source, currentRel, relToOutput) {
  return source.replace(/(!?\[[^\]\n]*\]\()([^)]+)(\))/g, (match, prefix, target, suffix) => {
    if (prefix.startsWith("!")) {
      return match;
    }

    const resolved = resolveDocsLink(currentRel, target);
    if (!resolved) {
      return match;
    }

    const output = relToOutput.get(resolved.resolved);
    if (!output) {
      throw new Error(`${currentRel}: unresolved docs link target ${target}`);
    }

    return `${prefix}${wikiTarget(output, resolved.anchor)}${suffix}`;
  });
}

const files = await walkMarkdown(sourceDir);
const relPaths = files.map((file) => toPosix(path.relative(sourceDir, file)));
const basenameCounts = new Map();

for (const relPath of relPaths) {
  const basename = path.posix.basename(relPath);
  basenameCounts.set(basename, (basenameCounts.get(basename) ?? 0) + 1);
}

const relToOutput = new Map();
const outputs = new Map();

for (const relPath of relPaths) {
  const outputName = outputNameFor(relPath, basenameCounts);
  const existing = outputs.get(outputName);
  if (existing) {
    throw new Error(`wiki output collision: ${existing} and ${relPath} both map to ${outputName}`);
  }
  outputs.set(outputName, relPath);
  relToOutput.set(relPath, outputName);
}

await cleanDestination(destDir);

for (const file of files) {
  const relPath = toPosix(path.relative(sourceDir, file));
  const outputName = relToOutput.get(relPath);
  const source = await readFile(file, "utf8");
  const rendered = rewriteMarkdownLinks(source, relPath, relToOutput);
  const destFile = path.join(destDir, outputName);

  if (existsSync(destFile)) {
    throw new Error(`refusing to overwrite ${destFile}`);
  }

  await writeFile(destFile, rendered);
}

console.log(`Built ${files.length} wiki pages in ${destArg}`);
