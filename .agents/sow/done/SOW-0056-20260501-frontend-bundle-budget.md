# SOW-0056 | 2026-05-01 | frontend-bundle-budget

## Status

completed

## Requirements

### Purpose

Add a measured frontend bundle-size budget so large UI dependencies and route
boundary regressions are caught intentionally instead of by manual inspection.

### Scope

- Evaluate lightweight bundle budget tooling compatible with Vite 8 and pnpm.
- Define budgets for the public entry shell and selected lazy route chunks.
- Keep the budget realistic for the current app and document intentional large
  chunks such as feed detail visualizations.

### Acceptance criteria

- Budget checks run locally and in CI without fragile hash-specific paths.
- Public-shell growth fails or warns according to explicit thresholds.
- The check does not require generated frontend assets to be committed.

## Analysis

Created from SOW-0041 Decision 3 = (a). This is separate from behavioral tests
because it protects performance and route splitting, not DOM behavior.

Current build measurements before this SOW:

- Public entry JS: `index-*.js` about 466 kB raw / 148 kB gzip.
- Public entry CSS: `index-*.css` about 100 kB raw / 16 kB gzip.
- Feed detail route: `feed-detail-*.js` about 473 kB raw / 134 kB gzip;
  intentionally large because it owns visualization dependencies.
- Admin route chunks: `admin-*.js` plus `admin-layout-*.js` about 137 kB raw
  combined.

Tooling decision:

- Use a small repo-local Node script instead of adding a bundle-size package.
  This keeps the check compatible with Vite 8/pnpm, avoids another dependency
  during release hardening, and lets budgets match Vite's stable chunk-name
  prefixes while ignoring content hashes.

## Plan

1. Add a source-controlled bundle budget config with stable regex patterns for
   public shell and selected lazy route chunks.
2. Add a dependency-free Node checker that reads `ui/dist/assets`, calculates
   raw and gzip sizes, warns near limits, and fails above explicit thresholds.
3. Add focused unit tests for missing files, over-budget failures, and passing
   grouped chunks.
4. Wire local scripts, Makefile, and CI so the budget runs after a production
   build without committing generated assets.
5. Run UI build, budget, tests, lint, browser smoke, and project build/test.

## Validation

- `pnpm --dir ui test:bundle-budget` - passed.
- `pnpm --dir ui bundle-budget` - passed against current production build.
- `make ui-budget` - passed.
- `make ui-test` - passed; now includes the bundle-budget unit tests.
- `pnpm --dir ui lint` - passed.
- `make ui-e2e` - passed.
- `make build` - passed.
- `make test` - passed.
- `git diff --check` - passed.

## Outcome

- Added `ui/bundle-budget.config.mjs` with explicit raw/gzip budgets for the
  public shell, home route, feed detail visualization route, admin route, and
  entity detail routes.
- Added `ui/scripts/check-bundle-budget.mjs`, a dependency-free checker that
  reads built assets, matches stable Vite chunk-name prefixes, reports raw and
  gzip usage, warns near limits, and fails above limits or when required chunks
  disappear.
- Added `ui/scripts/check-bundle-budget.test.mjs` covering grouped route
  chunks, missing chunks, over-budget failures, and report formatting.
- Added local scripts, `make ui-budget`, and a CI bundle-budget check after the
  production UI build.
- Updated project testing/frontend guidance so route/dependency changes run the
  budget gate.

## Lessons extracted

- Bundle budgets should be enforced from source-controlled config and generated
  build output, not from committed assets or hash-specific filenames.
