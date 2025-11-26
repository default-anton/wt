# wt shell integration for zsh
# Source this file in your .zshrc:
#   source /path/to/wt.zsh
# Or use: eval "$(wt shell-init zsh)"

wt() {
  if [[ "$1" == "cd" ]] && [[ ! " $* " =~ " --tmux " ]]; then
    local result
    result=$(command wt cd --print-path "${@:2}")
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
