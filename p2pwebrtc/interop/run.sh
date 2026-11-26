#!/usr/bin/env bash
# interop between this package and js-libp2p's webrtc transport (node, node-datachannel), both ways:
#   1. js dials the go runner through the go relay (the browser to runner case).
#   2. the go runner dials a js listener through the go relay with Transport.Connect.
# requires go and node >= 22. installs the js dependencies on first use.
set -euo pipefail
cd "$(dirname "$0")"

[ -d node_modules ] || npm install --silent --no-audit --no-fund

work="$(mktemp -d)"
pids=()
cleanup() {
	exec 3>&- || true
	for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
	wait 2>/dev/null || true
	rm -rf "$work"
}
trap cleanup EXIT

# waitline <file> <count>: wait for the file to have count lines.
waitline() {
	for _ in $(seq 300); do
		[ "$(wc -l < "$1")" -ge "$2" ] && return 0
		sleep 0.1
	done
	echo "timed out waiting for $1" >&2
	cat "$work"/*.log >&2 || true
	return 1
}

# field <file> <line> <key>
field() {
	node -e 'const l=require("fs").readFileSync(process.argv[1],"utf8").split("\n")[process.argv[2]-1];const v=JSON.parse(l)[process.argv[3]];if(v!==undefined)console.log(v)' "$1" "$2" "$3"
}

# GOWORK=off: the module builds on its own, also when vendored into a parent workspace.
(cd .. && GOWORK=off go build -o "$work/runner" ./interop)
mkfifo "$work/in"
"$work/runner" < "$work/in" > "$work/runner.out" 2> "$work/runner.log" &
pids+=($!)
exec 3> "$work/in"
waitline "$work/runner.out" 1

relay="$(field "$work/runner.out" 1 relay)"
target="$(field "$work/runner.out" 1 target)"

echo "== js dials go: $target"
node dial.mjs "$target"

node listen.mjs "$relay" > "$work/listen.out" 2> "$work/listen.log" &
pids+=($!)
waitline "$work/listen.out" 1
remote="$(field "$work/listen.out" 1 target)"

echo "== go dials js: $remote"
echo "$remote" >&3
waitline "$work/runner.out" 2
failure="$(field "$work/runner.out" 2 error)"
if [ -n "$failure" ]; then
	echo "go dial failed: $failure" >&2
	cat "$work/listen.log" >&2
	exit 1
fi
cat "$work/listen.log"
echo "== interop ok"
