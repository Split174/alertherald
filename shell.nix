{pkgs ? import <nixpkgs> {}}:
pkgs.mkShell {
  buildInputs = [
    pkgs.go
    pkgs.gopls
    pkgs.gotools
    pkgs.curl
  ];

  shellHook = ''
    export PATH=$PATH:$(go env GOPATH)/bin
    go install github.com/xhd2015/xgo/cmd/xgo@latest
    xgo version
  '';
}
