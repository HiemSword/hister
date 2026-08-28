{
  description = "Hister - Web history on steroids";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-parts.inputs.nixpkgs-lib.follows = "nixpkgs";
  };

  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "x86_64-darwin"
        "aarch64-linux"
        "aarch64-darwin"
      ];

      perSystem =
        {
          config,
          self',
          inputs',
          pkgs,
          system,
          ...
        }:
        let
          histerPackage = pkgs.callPackage ./nix/package.nix { histerRev = inputs.self.rev or "unknown"; };
        in
        {
          packages.default = histerPackage;
          packages.hister = histerPackage;

          devShells.default = pkgs.mkShell {
            packages = builtins.attrValues { inherit (pkgs) go gopls gotools; };
          };
        };

      flake = {
        nixosModules.default = inputs.self.nixosModules.hister;
        nixosModules.hister =
          { lib, pkgs, ... }:
          {
            services.hister.package = (
              lib.mkDefault inputs.self.packages.${pkgs.stdenvNoCC.hostPlatform.system}.default
            );
          };

        homeModules.default = inputs.self.homeModules.hister;
        homeModules.hister =
          { lib, pkgs, ... }:
          {
            imports = [ ./nix/home.nix ];
            services.hister.package = (
              lib.mkDefault inputs.self.packages.${pkgs.stdenvNoCC.hostPlatform.system}.default
            );
          };

        darwinModules.default = inputs.self.darwinModules.hister;
        darwinModules.hister =
          { lib, pkgs, ... }:
          {
            imports = [ ./nix/darwin.nix ];
            services.hister.package = (
              lib.mkDefault inputs.self.packages.${pkgs.stdenvNoCC.hostPlatform.system}.default
            );
          };
      };
    };
}
