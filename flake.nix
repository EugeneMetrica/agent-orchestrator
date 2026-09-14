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
        # The Go toolchain is pinned by mise (mise.toml) rather than nixpkgs, so
        # the dev shell and CI both resolve the version declared in go.mod
        # instead of whatever go_1_xx nixpkgs happens to carry.
        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.gotools
            pkgs.nodejs_22
            pkgs.pnpm_10
            pkgs.just
          ];

          shellHook = ''
            export GOPATH="$PWD/.go"
            export GOBIN="$GOPATH/bin"
            export PNPM_HOME="$PWD/.pnpm"
            export PATH="$GOBIN:$PNPM_HOME:$PATH"
          '';
        };
      }
    );
}
