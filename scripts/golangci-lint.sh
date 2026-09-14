#!/usr/bin/env bash
# Runs the pinned golangci-lint *release binary*, the same artifact CI uses.
#
# Building the linter from source (`go run golangci-lint@vX`) is not an option:
# the binary embeds the Go language version of its own module (1.26 for v2.13.2)
# and refuses to analyze a module targeting a newer Go (this repo is on 1.27), so
# a source build fails with "the Go language version used to build golangci-lint
# is lower than the targeted Go version".
#
# The version is pinned once, in mise.toml; the `lint` job in
# .github/workflows/go.yml reads the same pin, so CI and the local loop cannot
# drift apart. Resolution order: $GOLANGCI_LINT, a matching binary on PATH, mise,
# then the official release download cached under XDG_CACHE_HOME.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(sed -n 's/^golangci-lint = "\(.*\)"$/\1/p' "${REPO_ROOT}/mise.toml")"
if [ -z "$VERSION" ]; then
	echo "no golangci-lint pin found in ${REPO_ROOT}/mise.toml" >&2
	exit 1
fi
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/agent-orchestrator/golangci-lint/${VERSION}"

matches_pin() {
	local binary="$1"
	[ -x "$binary" ] || return 1
	"$binary" --version 2>/dev/null | grep -qF "version ${VERSION} "
}

on_path() {
	local binary
	binary="$(command -v golangci-lint || true)"
	[ -n "$binary" ] && matches_pin "$binary" && echo "$binary"
}

download_pinned() {
	if ! command -v curl >/dev/null 2>&1; then
		return 1
	fi
	mkdir -p "$CACHE_DIR"
	echo "Downloading golangci-lint v${VERSION} to ${CACHE_DIR}" >&2
	curl -sSfL "https://raw.githubusercontent.com/golangci/golangci-lint/v${VERSION}/install.sh" |
		sh -s -- -b "$CACHE_DIR" "v${VERSION}" >&2
	matches_pin "${CACHE_DIR}/golangci-lint"
}

if [ -n "${GOLANGCI_LINT:-}" ]; then
	exec "$GOLANGCI_LINT" "$@"
fi

if binary="$(on_path)" && [ -n "$binary" ]; then
	exec "$binary" "$@"
fi

# mise installs the same official release archive and is this repo's toolchain
# manager, so prefer it over a private download when it is available.
if command -v mise >/dev/null 2>&1; then
	exec mise exec "golangci-lint@${VERSION}" -- golangci-lint "$@"
fi

if matches_pin "${CACHE_DIR}/golangci-lint" || download_pinned; then
	exec "${CACHE_DIR}/golangci-lint" "$@"
fi

echo "Could not obtain golangci-lint v${VERSION}: no matching binary on PATH, no mise, and no curl." >&2
echo "Install mise (https://mise.jdx.dev) and run 'mise install', or set \$GOLANGCI_LINT to a v${VERSION} binary." >&2
exit 1
