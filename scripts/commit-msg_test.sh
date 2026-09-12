#!/bin/sh
# Fixture test for scripts/commit-msg.sh. Uses only surfaces present in every
# repo's registry (infra, shared) so the same test runs in pro, engine, builder.
set -u
dir=$(cd "$(dirname "$0")" && pwd)
fails=0
expect() { # expect pass|fail <message>
	if "$dir/commit-msg.sh" --message "$2" >/dev/null 2>&1; then got=pass; else got=fail; fi
	if [ "$got" != "$1" ]; then
		fails=$((fails + 1))
		printf 'expected %s, got %s:\n%s\n\n' "$1" "$got" "$2"
	fi
}
nl='
'
expect pass "build(infra): upgrade goreleaser"
expect pass "refactor(shared/logging): drop the unused field"
expect pass "feat(infra): add a release target${nl}${nl}Flow: set-up-instance"
expect pass "fix(shared/logging)!: rename the level field${nl}${nl}BREAKING CHANGE: level is now severity.${nl}Flow: inspect-trace"
expect pass "perf(shared): cache the parsed registry${nl}${nl}Body text here.${nl}${nl}Flow: build-agent${nl}Refs: #12"
expect pass "chore(infra): tidy go.mod${nl}${nl}Co-authored-by: Ada Lovelace <ada@example.com>"
expect pass "fixup! feat(infra): add a release target"
expect pass "Merge branch 'main' into feat/x"
expect pass "docs(infra): explain hooks${nl}# a comment line git will strip"
expect fail "Add a release target"
expect fail "feat: add a release target${nl}${nl}Flow: set-up-instance"
expect fail "feat(login): add a release target${nl}${nl}Flow: set-up-instance"
expect fail "feat(infra): add a release target"
expect fail "feat(infra): add a release target${nl}${nl}Flow: make-money"
expect fail "feat(infra): Add a release target${nl}${nl}Flow: set-up-instance"
expect fail "feat(infra): add a release target.${nl}${nl}Flow: set-up-instance"
expect fail "feat(infra)!: add a release target${nl}${nl}Flow: set-up-instance"
expect fail "refactor(infra): drop the field${nl}${nl}BREAKING CHANGE: gone"
expect fail "feat(Infra/Build): add a release target${nl}${nl}Flow: set-up-instance"
expect fail "feat(infra/build): add a release target that is far too long for the seventy-two limit${nl}${nl}Flow: set-up-instance"
expect fail "chore(infra): tidy go.mod${nl}${nl}Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
expect fail "chore(infra): tidy go.mod${nl}${nl}Generated with Claude Code"
expect fail "feature(infra): add x${nl}${nl}Flow: set-up-instance"
if [ $fails -eq 0 ]; then echo "commit-msg: all cases pass"; else echo "commit-msg: $fails case(s) wrong"; exit 1; fi
