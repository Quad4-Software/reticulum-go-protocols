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
	gosec --exclude-rules='.*/go-build/.*:G115' ./...
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
		./internal/leaktest/... ./internal/gorrcd/... ./pkg/lxmf/... ./pkg/mf/... ./pkg/rrc/... ./pkg/lxst/...
	;;
live)
	"$GOCMD" test -count=1 -timeout 25m \
		-run 'Test(E2E_|HubClientLoopback|Messenger_TwoWay)' \
		./pkg/mf/... ./pkg/lxmf/... ./pkg/rrc/... ./internal/gorrcd/...
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
	build)
	sh "$ROOT/scripts/ci/build-gorrcd.sh" "$("$GOCMD" env GOOS)" "$("$GOCMD" env GOARCH)" bin
	"$ROOT/bin/gorrcd-$("$GOCMD" env GOOS)-$("$GOCMD" env GOARCH)" --version
	sh "$ROOT/scripts/ci/build-golxmd.sh" "$("$GOCMD" env GOOS)" "$("$GOCMD" env GOARCH)" bin
	golxmd_bin="$ROOT/bin/golxmd-$("$GOCMD" env GOOS)-$("$GOCMD" env GOARCH)"
	"$golxmd_bin" --version
	GOLXMD_HOME="$(mktemp -d)" "$golxmd_bin" --self-check
	;;
interop-lxmf)
	sh "$ROOT/scripts/ci/clone-refs.sh"
	task test:lxmf:interop
	task test:lxmf:golxmd
	;;
interop-rrc)
	sh "$ROOT/scripts/ci/clone-refs.sh"
	task test:rrc:interop
	;;
interop-lxst)
	sh "$ROOT/scripts/ci/run-lxst-interop.sh"
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
