self:
{ config, lib, pkgs, ... }:
let
  cfg = config.programs.gm;
  # The skill comes from the package, so an override of the package also
  # overrides the skill, and both describe the same gm.
  skill = "${cfg.package}/share/gm/skills/memory";
in
{
  options.programs.gm = {
    enable = lib.mkEnableOption "gm, project memory kept as note files in the repository";
    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = lib.literalExpression "gm.packages.\${system}.default";
      description = "The gm package; its share/gm/skills holds the matching skill.";
    };
    claudeCode = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Link the memory skill into ~/.claude/skills for Claude Code.";
    };
    codex = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = ''
        Link the memory skill into ~/.agents/skills, where Codex reads personal
        skills. Off by default, because a configuration that links all of
        ~/.agents/skills as one directory cannot also hold this link.
      '';
    };
  };

  config = lib.mkIf cfg.enable (lib.mkMerge [
    { home.packages = [ cfg.package ]; }
    (lib.mkIf cfg.claudeCode { home.file.".claude/skills/memory".source = skill; })
    (lib.mkIf cfg.codex { home.file.".agents/skills/memory".source = skill; })
  ]);
}
