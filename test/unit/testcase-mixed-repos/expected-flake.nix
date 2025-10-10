{
  description = "mixed-app_2.13 - managed by Herman";

  inputs = {
    hermanRoot.url = "path:/Users/glen/.a8/herman";
    nixpkgs.follows = "hermanRoot/nixpkgs";
  };

  outputs = { self, nixpkgs, hermanRoot }:
    let
      system = "aarch64-darwin";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      packages.${system}.default = pkgs.callPackage ./default.nix {};
    };
}
