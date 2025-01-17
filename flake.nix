{
  description = "Dependencies of app-autoscaler-release";

  inputs = {
    nixpkgs.url = github:NixOS/nixpkgs/nixos-unstable;
    nixpkgs-bosh-cli-v7-3-1.url = github:NixOS/nixpkgs/1179c6c3705509ba25bd35196fca507d2a227bd0;
  };

  outputs = { self, nixpkgs, nixpkgs-bosh-cli-v7-3-1 }:
    let
      supportedSystems = [ "x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin" ];

      # Helper function to generate an attrset '{ x86_64-linux = f "x86_64-linux"; ... }'.
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # Nixpkgs instantiated for supported system types.
      nixpkgsFor = forAllSystems (system: import nixpkgs { inherit system; });
      nixpkgsFor-bosh-cli-v7-3-1 = forAllSystems (system: import nixpkgs-bosh-cli-v7-3-1 { inherit system; });
    in {
      packages = forAllSystems (system:{
        app-autoscaler-cli-plugin = nixpkgsFor.${system}.buildGoModule rec {
          pname = "app-autoscaler-cli-plugin";
          gitCommit = "f46dc1ea62c4c7bd426c82f4e2a525b3a3c42300";
          version = "${gitCommit}";
          src = nixpkgsFor.${system}.fetchgit {
            url = "https://github.com/cloudfoundry/app-autoscaler-cli-plugin";
            rev = "${gitCommit}";
            hash = "sha256-j8IAUhjYjEFvtRbA6o2vA7P2uUmKVYsd9uJmN0WtVCM=";
            fetchSubmodules = true;
          };
          doCheck = false;
          vendorHash = "sha256-NzEStcOv8ZQsHOA8abLABKy+ZE3/SiYbRD/ZVxo0CEk=";
        };
      });

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgsFor.${system};
          pkgs-bosh-cli-v7-3-1 = nixpkgsFor-bosh-cli-v7-3-1.${system};
        in {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              act
              actionlint
              self.packages.${system}.app-autoscaler-cli-plugin
              # to make `bosh create-release` work in a Nix shell on macOS, use an older bosh-cli version that reuses
              # a bosh-utils version under the hood that doesn't use the tar option `--no-mac-metadata`.
              # unfortunately, Nix provides gnutar by default, which doesn't have the `--no-mac-metadata` option.
              # bosh-utils assumes blindly bsdtar when building on macOS which comes with the `--no-mac-metadata` option,
              # see bosh-utils change https://github.com/cloudfoundry/bosh-utils/commit/f79167bd43f3afc154065edc95799a464a80605f.
              # this blind bsdtar assumption by bosh-utils breaks creating bosh releases in a Nix shell on macOS.
              # a GitHub issue related to this problem can be found here: https://github.com/cloudfoundry/bosh-utils/issues/86.
              pkgs-bosh-cli-v7-3-1.bosh-cli
              cloudfoundry-cli
              credhub-cli
              delve # go-debugger
              fly
              ginkgo
              gh
              gnumake
              go
              go-tools
              golangci-lint
              gopls # See: <https://github.com/golang/vscode-go/blob/master/docs/tools.md>
              google-cloud-sdk
              maven
              nodejs
              nodePackages.yaml-language-server
              # # We use the binary-buildpack and nix-build-results have set the wrong ELF-Interpreter.
              # # For more background, see:
              # # <https://blog.thalheim.io/2022/12/31/nix-ld-a-clean-solution-for-issues-with-pre-compiled-executables-on-nixos>
              # patchelf
              ruby
              rubocop
              rubyPackages.solargraph
              # The following package for cf-uaac is needed by our makefile as well.
              # Until PR https://github.com/NixOS/nixpkgs/pull/189079 has been merged, this requires
              # as additional input: `jdwpkgs.url = github:joergdw/nixpkgs/rubyPackages.cf-uaac;`
              # Alternative solution 1: `gem install …` using https://direnv.net/man/direnv-stdlib.1.html#codelayout-rubycode
              # to create a project-specific ruby-gem-path – This solution is currently applied!
              #
              # Alternative solution 2: produce a package here locally that contains cf-uaac
              # by making use of <https://nixos.org/manual/nixpkgs/stable/#developing-with-ruby>, see
              # chapter: 17.30.2.5. Packaging applications
              #
              # jdwpkgs.rubyPackages.cf-uaac
              shellcheck
              swagger-cli
              temurin-bin
              yq-go
            ];

            # Needed to run with delve, see: <https://nixos.wiki/wiki/Go#Using_cgo_on_NixOS>
            hardeningDisable = [ "fortify" ];

            shellHook = ''
              # install required CF CLI plugins
              cf install-plugin "$(whereis -q app-autoscaler-cli-plugin)" -f

              aes_terminal_font_yellow='\e[38;2;255;255;0m'
              aes_terminal_font_blink='\e[5m'
              aes_terminal_reset='\e[0m'

              echo -ne "$aes_terminal_font_yellow" "$aes_terminal_font_blink"
              cat << 'EOF'
                ⚠️ If `whoami` does not work properly on your computer, `bosh ssh` commands may fail.
                The solution is, to provide your nix dev-shell the path to the `libnss_sss.so.2` of
                your host system, see: <https://github.com/NixOS/nixpkgs/issues/230110>

                Adapt the following line to contain the correct path:
                export LD_PRELOAD='/lib/x86_64-linux-gnu/libnss_sss.so.2'
              EOF
              echo -ne "$aes_terminal_reset"
            '';
          };
      });
  };
}
