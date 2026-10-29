# Ambiente no Mac Apple Silicon

Setup local (2026-10-29), sem sudo e sem Homebrew:

```bash
ruby scripts/dev/setup-mac-tools.rb
```

O instalador baixa os arquivos oficiais, verifica SHA-256 e instala:

- Node 24.11.1 e npm, alinhados ao Dockerfile do projeto;
- PowerShell 7.6.9 ARM64, usado pelos scripts de deploy;
- prompt com cores, diretorio e branch Git para zsh e bash.

Executaveis em `~/.local/bin`; distribuicoes em `~/.local/share/node` e
`~/.local/share/powershell`. O shell padrao deste Mac e zsh; nao e necessario troca-lo.
Configuracoes existentes sao preservadas e copiadas para
`~/.local/state/omni-mac-setup/<data>/` antes de adicionar o carregador.
O prompt e o PATH ficam em `~/.config/crow-shell/`.

Abra um novo terminal ou, no zsh atual:

```bash
source ~/.zshrc
node --version
npm --version
pwsh --version
```

O prompt mostra a branch atual (exemplo: `main`) dentro de um repositorio Git.
Cores exigem um terminal interativo com suporte ANSI; execucoes automatizadas com
`TERM=dumb` usam o prompt simples.

O Node do host permite executar os atalhos npm e ferramentas de orquestracao.
O desenvolvimento do app continua Docker-first; nao e preciso instalar as dependencias
do frontend no host para usar os atalhos de deploy.

## Deploy

Os atalhos usam `pwsh` no Mac/Linux e `powershell.exe` no Windows.
Veja [DEPLOY_VPS.md](DEPLOY_VPS.md). Consultar ajuda nao faz build nem publica:

```bash
npm run deploy -- --help
```

Para publicar, o Docker Desktop precisa estar aberto, o GHCR autenticado com permissao
de publicar imagens e a chave/usuario SSH autorizados configurados. Se a chave padrao
nao existir, informe `-KeyPath`; o deploy rapido verifica isso antes de qualquer build.
As imagens usam `linux/amd64` por padrao para a arquitetura da VPS.
Um pedido de instalacao/configuracao do ambiente nao executa deploy nem build de validacao.

O launcher npm le `~/.config/omni/deploy.json` para configurar o acesso deste computador:

```json
{
  "sshUser": "USUARIO_AUTORIZADO",
  "sshKeyPath": "~/.ssh/omni_vps_mac"
}
```

O arquivo e local e a chave privada permanece em `~/.ssh/`. Nao colocar chaves no
repositorio. Precedencia: argumentos `-User`/`-KeyPath`, variaveis `OMNI_DEPLOY_USER`/
`OMNI_DEPLOY_KEY_PATH`, configuracao local, defaults dos scripts.
Assim `npm run deploy` funciona sem repetir o caminho da chave em cada comando,
inclusive nos scripts aninhados de staging/promocao. Ajuda nao le nem usa credenciais.
