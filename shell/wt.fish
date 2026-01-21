# wt shell integration
# Add this to your config.fish:
#   wt shell-init fish | source

function wt
  if test "$argv[1]" = "cd"; and not contains -- --tmux $argv; and not contains -- -t $argv
    set -l result (command wt cd --print-path $argv[2..])
    if test -n "$result"; and test -d "$result"
      cd $result
    end
  else if test "$argv[1]" = "add"; and not contains -- --tmux $argv; and not contains -- -t $argv
    set -l result (command wt add $argv[2..] --print-path)
    if test -n "$result"; and test -d "$result"
      cd $result
    end
  else
    command wt $argv
  end
end
