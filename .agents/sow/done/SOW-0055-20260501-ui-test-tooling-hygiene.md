# SOW-0055 | 2026-05-01 | ui-test-tooling-hygiene

## Status

completed

## Requirements

### Purpose

Clean up small but recurring UI test tooling gaps before they become accepted
noise.

### Scope

- Re-check whether `eslint-plugin-jest-dom` supports the repo's ESLint major;
  adopt it only if peer ranges are compatible without overrides.
- Investigate the Vitest/Node `--localstorage-file` warning and either fix it
  or document it as verified upstream noise.
- Add tiny MSW request-capture helpers only if repeated tests need them.

### Acceptance criteria

- No peer-dependency overrides for lint plugins without evidence.
- The warning is either gone or explicitly documented with upstream evidence.
- Any helper added reduces real duplication and does not hide request details.

## Analysis

Created from SOW-0041 Decisions 5 = (a) and 6 = (a), plus the C-new-2 helper
suggestion if the page-test batch proves duplication.

Findings:

- `eslint-plugin-jest-dom@5.5.0` is not compatible with the repo's current
  ESLint major without overrides. `pnpm view eslint-plugin-jest-dom@latest
  version peerDependencies` reports `eslint: ^6.8.0 || ^7.0.0 || ^8.0.0 ||
  ^9.0.0`; the repo uses `eslint@10.2.1`.
- The `--localstorage-file` warning is caused by Node 25 Web Storage globals
  interacting with Vitest/jsdom. Local evidence: `node --help` exposes
  `--localstorage-file` and `--no-experimental-webstorage`, and
  `NODE_OPTIONS="--no-experimental-webstorage" pnpm --dir ui test` removes the
  warning while preserving test results.
- Upstream evidence: Node 25 documents global `localStorage` and the
  `--no-experimental-webstorage`/`--no-webstorage` disable path; Vitest issue
  `vitest-dev/vitest#8757` tracks the Node 25 test-runner symptom.
- Browser smoke validation also exposed a separate local color-env warning:
  Node warns when `NO_COLOR` and Playwright's forced color output meet. Clearing
  `NO_COLOR` at the Playwright script boundary removes the warning without
  hiding test output.
- The admin request-capture pattern is currently one helper with three local
  call sites in one test file. Adding a second abstraction now would hide useful
  request details more than it would reduce duplication.

## Plan

1. Verify current package metadata for `eslint-plugin-jest-dom` before adding
   any lint dependency.
2. Reproduce and isolate the `--localstorage-file` warning against the local
   Vitest/jsdom/Node stack.
3. Fix the warning if it is caused by local config; otherwise document the
   verified upstream/runtime cause in the SOW and skills.
4. Inspect the new admin-action request capture usage and add a helper only if
   it removes real repeated code without hiding request details.
5. Run UI tests/lint/build, the browser smoke gate, and project build/test.

## Validation

- `NODE_OPTIONS="${NODE_OPTIONS:-} --no-experimental-webstorage" pnpm --dir ui
  test` - passed; no `--localstorage-file` warning.
- `pnpm --dir ui test` - passed; package script suppresses the Node 25 warning.
- `pnpm --dir ui lint` - passed.
- `pnpm --dir ui build` - passed; existing static font resolution warnings
  remain unchanged.
- `make ui-test` - passed.
- `make ui-e2e` - passed; Playwright color-env warning removed.
- `make build` - passed.
- `make test` - passed.

## Outcome

- Kept `eslint-plugin-jest-dom` out of the dependency tree because the latest
  peer range does not include ESLint 10.
- Made the Vitest scripts pass `--no-experimental-webstorage` through
  `NODE_OPTIONS`, preserving any existing `NODE_OPTIONS` while disabling the
  Node 25 global Web Storage path that emits the warning.
- Made the Playwright scripts clear `NO_COLOR` to avoid local color-env
  warnings when the runner forces colored output.
- Did not add a request-capture helper; the existing helper is sufficient and
  the remaining local arrays are clearer than another abstraction.

## Lessons extracted

- Test-tooling warnings deserve the same treatment as product regressions:
  isolate whether they are local config, runtime behavior, or upstream tool
  behavior, then encode the verified workaround in package scripts and project
  skills.
