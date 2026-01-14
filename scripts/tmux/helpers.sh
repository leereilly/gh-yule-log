#!/usr/bin/env bash
#
# Tmux helper functions for yule-log plugin
#

# Get a tmux option value, with a default fallback
get_tmux_option() {
    local option="$1"
    local default_value="${2:-}"
    local option_value

    option_value=$(tmux show-option -gqv "$option" 2>/dev/null)

    if [ -z "$option_value" ]; then
        echo "$default_value"
    else
        echo "$option_value"
    fi
}

# Set a tmux option
set_tmux_option() {
    local option="$1"
    local value="$2"
    tmux set-option -gq "$option" "$value"
}

# Get the current tmux server PID
current_tmux_server_pid() {
    tmux display-message -p "#{pid}"
}

# Check if running inside tmux
is_inside_tmux() {
    [ -n "${TMUX:-}" ]
}
