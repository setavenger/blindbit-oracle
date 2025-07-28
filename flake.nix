{
  description = "Blindbit Oracle - Silent Payments indexing server";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "aarch64-darwin" "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.buildGoModule {
            pname = "blindbit-oracle";
            version = "0.1.0";
            src = ./.;
            subPackages = [ "src" ];
            vendorHash = "sha256-Y2HtkegrGiDdgLDmgFY4gQ27JONT1oNv3Mgpu9gzB6s=";
            
            nativeBuildInputs = [ pkgs.just ];
            
            # Use justfile for building
            buildPhase = ''
              runHook preBuild
              just build
              runHook postBuild
            '';
            
            installPhase = ''
              runHook preInstall
              mkdir -p $out/bin
              cp blindbit-oracle $out/bin/
              runHook postInstall
            '';
          };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ 
            go_1_24
            just
            golangci-lint
            delve
            git
          ];
          
          # Set environment variables inside mkShell
          GOFLAGS = "-buildvcs=false";
          
          shellHook = ''
            echo "Blindbit Oracle development environment"
            echo "Go version: $(go version)"
            echo ""
            echo "Available commands:"
            echo "  just help    - Show available tasks"
            echo "  just build   - Build the application"
            echo "  just run     - Run with development config"
            echo "  just test    - Run tests"
            echo ""
            echo ""
            echo "To get started use:"
            echo "  just init    - Initialize ~/.blindbit-oracle"
            echo ""
          '';
        };
      });
}