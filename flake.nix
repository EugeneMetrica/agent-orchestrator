{
  description = "agent-orchestrator development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        # The Go toolchain and golangci-lint are pinned by mise (mise.toml)
        # rather than nixpkgs, so the dev shell and CI both resolve the version
        # declared in go.mod instead of whatever go_1_xx nixpkgs happens to
        # carry. nix provides mise itself; the shell hook installs the pinned
        # tools and puts their shims on PATH so `go` is available in the shell.
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.mise
            pkgs.gotools
            pkgs.nodejs_22
            pkgs.pnpm_10
            pkgs.just
          ];

          shellHook = ''
            export GOPATH="$PWD/.go"
            export GOBIN="$GOPATH/bin"
            export PNPM_HOME="$PWD/.pnpm"
            export MISE_DATA_DIR="''${MISE_DATA_DIR:-$HOME/.local/share/mise}"
            export PATH="$GOBIN:$PNPM_HOME:$MISE_DATA_DIR/shims:$PATH"

            if ! mise install; then
              echo "flake.nix: 'mise install' failed; the toolchain pinned in mise.toml (Go, golangci-lint) is not available in this shell." >&2
            fi
          '';
        };
      }
    );
}
