# Installs gm and, for Claude Code, links the memory skill.
self:
{ config, lib, pkgs, ... }:
let
  cfg = config.programs.gm;
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
  };

  config = lib.mkIf cfg.enable (lib.mkMerge [
    { home.packages = [ cfg.package ]; }
    (lib.mkIf cfg.claudeCode {
      home.file.".claude/skills/memory".source = "${cfg.package}/share/gm/skills/memory";
    })
  ]);
}
