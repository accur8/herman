{
  description = "Herman - a builder for nix based java app launchers";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        herman = pkgs.buildGoModule {
          pname = "herman";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          subPackages = [ "src" ];

          # Skip tests that require nix in the build environment
          doCheck = false;

          postInstall = ''
            mv $out/bin/src $out/bin/herman
          '';

          meta = with pkgs.lib; {
            description = "Herman - a builder for nix based java app launchers";
            homepage = "https://github.com/accur8/herman";
            license = licenses.mit;
            maintainers = [ ];
          };
        };
      in
      {
        packages = {
          default = herman;
          herman = herman;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gotools
            go-tools
            gopls
          ];

          shellHook = ''
            echo "Herman development environment"
            echo "Go version: $(go version)"
          '';
        };
      }
    );
}
