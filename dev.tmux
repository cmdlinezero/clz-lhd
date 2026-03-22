#!/usr/bin/env bash

DEFAULT_PORT=8080

# Generate random Docker-like name
generate_session_name() {
  ADJECTIVES=(happy brave calm eager fancy gentle jolly kind lucky nice proud quick shiny witty bold clever)
  NOUNS=(tiger panda eagle otter fox whale lion falcon wolf bear koala shark)

  adj=${ADJECTIVES[$RANDOM % ${#ADJECTIVES[@]}]}
  noun=${NOUNS[$RANDOM % ${#NOUNS[@]}]}
  echo "${adj}-${noun}-dev"
}

# Check if port is in use
port_in_use() {
  lsof -i :"$1" >/dev/null 2>&1
}

# Find next available port
find_free_port() {
  local port=$1
  while port_in_use "$port"; do
    echo "Port $port is in use, trying $((port + 1))..."
    port=$((port + 1))
  done
  echo "$port"
}

# Args
REQUESTED_PORT=${1:-$DEFAULT_PORT}
SESSION=${2:-$(generate_session_name)}

# Resolve port
PORT=$(find_free_port "$REQUESTED_PORT")

echo "Using PORT=$PORT"
echo "Using SESSION=$SESSION"

# Check if the session already exists
tmux has-session -t "$SESSION" 2>/dev/null


if [ $? != 0 ]; then
  tmux new-session -d -s "$SESSION"
  tmux split-window -h -t "$SESSION"
  tmux split-window -v -t "$SESSION"

  tmux send-keys -t "$SESSION":0 
fi

# Hugo Example retry
# if [ $? != 0 ]; then
#   tmux new-session -d -s "$SESSION"
#   tmux split-window -h -t "$SESSION"
#   tmux split-window -v -t "$SESSION"
# 
#   tmux send-keys -t "$SESSION":0 
#   tmux send-keys -t "$SESSION":0 \
#   "until hugo serve -D -p $PORT; do echo 'Command failed. Press Enter to retry...'; read; done" C-m
# fi

tmux attach-session -t "$SESSION"
