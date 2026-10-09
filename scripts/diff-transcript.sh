#!/usr/bin/env bash
# Build labntp at the merge-base and at HEAD, run the scenario twice for each
# binary, and fail when the normalized transcripts differ.
set -euo pipefail

if [ "$(uname -s)" != "Linux" ]; then
	echo "diff-transcript: linux only" >&2
	exit 1
fi

cd "$(git rev-parse --show-toplevel)"

export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"

if ! git rev-parse --verify --quiet origin/main >/dev/null; then
	git fetch origin main
fi

base="${DIFFTRANSCRIPT_BASE:-$(git merge-base HEAD origin/main)}"
head="${DIFFTRANSCRIPT_HEAD:-HEAD}"
base="$(git rev-parse "$base")"
head="$(git rev-parse "$head")"

out="${DIFFTRANSCRIPT_OUT:-difftranscript-out}"
rm -rf "$out"
mkdir -p "$out"

tmp="$(mktemp -d)"
cleanup() {
	git worktree remove --force "$tmp/base" >/dev/null 2>&1 || true
	git worktree remove --force "$tmp/head" >/dev/null 2>&1 || true
	rm -rf "$tmp"
}
trap cleanup EXIT

build_one() {
	local sha="$1"
	local dest="$2"
	local bin="$3"
	git worktree add --detach "$dest" "$sha"
	(cd "$dest" && go build -o "$bin" ./cmd/labntp)
}

build_one "$base" "$tmp/base" "$tmp/labntp-base"
if [ "$base" = "$head" ]; then
	cp "$tmp/labntp-base" "$tmp/labntp-head"
else
	build_one "$head" "$tmp/head" "$tmp/labntp-head"
fi

go test -c -o "$tmp/driver" ./internal/testutil/difftranscript

run_once() {
	local bin="$1"
	local dest="$2"
	timeout 12m "$tmp/driver" -mode=scenario -binary "$bin" -fixture "$tmp/fixture" -out "$dest"
}

self_diff() {
	local label="$1"
	local bin="$2"
	local a="$3"
	local b="$4"
	run_once "$bin" "$a"
	run_once "$bin" "$b"
	if ! cmp -s "$a" "$b"; then
		echo "diff-transcript: self-diff $label failed; the difference is not covered by a named rule" >&2
		diff -u "$a" "$b" | head -n 200 >&2 || true
		exit 1
	fi
}

self_diff base "$tmp/labntp-base" "$tmp/base-a.txt" "$tmp/base-b.txt"
cp "$tmp/base-a.txt" "$out/base.txt"
if [ "$base" = "$head" ]; then
	cp "$tmp/base-a.txt" "$out/head.txt"
else
	self_diff head "$tmp/labntp-head" "$tmp/head-a.txt" "$tmp/head-b.txt"
	cp "$tmp/head-a.txt" "$out/head.txt"
fi

if ! cmp -s "$out/base.txt" "$out/head.txt"; then
	diff -u "$out/base.txt" "$out/head.txt" >"$out/diff.txt" || true
	echo "diff-transcript: transcripts differ" >&2
	cat "$out/diff.txt" >&2
	exit 1
fi

echo "diff-transcript: empty (base $base head $head)"
