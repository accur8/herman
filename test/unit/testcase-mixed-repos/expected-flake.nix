{
  description = "testcase-mixed-repos - managed by Herman";

  inputs = {
    hermanRoot.url = "path:/Users/glen/.a8/herman";
    nixpkgs.follows = "hermanRoot/nixpkgs";
  };

  outputs = { self, nixpkgs, hermanRoot }:
    let

    resolvedJdk = if jdk != null then jdk else jdk25;
      system = "aarch64-darwin";
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      packages.${system}.default = pkgs.callPackage ./default.nix {};
    };
}
