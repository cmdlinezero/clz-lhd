with import <nixpkgs> {};

let
  # External Script: This reads './dev.tmux' and puts it into a binary named 'dev-tmux'
  tmuxLayout   = pkgs.writeShellScriptBin "dev-tmux" (builtins.readFile ./dev.tmux);
in
pkgs.mkShell {

  name = "tmux-dev";
  nativeBuildInputs = with pkgs; [
    asciinema   # Cast videos
    asciinema-agg   # Cast videos to Gif
    mkcert      # Local development certificates
    go          # go runtime
    nodejs_22   # Node JavaScript
    tree        # Tree folder list
    tmux        # Tmux 
    tmuxLayout  # Tmux custom window layout
  ];

  LANGUAGE = "Tmux";
  VERSION  = "tmux -V";

  shellHook = ''
    # Optional: Script environment start up
    echo "Welcome to $LANGUAGE Development Environment"
    $VERSION

    # The Trap: This kills the tmux session as soon as you exit the nix-shell
    # It ensures no "ghost" sessions stay running in the background.
    trap "tmux kill-session -t go-dev" EXIT

    # Perform Tmux Dev Layout
    exec dev-tmux
  '';
}
