#!/bin/sh
# SPDX-License-Identifier: 0BSD
set -eu
ROOT="$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
task build-libmf
task build-liblxmf
task build-liblxst
go test -count=1 ./pkg/libmf/... ./pkg/liblxmf/... ./pkg/liblxst/...
make -C bindings/python test
make -C bindings/python examples/lxmf-roundtrip
make -C bindings/python examples/lxst-roundtrip
make -C bindings/c examples/lxmf-smoke
make -C bindings/c examples/lxmf-interop
make -C bindings/c examples/lxst-smoke
make -C bindings/cpp test
