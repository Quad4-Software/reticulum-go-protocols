#!/bin/sh
# SPDX-License-Identifier: 0BSD
set -eu
ROOT="$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
task build-librrc
task build-libmf
task build-liblxmf
task build-liblxst
make -C bindings/c test
