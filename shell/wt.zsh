# wt shell integration
# Add this to your .bashrc or .zshrc:
#   eval "$(wt shell-init bash)"  # for bash
#   eval "$(wt shell-init zsh)"   # for zsh

wt() {
  if [[ "$1" == "cd" ]] && [[ ! " $* " =~ " --tmux " ]] && [[ ! " $* " =~ " -t " ]]; then
    local result
    result=$(command wt cd --print-path "${@:2}")
    if [[ -n "$result" && -d "$result" ]]; then
      cd "$result"
    fi
  elif [[ "$1" == "add" ]] && [[ ! " $* " =~ " --tmux " ]] && [[ ! " $* " =~ " -t " ]]; then
    local result
    result=$(command wt add "${@:2}" --print-path)
    if [[ -n "$result" && -d "$result" ]]; then
      cd "$result"
    fi
  else
    command wt "$@"
  fi
}
