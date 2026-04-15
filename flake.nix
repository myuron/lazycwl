{
  description = "flake for Go with Agent-skills";

  inputs = {
    nixpkgs = {
      url = "https://flakehub.com/f/NixOS/nixpkgs/0.1";
    };
    flake-utils = {
      url = "github:numtide/flake-utils";
    };
    agent-skills = {
      url = "github:Kyure-A/agent-skills-nix";
    };
    anthropic-skills = {
      url = "github:anthropics/skills";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, flake-utils, agent-skills, anthropic-skills, ... }:
  {
    overlays.default = final: prev: {
      lazycwl = final.buildGoModule {
        pname = "lazycwl";
        version = "0.1.0";
        src = self;
        vendorHash = "sha256-71IHdtlB5cjOiYrrr5SJ8d/61ZSuWXGvtra3q1ULFCE=";
      };
    };
  }
  //
  flake-utils.lib.eachDefaultSystem (
    system:
    let
      pkgs = import nixpkgs {
        inherit system;
        overlays = [ self.overlays.default ];
      };
      agentLib = agent-skills.lib.agent-skills;
      sources = {
        anthropic = {
          path = anthropic-skills;
          subdir = "skills";
        };
      };
      catalog = agentLib.discoverCatalog sources;
      allowlist = agentLib.allowlistFor {
        inherit catalog sources;
        # Add Agent Skills
        enable = [
          "doc-coauthoring"
          "skill-creator"
        ];
      };
      selection = agentLib.selectSkills {
        inherit catalog allowlist sources;
        skills = { };
      };
      bundle = agentLib.mkBundle { inherit pkgs selection; };
      localTargets = {
        claude = agentLib.defaultLocalTargets.claude // { enable = true; };
      };
    in
    {
      packages.default = pkgs.lazycwl;

      apps = {
        default = {
          type = "app";
          program = "${pkgs.lazycwl}/bin/lazycwl";
        };
        skills-install-local = {
          type = "app";
          program = "${agentLib.mkLocalInstallScript {inherit pkgs bundle; targets = localTargets; }}/bin/skills-install-local";
        };
      };
      devShells.default = pkgs.mkShell {
        packages = with pkgs; [
          go
          awscli2
        ];
      };
    }
  );
}
