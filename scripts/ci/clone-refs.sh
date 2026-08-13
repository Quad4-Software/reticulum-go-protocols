#!/bin/sh
# Shallow-clone upstream reference trees used by Python interop harnesses.
# Usage: clone-refs.sh
# Env:
#   LXMF_REF_REV   git ref for markqvist/LXMF (default 1.1.0)
#   RRC_REF_REV    git ref for kc1awv/rrcd (default main)
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

LXMF_REF_REV="${LXMF_REF_REV:-1.1.0}"
RRC_REF_REV="${RRC_REF_REV:-main}"

clone_ref() {
	name="$1"
	url="$2"
	rev="$3"
	dest="$ROOT/$name"
	if [ -d "$dest/.git" ]; then
		echo "clone-refs: $name already present"
		return 0
	fi
	rm -rf "$dest"
	git clone --depth 1 --branch "$rev" "$url" "$dest"
}

clone_ref LXMF-ref https://github.com/markqvist/LXMF.git "$LXMF_REF_REV"
clone_ref RRC-ref https://github.com/kc1awv/rrcd.git "$RRC_REF_REV"

echo "clone-refs: OK"
