# Go CLI template implementation plan

Build a reusable Go CLI for agents and occasional human use. Default to text;
require `--json` for machine output. Use Cobra, typed YAML, slog, injectable IO,
context cancellation, stable exit codes, and a generated command description.

## Constraints

- Select the latest official LTS, or the latest stable release when no LTS exists;
  resolve and pin versions rather than floating on `latest`.
- Go 1.27.3, gopls 0.24.0, golangci-lint 2.15.0, GoReleaser 2.19.0,
  mise 2026.10.21; Cobra 1.10.2 and YAML v3.0.5.
- Only Codex project integration. Reuse user-level RTK and superpowers.
- No TUI, application MCP server, business SDKs, or global agent configuration changes.
- Retain existing user files. Do not publish or push during implementation.

## Tasks

1. Implement and test CLI/configuration contracts: greet, version, completion,
   schema; stdout results, stderr diagnostics; exit codes 0/1/2/130.
2. Implement and test the standard-library project initializer and portable
   development helpers. Initialization rewrites only template files, is idempotent
   for the same identity, and refuses a conflicting second identity.
3. Configure mise, Codex/gopls, CI, cross-platform releases, and Chinese onboarding.
   Verify the generated project independently, including snapshot packaging and MCP.
4. Review the whole change with a fresh reviewer; fix material findings and verify.

## Acceptance

Text/JSON, IO separation, invalid arguments, configuration precedence and failures,
configuration-independent help/version/completion/schema, cancellation, generated
schema, fresh-project rename/build/test/release, locked tools, and a real MCP handshake
and navigation/diagnostic call. Basic checks work without user-level agent tools.
