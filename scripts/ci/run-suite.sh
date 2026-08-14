#!/bin/sh
# Run a CI test or lint suite (POSIX, vendor-mode Go).
# Usage: run-suite.sh <suite>
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

. "$ROOT/scripts/ci/env.sh"
export PATH="/usr/local/go/bin:${HOME:-/tmp}/.local/bin:${PATH}"

SUITE="${1:?suite name required}"
GOCMD="${GOCMD:-go}"

case "$SUITE" in
fmt)
	sh "$ROOT/scripts/ci/fmt-check.sh"
	;;
vet)
	"$GOCMD" vet ./...
	;;
lint)
	revive -config revive.toml -formatter friendly ./pkg/...
	;;
scan)
	gosec ./...
	;;
check)
	task check
	;;
test-short)
	"$GOCMD" test -short -count=1 -timeout 20m ./...
	;;
test)
	"$GOCMD" test -count=1 -timeout 30m ./...
	;;
test-race)
	"$GOCMD" test -race -short -count=1 -timeout 20m \
		./internal/leaktest/... ./pkg/lxmf/... ./pkg/mf/... ./pkg/rrc/...
	;;
live)
	"$GOCMD" test -count=1 -timeout 25m \
		-run 'Test(E2E_|HubClientLoopback|Messenger_TwoWay)' \
		./pkg/mf/... ./pkg/lxmf/... ./pkg/rrc/...
	;;
examples)
	# examples/*.go use //go:build ignore; single-file go build still type-checks them.
	# lxmf_send.go needs reticulumconfig from a newer Reticulum-Go than this vendor pin.
	for base in example.go messenger.go; do
		f="$ROOT/examples/$base"
		if [ ! -f "$f" ]; then
			echo "examples: missing $f" >&2
			exit 1
		fi
		echo "build example: $f"
		"$GOCMD" build -o /dev/null "$f"
	done
	;;
interop-lxmf)
	sh "$ROOT/scripts/ci/clone-refs.sh"
	task test:lxmf:interop
	;;
interop-rrc)
	sh "$ROOT/scripts/ci/clone-refs.sh"
	task test:rrc:interop
	;;
interop)
	sh "$ROOT/scripts/ci/clone-refs.sh"
	task test:lxmf:interop
	task test:rrc:interop
	;;
*)
	echo "run-suite: unknown suite: $SUITE" >&2
	exit 1
	;;
esac
