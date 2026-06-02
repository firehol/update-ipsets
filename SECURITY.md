# Security Policy

## Reporting Vulnerabilities

Report security vulnerabilities through GitHub private vulnerability reporting
for this repository.

Do not open public issues for vulnerabilities, exploit details, credentials, or
private operational data.

When reporting, include:

- affected version or commit;
- affected component or command;
- reproduction steps;
- expected and actual impact;
- any safe, redacted proof of concept.

## Scope

Security reports are in scope for:

- the Go daemon, CLI, and web/API surfaces;
- installer and systemd deployment behavior;
- GitHub Actions and release automation;
- feed acquisition, parsing, publication, and serving logic;
- frontend code that affects authentication, admin actions, or public data
  interpretation.

Scanner findings are treated as actionable until fixed, narrowly baselined, or
rejected with evidence.
