#!/usr/bin/env bash
# R7 auth fence. git grep -nE (POSIX ERE) on tracked non-_test.go Go files.
# Prints file:line:rule for each hit outside the per-rule exact-path allowlist.
# Exits 1 when any hit remains.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
export LC_ALL=C

# Per-rule exact-path allowlist. One "<rule> <path>" pair per line.
# An entry exempts that one path from that one rule. No path is exempt from
# every rule. ntp has no cursor signers (no internal/control/rest/cursor.go
# and no internal/control/mcp/cursor.go), so every rule's list is empty.
# internal/auth/controlkit.go and internal/auth/errors.go are exempt from nothing.
allowlist_entries=""

allowed() {
	local rule=$1 file=$2 r p
	if [ -z "$allowlist_entries" ]; then
		return 1
	fi
	while read -r r p; do
		if [ -z "${r:-}" ]; then
			continue
		fi
		if [ "$r" = "$rule" ] && [ "$p" = "$file" ]; then
			return 0
		fi
	done <<< "$allowlist_entries"
	return 1
}

auth_go=(
	':(glob)internal/auth/*.go'
	':(glob)internal/auth/**/*.go'
	':!*_test.go'
)
auth_control_go=(
	':(glob)internal/auth/*.go'
	':(glob)internal/auth/**/*.go'
	':(glob)internal/control/*.go'
	':(glob)internal/control/**/*.go'
	':!*_test.go'
)
all_go=(
	':(glob)*.go'
	':(glob)**/*.go'
	':!*_test.go'
)

hits=$(mktemp)
chunk=$(mktemp)
trap 'rm -f "$hits" "$chunk"' EXIT
: > "$hits"
: > "$chunk"

scan() {
	local rule=$1 pattern=$2
	shift 2
	local out rc line file rest lineno
	if out=$(git -c color.ui=never grep -nE -I -- "$pattern" "$@"); then
		rc=0
	else
		rc=$?
	fi
	if [ "$rc" -eq 1 ]; then
		return 0
	fi
	if [ "$rc" -ne 0 ]; then
		printf 'check-auth-fence: git grep failed for %s (exit %s)\n' "$rule" "$rc" >&2
		exit 2
	fi
	while IFS= read -r line; do
		if [ -z "$line" ]; then
			continue
		fi
		file=${line%%:*}
		rest=${line#*:}
		lineno=${rest%%:*}
		if allowed "$rule" "$file"; then
			continue
		fi
		printf '%s:%s:%s\n' "$file" "$lineno" "$rule" >> "$chunk"
	done <<< "$out"
}

commit_chunk() {
	if [ -s "$chunk" ]; then
		sort -t: -k1,1 -k2,2n -u "$chunk" >> "$hits"
	fi
	: > "$chunk"
}

# subtle, digest: internal/auth and internal/control.
scan subtle '"crypto/subtle"' "${auth_control_go[@]}"
commit_chunk

scan digest '"crypto/sha256"|sha256\.(Sum256|New)\(' "${auth_control_go[@]}"
commit_chunk

# sessions, tokenfile: internal/auth only. Two sessions patterns, one rule.
scan sessions 'map\[string\]\*?(record|[A-Za-z_]*[Ss]ession[A-Za-z_]*)' "${auth_go[@]}"
scan sessions 'type[[:space:]]+Store[[:space:]]+struct' "${auth_go[@]}"
commit_chunk

scan tokenfile 'os\.(ReadFile|Open|OpenFile)\(|bufio\.NewScanner\(' "${auth_go[@]}"
commit_chunk

# byid, allowall: repo-wide non-test Go.
scan byid '\bPrincipalByID\b' "${all_go[@]}"
commit_chunk

scan allowall 'authn\.AllowAll\(' "${all_go[@]}"
commit_chunk

# replace: go.mod only.
scan replace '^replace .*go-lab-controlkit' go.mod
commit_chunk

# gowork: either tracked path fails. git ls-files has no line number;
# a hit is printed as <path>:1:gowork.
gowork_tracked=$(git ls-files -- go.work go.work.sum)
if [ -n "$gowork_tracked" ]; then
	while IFS= read -r f; do
		if [ -z "$f" ]; then
			continue
		fi
		if allowed gowork "$f"; then
			continue
		fi
		printf '%s:1:%s\n' "$f" gowork >> "$chunk"
	done <<< "$gowork_tracked"
fi
commit_chunk

if [ -s "$hits" ]; then
	cat "$hits"
	exit 1
fi
