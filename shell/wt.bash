# wt shell integration for bash
# Source this file in your .bashrc:
#   source /path/to/wt.bash
# Or use: eval "$(wt shell-init bash)"

wt() {
  if [[ "$1" == "switch" ]] && [[ ! " $* " =~ " --tmux " ]]; then
    local result
    result=$(command wt switch --print-path "${@:2}")
    if [[ -n "$result" && -d "$result" ]]; then
      cd "$result"
    fi
  elif [[ "$1" == "add" ]] && [[ ! " $* " =~ " --tmux " ]]; then
    local result
    result=$(command wt add "${@:2}" --print-path)
    if [[ -n "$result" && -d "$result" ]]; then
      cd "$result"
    fi
  else
    command wt "$@"
  fi
}
