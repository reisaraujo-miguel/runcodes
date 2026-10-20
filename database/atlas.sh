#!/bin/sh
#
# Runs the pinned Atlas CLI (Community Edition, Apache 2.0) for this project.
#
# Atlas is downloaded on first use into database/.tools/ and checked against the
# SHA-256 pinned below, so every contributor and CI run uses the same version.
# The script changes to database/ before running it, so atlas.hcl and the
# relative schema/migration URLs resolve regardless of the caller's directory.
#
# Typical use:
#
#   ATLAS_DEV_URL='postgres://runcodes:PASSWORD@localhost:5432/atlas_dev?sslmode=disable&search_path=public' \
#     ./database/atlas.sh migrate diff add_the_column
#
#   ./database/atlas.sh migrate apply --url "postgres://runcodes:PASSWORD@localhost:5432/runcodes?sslmode=disable"
#
# See database/README.md for the full workflow.

set -eu

ATLAS_VERSION="v1.3.0"
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TOOLS_DIR="${SCRIPT_DIR}/.tools"

case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*)
		echo "atlas.sh: unsupported operating system: $(uname -s)" >&2
		exit 1
		;;
esac

case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*)
		echo "atlas.sh: unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
esac

case "${os}-${arch}" in
	linux-amd64) checksum="10d7913e3dce43ab99b8d71534a4cbadaf11a16dc293adf3b91d10e83a0ac70b" ;;
	linux-arm64) checksum="082188c57a53439596a3ee52173b12e785aa83ec718505674e2052db0fec0c4c" ;;
	darwin-amd64) checksum="650981a024301775ec964e5134e2d5712b7ef1b25fec4b2ec54bad762b4bdf6f" ;;
	darwin-arm64) checksum="4e5ffdc10b2b4fd3a06074aba72848150907d5316cacad7b19bae6c6ae3db991" ;;
esac

binary="${TOOLS_DIR}/atlas-${ATLAS_VERSION}-${os}-${arch}"

if [ ! -x "${binary}" ]; then
	command -v curl >/dev/null 2>&1 || {
		echo "atlas.sh: curl is required to download Atlas ${ATLAS_VERSION}" >&2
		exit 1
	}

	mkdir -p "${TOOLS_DIR}"
	tmp="${binary}.tmp.$$"
	trap 'rm -f "${tmp}"' EXIT INT TERM

	url="https://release.ariga.io/atlas/atlas-community-${os}-${arch}-${ATLAS_VERSION}"
	echo "atlas.sh: downloading Atlas ${ATLAS_VERSION} (${os}-${arch})..." >&2
	if ! curl -fsSL -o "${tmp}" "${url}"; then
		echo "atlas.sh: could not download ${url}" >&2
		exit 1
	fi

	if command -v sha256sum >/dev/null 2>&1; then
		actual=$(sha256sum "${tmp}" | cut -d' ' -f1)
	else
		actual=$(shasum -a 256 "${tmp}" | cut -d' ' -f1)
	fi
	if [ "${actual}" != "${checksum}" ]; then
		echo "atlas.sh: checksum mismatch for ${url}" >&2
		echo "  expected: ${checksum}" >&2
		echo "  actual:   ${actual}" >&2
		exit 1
	fi

	chmod +x "${tmp}"
	mv "${tmp}" "${binary}"
	trap - EXIT INT TERM
fi

export ATLAS_NO_UPDATE_NOTIFIER=true

cd "${SCRIPT_DIR}"
exec "${binary}" "$@"
