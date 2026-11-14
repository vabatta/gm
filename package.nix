# gm: project memory kept as note files in the repository, merged by git.
{ lib, buildGoModule, git, makeWrapper }:
buildGoModule {
  pname = "gm";
  version = "0.1.0";
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [ ./go.mod ./cmd ./internal ./skills ];
  };
  vendorHash = null; # standard library only
  env.CGO_ENABLED = "0";
  ldflags = [ "-s" "-w" ];
  nativeCheckInputs = [ git ]; # the tests make real git repositories
  nativeBuildInputs = [ makeWrapper ];
  # The skill ships with the binary so both always come from the same version.
  postInstall = ''
    wrapProgram $out/bin/gm --prefix PATH : ${lib.makeBinPath [ git ]}
    mkdir -p $out/share/gm && cp -r skills $out/share/gm/
  '';
  meta = {
    description = "Project memory kept as note files in the repository, merged by git";
    mainProgram = "gm";
  };
}
