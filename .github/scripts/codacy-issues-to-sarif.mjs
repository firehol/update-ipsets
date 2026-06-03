#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";

const SARIF_VERSION = "2.1.0";
const MAX_CODACY_ISSUES_PER_QUERY = 1000;
const INPUT_ROOT = path.resolve(process.env.CODACY_SARIF_WORK_DIR || "codacy-sarif");

function fail(message) {
  console.error(message);
  process.exit(1);
}

function loadJson(filePath) {
  const safePath = resolveInputPath(filePath);
  try {
    return JSON.parse(fs.readFileSync(safePath, "utf8")); // nosemgrep: javascript.pathtraversal.rule-non-literal-fs-filename - path is root-confined by resolveInputPath.
  } catch (error) {
    fail(`Failed to read JSON from ${filePath}: ${error.message}`);
  }
}

function resolveInputPath(filePath) {
  const resolved = path.resolve(filePath);
  const relative = path.relative(INPUT_ROOT, resolved);
  if (relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative))) {
    return resolved;
  }
  fail(`JSON input must be under ${path.relative(process.cwd(), INPUT_ROOT) || INPUT_ROOT}: ${filePath}`);
}

function normalizeUri(filePath) {
  return String(filePath || "")
    .replace(/\\/g, "/")
    .replace(/^\.\/+/, "");
}

function issueRuleId(issue) {
  return String(issue?.patternInfo?.id || "codacy.issue");
}

function issueLevel(issue) {
  const severity = String(issue?.patternInfo?.severityLevel || "").toLowerCase();
  if (severity === "critical" || severity === "high" || severity === "error") {
    return "error";
  }
  if (severity === "medium" || severity === "warning") {
    return "warning";
  }
  return "note";
}

function issueMessage(issue) {
  return String(issue?.message || issue?.patternInfo?.id || "Codacy issue");
}

function issueLine(issue) {
  const line = Number(issue?.lineNumber);
  return Number.isInteger(line) && line > 0 ? line : 1;
}

function issueFingerprint(issue) {
  if (issue?.resultDataId !== undefined && issue?.resultDataId !== null) {
    return String(issue.resultDataId);
  }

  return [
    normalizeUri(issue?.filePath),
    issueLine(issue),
    issueRuleId(issue),
    issueMessage(issue),
  ].join(":");
}

function issueKey(issue) {
  return issueFingerprint(issue);
}

function ruleForIssue(issue) {
  const ruleId = issueRuleId(issue);
  const category = String(issue?.patternInfo?.category || "Codacy");
  const subCategory = issue?.patternInfo?.subCategory
    ? ` / ${issue.patternInfo.subCategory}`
    : "";
  const severity = String(issue?.patternInfo?.severityLevel || "unknown");

  return {
    id: ruleId,
    name: ruleId,
    shortDescription: {
      text: `${category}${subCategory}`,
    },
    fullDescription: {
      text: `Codacy ${category}${subCategory} issue with severity ${severity}.`,
    },
    properties: {
      category,
      severity,
      tags: [category].filter(Boolean),
    },
  };
}

function locationForIssue(issue) {
  const uri = normalizeUri(issue?.filePath);
  if (!uri) {
    return null;
  }

  return {
    physicalLocation: {
      artifactLocation: {
        uri,
      },
      region: {
        startLine: issueLine(issue),
      },
    },
  };
}

function codacyResultDataId(issue) {
  if (issue?.resultDataId === undefined || issue?.resultDataId === null) {
    return null;
  }

  return String(issue.resultDataId);
}

function resultForIssue(issue) {
  const location = locationForIssue(issue);

  return {
    ruleId: issueRuleId(issue),
    level: issueLevel(issue),
    message: {
      text: issueMessage(issue),
    },
    ...(location ? { locations: [location] } : {}),
    partialFingerprints: {
      codacyResultDataId: issueFingerprint(issue),
    },
    properties: {
      codacyCategory: issue?.patternInfo?.category || null,
      codacySubCategory: issue?.patternInfo?.subCategory || null,
      codacySeverity: issue?.patternInfo?.severityLevel || null,
      codacyResultDataId: codacyResultDataId(issue),
      falsePositiveThreshold: issue?.falsePositiveThreshold ?? null,
    },
  };
}

function plan(overviewPath) {
  const overview = loadJson(overviewPath).overview;
  if (!overview || !Array.isArray(overview.categories)) {
    fail("Codacy overview JSON does not include overview.categories.");
  }

  const oversized = overview.categories.filter(
    (category) => Number(category.total || 0) > MAX_CODACY_ISSUES_PER_QUERY,
  );
  if (oversized.length > 0) {
    const names = oversized
      .map((category) => `${category.name} (${category.total})`)
      .join(", ");
    fail(
      `Codacy categories exceed the CLI per-query limit of ${MAX_CODACY_ISSUES_PER_QUERY}: ${names}. Split those categories before exporting all SARIF.`,
    );
  }

  for (const category of overview.categories) {
    if (category.name && Number(category.total || 0) > 0) {
      console.log(category.name);
    }
  }
}

function sarif(issuePaths) {
  if (issuePaths.length === 0) {
    fail("No Codacy issue JSON files were provided.");
  }

  const rules = new Map();
  const results = [];
  const seen = new Set();

  for (const issuePath of issuePaths) {
    const payload = loadJson(issuePath);
    if (!Array.isArray(payload.issues)) {
      fail(`Codacy issue JSON file ${issuePath} does not include an issues array.`);
    }

    for (const issue of payload.issues) {
      const key = issueKey(issue);
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);

      const ruleId = issueRuleId(issue);
      if (!rules.has(ruleId)) {
        rules.set(ruleId, ruleForIssue(issue));
      }
      results.push(resultForIssue(issue));
    }
  }

  const output = {
    $schema:
      "https://json.schemastore.org/sarif-2.1.0.json",
    version: SARIF_VERSION,
    runs: [
      {
        tool: {
          driver: {
            name: "Codacy Cloud",
            informationUri: "https://app.codacy.com/gh/firehol/update-ipsets",
            rules: [...rules.values()].sort((left, right) =>
              left.id.localeCompare(right.id),
            ),
          },
        },
        automationDetails: {
          id: "codacy",
        },
        results,
      },
    ],
  };

  process.stdout.write(`${JSON.stringify(output, null, 2)}\n`);
}

function summary(sarifPath) {
  const payload = loadJson(sarifPath);
  const results = payload?.runs?.[0]?.results || [];
  const levels = new Map();
  for (const result of results) {
    levels.set(result.level, (levels.get(result.level) || 0) + 1);
  }

  console.log("## Codacy SARIF Export");
  console.log("");
  console.log(`- Results: ${results.length}`);
  console.log(`- Rules: ${payload?.runs?.[0]?.tool?.driver?.rules?.length || 0}`);
  for (const level of ["error", "warning", "note"]) {
    console.log(`- ${level}: ${levels.get(level) || 0}`);
  }
}

const [mode, ...args] = process.argv.slice(2);

if (mode === "plan") {
  if (args.length !== 1) {
    fail("Usage: codacy-issues-to-sarif.mjs plan <overview.json>");
  }
  plan(args[0]);
} else if (mode === "sarif") {
  sarif(args.filter((arg) => path.basename(arg) !== "*.json"));
} else if (mode === "summary") {
  if (args.length !== 1) {
    fail("Usage: codacy-issues-to-sarif.mjs summary <results.sarif>");
  }
  summary(args[0]);
} else {
  fail(
    "Usage: codacy-issues-to-sarif.mjs <plan|sarif|summary> [arguments...]",
  );
}
