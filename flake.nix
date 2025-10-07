{
  description = "Herman - A launcher for Java applications";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "herman";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          subPackages = [ "src" ];

          postInstall = ''
            mv $out/bin/src $out/bin/herman
          '';

          meta = with pkgs.lib; {
            description = "A launcher for Java applications";
            homepage = "https://github.com/accur8/herman";
            license = licenses.mit;
            maintainers = [ ];
          };
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
