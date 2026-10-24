#!/usr/bin/env bash

set -euo pipefail

DEFAULT_PORT=8088

PORT="${1:-$DEFAULT_PORT}"

# Docker-style random session names
# ADJECTIVES=(
#     admiring bold clever eager festive focused
#     gifted happy kind lucid nifty quirky
#     relaxed serene stoic vibrant wonderful zealous
# )
#
# NAMES=(
#     babbage berners_lee dijkstra hopper knuth
#     lovelace mccarthy ritchie shannon stallman
#     thompson torvalds turing von_neumann wozniak
# )

random_item() {
    local -n array=$1
    printf '%s' "${array[RANDOM % ${#array[@]}]}"
}


generate_session_name() {
  local adjectives=(
    happy brave calm eager fancy gentle jolly kind
    lucky nice proud quick shiny witty bold clever
  )

  local nouns=(
    tiger panda eagle otter fox whale lion falcon
    wolf bear koala shark
  )

  local adj="${adjectives[RANDOM % ${#adjectives[@]}]}"
  local noun="${nouns[RANDOM % ${#nouns[@]}]}"

  echo "${adj}-${noun}-dev"
}

if [[ -n "${2:-}" ]]; then
    SESSION="$2"
else
    SESSION="$(generate_session_name)"

    # Avoid collisions with existing sessions.
    while tmux has-session -t "$SESSION" 2>/dev/null; do
        SESSION="$(generate_session_name)"
    done
fi

echo "Using PORT=$PORT"
echo "Using SESSION=$SESSION"


# -----------------------------------------------------------------------------
# Attach to an existing session
# -----------------------------------------------------------------------------

# Check if the session already exists
if ! tmux has-session -t "$SESSION" 2>/dev/null; then

    # -----------------------------------------------------------------------------
    # Create session
    # -----------------------------------------------------------------------------
    tmux new-session -d -s "$SESSION" "zsh -i"

    tmux split-window \
        -h \
        -t "$SESSION" \
        "hugo server -D -p $PORT --disableFastRender --cleanDestinationDir; exec zsh -i"

    tmux split-window \
        -v \
        -t "$SESSION:0.0" \
        "zsh -i"

    tmux select-layout -t "$SESSION:0" main-vertical
    tmux select-pane -t "$SESSION:0.0"

    tmux send-keys \
        -t "$SESSION:0.0" \
        "hugo version; tree -L 2" \
        C-m
fi

# -----------------------------------------------------------------------------
# Attach
# -----------------------------------------------------------------------------
exec tmux attach-session -t "$SESSION"
