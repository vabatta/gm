{
  description = "gm: project memory kept as note files in the repository, merged by git";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-darwin" "x86_64-linux" "aarch64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAll (pkgs: { default = pkgs.callPackage ./package.nix { }; });

      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.gopls pkgs.git ];
          CGO_ENABLED = "0";
        };
      });

      checks = forAll (pkgs: {
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      });

      homeManagerModules.default = import ./nix/home-manager.nix self;
    };
}
