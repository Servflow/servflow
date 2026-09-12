# ServFlow engine: agent rules

## Commits follow the ServFlow commit convention

Header: `<type>(<surface>/<feature>)!: <subject>`, 72 characters or fewer,
imperative, lowercase first letter, no trailing period, describes the effect
and not the file. Types: `feat fix perf refactor style test docs build ci
chore revert`. Surfaces for this repo, the only scopes allowed: `plan
actions entries integrations requestctx responses secrets server config
tracing shared infra`. The feature segment is free-form and optional.

Footer trailers: `Flow: <flow>` is required on `feat`, `fix`, and `perf`,
from `.commit-flows`. `Refs: #N` when an issue exists. `!` in the header
pairs with a `BREAKING CHANGE:` trailer. Never add an AI `Co-authored-by` or
a "Generated with" line.

Pull requests are squash-merged: the PR title is the header, the PR body is
the body and footer. `make hooks` installs the check together with the
pre-push lint; `make lint-commits` checks a branch. Full guide, registries,
and examples: https://git.servflow.io/servflow/servflowai/src/branch/main/docs/commit-convention.md
