# gm: project memory kept as note files in the repository, merged by git.
{ lib, buildGoModule, git, makeWrapper, asciidoctor, installShellFiles }:
buildGoModule (finalAttrs: {
  pname = "gm";
  version = "0.1.0";
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [ ./go.mod ./cmd ./internal ./skills ./docs ];
  };
  vendorHash = null; # standard library only
  env.CGO_ENABLED = "0";
  ldflags = [ "-s" "-w" "-X main.version=${finalAttrs.version}" ];
  nativeCheckInputs = [ git ]; # the specs run gm against real git repositories
  nativeBuildInputs = [ makeWrapper asciidoctor installShellFiles ];
  # The man pages and the skill ship with the binary, so all three always describe the same version.
  postInstall = ''
    wrapProgram $out/bin/gm --prefix PATH : ${lib.makeBinPath [ git ]}
    asciidoctor -b manpage -a revnumber=${finalAttrs.version} docs/gm.adoc -o gm.1
    asciidoctor -b manpage -a revnumber=${finalAttrs.version} docs/gm-note.adoc -o gm-note.5
    installManPage gm.1 gm-note.5
    mkdir -p $out/share/gm && cp -r skills $out/share/gm/
  '';
  meta = {
    description = "Project memory kept as note files in the repository, merged by git";
    mainProgram = "gm";
  };
})
