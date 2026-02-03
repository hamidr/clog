{
  description = "Claude Code session logger with DuckDB-backed search";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
    in
    {
      overlays.default = final: _prev: {
        clog = self.packages.${final.system}.clog;
        clog-ollama = self.packages.${final.system}.clog-ollama;
      };

      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        rec {
          clog = pkgs.buildGoModule {
            pname = "clog";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-ss40DUh/u76GpLisyIutcgFJd0LIEhSCdu/0nUkmpgo=";
          };

          clog-ollama = pkgs.writeShellScriptBin "clog-ollama" ''
            export OLLAMA_HOST="''${OLLAMA_HOST:-http://localhost:11434}"
            export OLLAMA_EMBED_MODEL="''${OLLAMA_EMBED_MODEL:-nomic-embed-text}"
            exec ${clog}/bin/clog "$@"
          '';

          default = clog;
        }
      );
    };
}
