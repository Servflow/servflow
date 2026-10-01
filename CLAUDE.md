# ServFlow engine: agent rules

This module is a library: the actions and integrations a ServFlow host
registers, plus the types they compile against (`pkg/engine/actions`,
`pkg/engine/integration`, `pkg/engine/requestctx`, `pkg/engine/kv`,
`pkg/logging`). Each action and integration package exports a `Definition`
function and has no `init()`; the host registers what it wants. There are no
registries, planner, server, secret manager, or binary here; servflowai runs
requests. Nothing here may import servflowai.

## Commits follow the ServFlow commit convention

Header: `<type>(<surface>/<feature>)!: <subject>`, 72 characters or fewer,
imperative, lowercase first letter, no trailing period, describes the effect
and not the file. Types: `feat fix perf refactor style test docs build ci
chore revert`. Surfaces for this repo, the only scopes allowed: `actions
integrations requestctx shared infra`. The feature segment is
free-form and optional.

Footer trailers: `Flow: <flow>` is required on `feat`, `fix`, and `perf`,
from `.commit-flows`. `Refs: #N` when an issue exists. `!` in the header
pairs with a `BREAKING CHANGE:` trailer. Never add an AI `Co-authored-by` or
a "Generated with" line.

Pull requests are squash-merged: the PR title is the header, the PR body is
the body and footer. `make hooks` installs the check together with the
pre-push lint; `make lint-commits` checks a branch. Full guide, registries,
and examples: https://git.servflow.io/servflow/servflowai/src/branch/main/docs/commit-convention.md
