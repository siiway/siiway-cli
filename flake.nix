{
  description = "Nix flake for siiway-cli";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    systems.url = "github:nix-systems/default";
  };

  outputs = inputs@{ self, flake-parts, systems, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = import systems;

      perSystem = { pkgs, system, ... }:
        let
          goToolchain = if pkgs ? go_1_26 then pkgs.go_1_26 else pkgs.go;

          package = pkgs.buildGoModule rec {
            pname = "siiway-cli";
            version =
              if self ? shortRev then self.shortRev
              else if self ? dirtyShortRev then self.dirtyShortRev
              else "dev";

            src = self;
            vendorHash = "sha256-f247K347OFddpKF6nME/AqoDqwBLOJGfW/IpNRU6gds=";

            nativeBuildInputs = [ pkgs.makeWrapper ];
            buildInputs = [ pkgs.git ];

            ldflags = [
              "-s"
              "-w"
              "-X github.com/SiiWay/siiway-cli/cmd.Version=${version}"
            ];

            postFixup = ''
              wrapProgram $out/bin/siiway-cli \
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git ]}
            '';
          };
        in
        {
          packages.default = package;
          packages.siiway-cli = package;

          apps.default = {
            type = "app";
            program = "${package}/bin/siiway-cli";
          };

          devShells.default = pkgs.mkShell {
            packages = [
              goToolchain
              pkgs.gopls
              pkgs.git
            ];
          };

          formatter = pkgs.nixpkgs-fmt;
        };
    };
}
