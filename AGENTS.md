# Project instructions

## Start here

- Read README.md for setup, CLI contracts and repository layout.
- Run commands from the repository root through mise. First run `mise run setup`,
  then `mise run doctor`. Requires Git and mise 2026.10.21 or newer.
- Reuse user-level RTK and superpowers when available. Follow their installed
  instructions; do not install copies here or add personal paths.
- Only Codex is supported. Keep personal models, credentials, permissions and
  plugin preferences in user configuration.

## Implementation

- The root entry point owns signals and process exit. `internal/cli` owns Cobra
  commands, argument validation, IO and exit-code mapping. Business logic accepts
  context and typed options in its own internal package.
- Construct fresh commands per invocation. Pass dependencies explicitly; avoid
  package-level mutable commands, configuration and logger state.
- Extend the typed configuration loader. Resolve explicit flags > environment >
  YAML > defaults. Check `Changed` before applying a flag override.
- Commands must run entirely from arguments. Missing input fails immediately.
  Do not add prompts, automatic pagers, spinners or a default TUI.
- Default output is text. `--json` writes one JSON document to stdout. Logs and
  errors go to stderr; failed commands leave stdout empty. Help and completion
  retain native formats; schema always emits JSON.
- Exit codes: 0 success, 1 runtime/configuration failure, 2 invalid invocation,
  130 cancellation. Validate arguments before beginning command execution.
- New commands need validation, help/examples, tests and typed results. Schema
  derives from the command tree; annotate required flags with `MarkFlagRequired`.
- `greet` and `internal/greeting` are removable examples. Update registration and
  tests when replacing them with application behavior.

## Go tools and verification

- Use gopls MCP for Go symbols, references and diagnostics. Read its instructions
  with `mise exec -- gopls mcp -instructions`. Discover the workspace, inspect
  affected symbols, and check diagnostics after editing. Shell checks remain
  the verification authority.
- Run focused tests while editing. Before handoff run `mise run fmt` and
  `mise run check`; also run `mise run test:race` on a supported host.
- For template/identity changes run `mise run template:check`. Update the managed
  file list when new files embed the application identity.
- For tool integration changes run `mise run mcp:check` and confirm
  `codex mcp list` includes gopls in a trusted project session.
- For release changes run `mise run release:snapshot`; it does not publish.
  Explain checks that could not run rather than claiming they passed.

## Dependencies and maintenance

- Use the latest official LTS, or latest stable release where no LTS exists.
  Exclude alpha, beta, RC and nightly versions. Pin resolved versions.
- Keep mise.toml, mise.lock, Go module/toolchain directives, doctor expectations,
  CI bootstrap version and documented versions consistent during upgrades.
- Lock Linux, macOS and Windows on amd64 and arm64. Basic checks must work
  without Codex, RTK or superpowers installed.
- Update tests, help and docs with behavior changes. Keep machine-readable
  fields and exit codes compatible unless a change is deliberate.
- Never copy local secrets, caches or user-level agent configuration into
  generated projects. Initialization must not change remotes, commit or publish.
