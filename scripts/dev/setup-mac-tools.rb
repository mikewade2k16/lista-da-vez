require 'fileutils'
require 'digest'
require 'time'

abort 'Este instalador requer macOS Apple Silicon (arm64).' unless RUBY_PLATFORM.include?('darwin') && IO.popen(['uname', '-m'], &:read).strip == 'arm64'

home_dir = Dir.home
local_bin = File.join(home_dir, '.local/bin')
backup_dir = File.join(home_dir, '.local/state/omni-mac-setup', Time.now.utc.strftime('%Y%m%dT%H%M%SZ'))
shell_dir = File.join(home_dir, '.config/crow-shell')
FileUtils.mkdir_p([local_bin, backup_dir, shell_dir])
%w[env.sh prompt.zsh prompt.bash].each do |name|
  path = File.join(shell_dir, name)
  FileUtils.cp(path, File.join(backup_dir, name)) if File.exist?(path)
end

def install_archive(archive, checksum, destination, strip: false)
  abort "Checksum invalido: #{archive}" unless Digest::SHA256.file(archive).hexdigest == checksum
  FileUtils.mkdir_p(destination)
  args = ['tar', '-xzf', archive, '-C', destination]
  args += ['--strip-components', '1'] if strip
  abort 'Falha na extracao' unless system(*args)
end

downloads = {
  '/private/tmp/omni-node-darwin-arm64.tar.gz' => 'https://nodejs.org/dist/v24.11.1/node-v24.11.1-darwin-arm64.tar.gz',
  '/private/tmp/omni-powershell-osx-arm64.tar.gz' => 'https://github.com/PowerShell/PowerShell/releases/download/v7.6.9/powershell-7.6.9-osx-arm64.tar.gz'
}
downloads.each do |path, url|
  next if File.exist?(path)
  abort 'Falha no download oficial' unless system('curl', '-fL', '--connect-timeout', '15', '--max-time', '120', url, '-o', path)
end

install_archive('/private/tmp/omni-node-darwin-arm64.tar.gz',
  'b05aa3a66efe680023f930bd5af3fdbbd542794da5644ca2ad711d68cbd4dc35',
  File.join(home_dir, '.local/share/node/v24.11.1'), strip: true)
install_archive('/private/tmp/omni-powershell-osx-arm64.tar.gz',
  'a1c7e98ac403da4be5ba75c1c5855af272de7787a91d478c4267466fe58fc1af',
  File.join(home_dir, '.local/share/powershell/7.6.9'))
File.chmod(0755, File.join(home_dir, '.local/share/powershell/7.6.9/pwsh'))

%w[node npm npx].each do |name|
  target = File.join(local_bin, name)
  source = File.join(home_dir, '.local/share/node/v24.11.1/bin', name)
  next if File.symlink?(target) && File.readlink(target) == source
  abort "Ja existe: #{target}" if File.exist?(target) || File.symlink?(target)
  File.symlink(source, target)
end
pwsh_target = File.join(local_bin, 'pwsh')
pwsh_source = File.join(home_dir, '.local/share/powershell/7.6.9/pwsh')
unless File.symlink?(pwsh_target) && File.readlink(pwsh_target) == pwsh_source
  abort "Ja existe: #{pwsh_target}" if File.exist?(pwsh_target) || File.symlink?(pwsh_target)
  File.symlink(pwsh_source, pwsh_target)
end

File.write(File.join(shell_dir, 'env.sh'), <<~'SH')
  # Ferramentas locais; nao requer sudo nem altera instalacoes do sistema.
  for crow_path in "$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/bin/fallback" "$HOME/Developer/.tools/gh" "$HOME/.local/bin"; do
    if [ -d "$crow_path" ]; then
      case ":$PATH:" in *":$crow_path:"*) ;; *) PATH="$crow_path:$PATH" ;; esac
    fi
  done
  export PATH
  unset crow_path
SH

File.write(File.join(shell_dir, 'prompt.zsh'), <<~'SH')
  autoload -Uz colors vcs_info add-zsh-hook
  colors
  zstyle ':vcs_info:*' enable git
  zstyle ':vcs_info:git:*' formats '(%b)'
  zstyle ':vcs_info:git:*' actionformats '(%b|%a)'
  add-zsh-hook precmd vcs_info
  setopt prompt_subst
  if [[ "$TERM" != dumb ]]; then
    PROMPT='%F{cyan}%~%f %F{magenta}${vcs_info_msg_0_}%f
  %F{green}❯%f '
    export CLICOLOR=1
    alias ls='ls -G'
  else
    PROMPT='%~ ${vcs_info_msg_0_} > '
  fi
SH

File.write(File.join(shell_dir, 'prompt.bash'), <<~'SH')
  __crow_git_branch() {
    local branch
    branch="$(git symbolic-ref --quiet --short HEAD 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)" || return 0
    [ -z "$branch" ] || printf '(%s)' "$branch"
  }
  if [ "$TERM" != dumb ]; then
    PS1='\[\033[36m\]\w\[\033[0m\] \[\033[35m\]$(__crow_git_branch)\[\033[0m\]\n\[\033[32m\]>\[\033[0m\] '
    export CLICOLOR=1
    alias ls='ls -G'
  else
    PS1='\w $(__crow_git_branch) > '
  fi
SH

{
  '.zshrc' => '[ -r "$HOME/.config/crow-shell/env.sh" ] && . "$HOME/.config/crow-shell/env.sh"\n[[ -o interactive ]] && source "$HOME/.config/crow-shell/prompt.zsh"',
  '.zprofile' => '[ -r "$HOME/.config/crow-shell/env.sh" ] && . "$HOME/.config/crow-shell/env.sh"',
  '.bashrc' => '[ -r "$HOME/.config/crow-shell/env.sh" ] && . "$HOME/.config/crow-shell/env.sh"\ncase "$-" in *i*) . "$HOME/.config/crow-shell/prompt.bash" ;; esac',
  '.bash_profile' => '[ -r "$HOME/.bashrc" ] && . "$HOME/.bashrc"'
}.each do |name, loader|
  path = File.join(home_dir, name)
  previous = File.exist?(path) ? File.read(path) : ''
  FileUtils.cp(path, File.join(backup_dir, name)) if File.exist?(path)
  next if previous.include?('# BEGIN crow-shell setup')
  File.write(path, previous + "\n# BEGIN crow-shell setup\n" + loader.gsub('\\n', "\n") + "\n# END crow-shell setup\n")
end
puts "Ferramentas e prompt instalados. Backup: #{backup_dir}"
