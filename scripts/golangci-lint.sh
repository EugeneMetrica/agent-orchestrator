#!/usr/bin/env bash
# Runs the pinned golangci-lint *release binary*, the same artifact CI uses.
#
# Building the linter from source (`go run golangci-lint@vX`) is not an option:
# the binary embeds the Go language version of its own module (1.26 for v2.13.2)
# and refuses to analyze a module targeting a newer Go (this repo is on 1.27),
# so a source build fails with "the Go language version used to build
# golangci-lint is lower than the targeted Go version".
#
# Resolution order: $GOLANGCI_LINT, a matching binary on PATH, a previously
# downloaded binary in the cache, then a download of the pinned release.
# Keep VERSION in sync with .github/workflows/go.yml.
set -euo pipefail

VERSION="2.13.2"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/agent-orchestrator/golangci-lint/${VERSION}"

matches_pin() {
	local binary="$1"
	[ -x "$binary" ] || return 1
	"$binary" --version 2>/dev/null | grep -qF "version ${VERSION} "
}

resolve() {
	if [ -n "${GOLANGCI_LINT:-}" ]; then
		echo "$GOLANGCI_LINT"
		return 0
	fi
	local on_path
	on_path="$(command -v golangci-lint || true)"
	if [ -n "$on_path" ] && matches_pin "$on_path"; then
		echo "$on_path"
		return 0
	fi
	if matches_pin "${CACHE_DIR}/golangci-lint"; then
		echo "${CACHE_DIR}/golangci-lint"
		return 0
	fi
	return 1
}

install_pinned() {
	if ! command -v curl >/dev/null 2>&1; then
		echo "golangci-lint v${VERSION} not found and curl is unavailable." >&2
		echo "Install the release binary manually (https://golangci-lint.run/docs/welcome/install/)" >&2
		echo "and either put it on PATH or point \$GOLANGCI_LINT at it." >&2
		return 1
	fi
	mkdir -p "$CACHE_DIR"
	echo "Downloading golangci-lint v${VERSION} to ${CACHE_DIR}" >&2
	curl -sSfL "https://raw.githubusercontent.com/golangci/golangci-lint/v${VERSION}/install.sh" |
		sh -s -- -b "$CACHE_DIR" "v${VERSION}" >&2
}

if ! binary="$(resolve)"; then
	install_pinned
	binary="${CACHE_DIR}/golangci-lint"
fi

exec "$binary" "$@"
