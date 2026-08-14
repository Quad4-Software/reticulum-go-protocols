#!/bin/sh
# Shallow-clone upstream reference trees used by Python interop harnesses.
# Usage: clone-refs.sh
# Env:
#   LXMF_REF_REV   git ref for markqvist/LXMF (tag or 40-char SHA, default 1.1.0)
#   RRC_REF_REV    git ref for kc1awv/rrcd (tag or 40-char SHA)
set -eu

ROOT="$(CDPATH='' cd -- "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

LXMF_REF_REV="${LXMF_REF_REV:-1.1.0}"
RRC_REF_REV="${RRC_REF_REV:-f6d7e9d72bf83c70d7a9373ae8c5edaa052a7bf2}"

is_sha() {
	# 40 hex chars: a pinned commit.
	printf '%s' "$1" | grep -Eq '^[0-9a-fA-F]{40}$'
}

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
	if is_sha "$rev"; then
		git init "$dest"
		git -C "$dest" remote add origin "$url"
		git -C "$dest" fetch --depth 1 origin "$rev"
		git -C "$dest" checkout --detach FETCH_HEAD
	else
		git clone --depth 1 --branch "$rev" "$url" "$dest"
	fi
	echo "clone-refs: $name at $rev"
}

clone_ref LXMF-ref https://github.com/markqvist/LXMF.git "$LXMF_REF_REV"
clone_ref RRC-ref https://github.com/kc1awv/rrcd.git "$RRC_REF_REV"

echo "clone-refs: OK"
