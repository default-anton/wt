# wt shell integration for fish
# Source this file in your config.fish:
#   source /path/to/wt.fish
# Or use: wt shell-init fish | source

function wt
  if test "$argv[1]" = "switch"; and not contains -- --tmux $argv
    set -l result (command wt switch --print-path $argv[2..])
    if test -n "$result"; and test -d "$result"
      cd $result
    end
  else if test "$argv[1]" = "add"; and not contains -- --tmux $argv
    set -l result (command wt add $argv[2..] --print-path)
    if test -n "$result"; and test -d "$result"
      cd $result
    end
  else
    command wt $argv
  end
end
