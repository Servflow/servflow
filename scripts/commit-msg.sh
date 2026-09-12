#!/bin/sh
# Check commit messages against the commit convention named by $guide.
#
#   commit-msg.sh <file>             hook mode: check the message in <file>
#   commit-msg.sh --message <text>   check a message given inline
#   commit-msg.sh --range [<range>]  check every non-merge commit in <range>
#                                    (default origin/main..HEAD)
#
# Allowed surfaces are read from .commit-surfaces and allowed flows from
# .commit-flows at the repo root, one name per line, # comments allowed.
# Editing a registry means editing the table in the guide in the same change.

set -u

guide=https://git.servflow.io/servflow/servflowai/src/branch/main/docs/commit-convention.md
root=$(cd "$(dirname "$0")/.." && pwd)
types='feat|fix|perf|refactor|style|test|docs|build|ci|chore|revert'
header_re="^($types)\\(([a-z0-9-]+)(/[a-z0-9-]+)?\\)(!)?: (.+)\$"

registry() { grep -Ev '^[[:space:]]*(#|$)' "$root/$1" 2>/dev/null; }
listed() { registry "$1" | grep -qx -- "$2"; }

# check <message>: prints one line per problem, returns 1 when any was found.
check() {
	msg=$(printf '%s\n' "$1" | sed '/^# -* >8 -*$/q' | grep -v '^#')
	header=$(printf '%s\n' "$msg" | grep -m1 -v '^[[:space:]]*$')
	case "$header" in
	fixup!\ *|squash!\ *|Merge\ *) return 0 ;;
	esac
	found=0
	fail() { found=1; printf '  - %s\n' "$1"; }

	if printf '%s\n' "$header" | grep -Eq "$header_re"; then
		type=$(printf '%s\n' "$header" | sed -E "s#$header_re#\\1#")
		surface=$(printf '%s\n' "$header" | sed -E "s#$header_re#\\2#")
		bang=$(printf '%s\n' "$header" | sed -E "s#$header_re#\\4#")
		subject=$(printf '%s\n' "$header" | sed -E "s#$header_re#\\5#")
		listed .commit-surfaces "$surface" ||
			fail "surface '$surface' is not in .commit-surfaces: $(registry .commit-surfaces | tr '\n' ' ')"
		case "$subject" in
		[A-Z]*) fail "subject starts with a capital letter; write it lowercase" ;;
		esac
		case "$subject" in
		*.) fail "subject ends with a period; drop it" ;;
		esac
		[ ${#header} -le 72 ] || fail "header is ${#header} characters; the limit is 72"

		flow=$(printf '%s\n' "$msg" | sed -nE 's/^Flow: *([^ ]*).*$/\1/p' | head -1)
		case "$type" in
		feat|fix|perf)
			if [ -z "$flow" ]; then
				fail "a $type commit needs a 'Flow: <flow>' trailer; flows: $(registry .commit-flows | tr '\n' ' ')"
			elif ! listed .commit-flows "$flow"; then
				fail "flow '$flow' is not in .commit-flows: $(registry .commit-flows | tr '\n' ' ')"
			fi
			;;
		esac

		if printf '%s\n' "$msg" | grep -Eq '^BREAKING[ -]CHANGE: '; then
			[ -n "$bang" ] || fail "a BREAKING CHANGE trailer needs '!' after the scope in the header"
		else
			[ -z "$bang" ] || fail "'!' in the header needs a 'BREAKING CHANGE: <what breaks and how to migrate>' trailer"
		fi
	else
		fail "header does not match '<type>(<surface>/<feature>)!: <subject>'"
		fail "types: $(printf '%s' "$types" | tr '|' ' ')"
		fail "surfaces: $(registry .commit-surfaces | tr '\n' ' ')"
	fi

	if printf '%s\n' "$msg" | grep -Eiq '^Co-authored-by:.*(claude|anthropic|copilot|chatgpt|openai|gemini|codex|cursor)'; then
		fail "Co-authored-by is for people; drop the AI attribution trailer"
	fi
	if printf '%s\n' "$msg" | grep -Eiq 'generated with .*(claude|copilot|chatgpt|cursor|codex)'; then
		fail "drop the 'Generated with ...' line; commits carry no AI attribution"
	fi
	return $found
}

report() { # report <label> <message>
	out=$(check "$2")
	[ -z "$out" ] && return 0
	printf '%s\n%s\n' "$1" "$out"
	return 1
}

case "${1:-}" in
--message)
	report "commit message rejected:" "${2:-}" || exit 1
	;;
--range)
	range=${2:-origin/main..HEAD}
	bad=0
	for sha in $(git -C "$root" rev-list --no-merges --reverse "$range"); do
		short=$(git -C "$root" rev-parse --short "$sha")
		report "$short $(git -C "$root" log -1 --format=%s "$sha")" "$(git -C "$root" log -1 --format=%B "$sha")" || bad=1
	done
	[ $bad -eq 0 ] || { echo "see $guide"; exit 1; }
	;;
"")
	sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
	;;
*)
	report "commit message rejected:" "$(cat "$1")" || {
		echo "see $guide; the message is kept in $1"
		exit 1
	}
	;;
esac
