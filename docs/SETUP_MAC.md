# Ambiente no Mac Apple Silicon

## Stack completa local

Com Docker Desktop aberto e `.env` local configurado (usar `.env.docker.example`
como referência, mantendo as chaves do banco restaurado):

```bash
npm run dev:mac:up
npm run dev:mac:ps
npm run dev:mac:stop
```

`dev:mac:up` sobe a stack em segundo plano e mantém o Compose Watch ativo no
terminal para sincronizar automaticamente as alterações de `web/` ao salvar.
Deixe esse terminal aberto enquanto trabalha; Ctrl-C encerra apenas o watcher,
os containers continuam ligados. Para reativar o watcher da stack já ligada,
use `npm run dev:mac:watch`. `npm run dev:mac:start` também retoma os containers
e inicia o watcher automaticamente.

Para retomar a stack ja criada, sem rebuild nem recriacao:

```bash
npm run dev:mac:start
```

Esse comando preserva a configuracao e o codigo dos containers existentes. Use
`dev:mac:up` na primeira instalacao ou quando precisar reconstruir/reconciliar
imagens e configuracoes. `start` nao sincroniza alteracoes de codigo.

O acesso segue `localhost:3003 → local-access → web` e
`localhost:8080 → local-access → api → postgres`. Portanto, ligar somente
`web` e `api` no Docker Desktop nao publica as portas no Mac: `local-access`
tambem precisa estar ativo. Redis/n8n executam automacoes; WAHA/Evolution
atendem integracoes WhatsApp; Whisper faz transcricao. Esses auxiliares nao
substituem o proxy do painel. Estado `running` nao significa `healthy`:
aguarde os healthchecks e consulte `npm run dev:mac:ps`.

O comando sobe API, web, proxy local, PostgreSQL, Redis, n8n, WAHA, Whisper,
Evolution e seu banco, além do runner de compatibilidade Meta Ads. Usa os profiles
explicitamente; `docker compose up -d` sozinho não ativa serviços com profiles.
O runner Meta é legado de diagnóstico; o Assistente 360 usa a API Go.
WAHA usa emulação AMD64 no Apple Silicon.

A rede `omni_local_services` permite saída para WhatsApp, modelos e provedores.
As portas dos auxiliares são locais (`127.0.0.1`): n8n 5680, WAHA 3010,
Whisper 8010, Evolution 8085, Redis 6380 e runner 8765, salvo override no `.env`.
Painel: http://localhost:3003; API: http://localhost:8080/healthz.
Volumes persistem durante recriações; nunca apagar volumes para corrigir rede.

### Tela branca com 502 nos arquivos `/_nuxt/`

Incidente de 2026-11-03: metadados AppleDouble do macOS foram encontrados no
container como `._CardapioProductModal.vue`, `._CardapioSectionProdutos.vue`,
`._OmniTagsInput.vue` e `._OmniCollectionFilters.vue`, alem de seis metadados
de arquivos TypeScript (10 AppleDouble removidos). Sao arquivos binarios,
nao componentes. O scanner Vue do Tailwind/Oxide 4.3.0 abortou com
`Utf8Error` em `pre_processors/vue.rs:17:59`, interrompendo o servidor web;
o proxy local devolveu 502 para CSS e JavaScript e o painel ficou em branco.

Git, contexto Docker e Compose watch excluem `._*` e o stylesheet principal
tambem os exclui da deteccao Tailwind. Para sincronizacao manual com tar no Mac,
usar `COPYFILE_DISABLE=1 tar ...` ao criar o arquivo para nao gerar AppleDouble.
Um watch ja em execucao precisa ser reaberto para ler as novas exclusoes.
Se o container ja estiver contaminado, remover somente esses metadados binarios;
as exclusoes nao apagam arquivos que ja entraram. Nao apagar volumes, caches,
fontes reais nem alterar dependencias para resolver esse incidente.

`crow-nuxt` depende do repositório irmão `../crow-nuxt`, não incluído neste projeto.
Fica no profile `bio` do override Mac; só iniciá-lo explicitamente com esse
repositório disponível. A URL pública da bio pode apontar para o site publicado.
Containers temporários de build/check/release que terminaram com código zero
não são serviços e devem permanecer encerrados.

Container saudável confirma disponibilidade do serviço. WhatsApp ainda depende
do pareamento; IA, e-mail, Meta e workflows dependem das credenciais/configurações
do tenant e do ambiente. Não criar seeds ou ativar workflows para simular sucesso.

Verificação em 2026-11-01: 11 containers ativos; API, web, PostgreSQL, Redis,
n8n, WAHA, Whisper, Evolution, banco Evolution e runner Meta saudáveis; proxy
local respondendo. Painel e `/healthz` retornaram HTTP 200. As chamadas da API
para n8n, WAHA, Whisper e Evolution também retornaram 200.
Os oito workflows foram recuperados do runtime da VPS e importados no n8n local,
com seus IDs canônicos. As três credenciais referenciadas pelo WhatsApp foram
restauradas com autorização explícita, cifradas no banco local do n8n; Redis e
WAHA foram adaptados para os endereços locais. O community node
`n8n-nodes-waha@2024.11.5` também foi instalado no volume local.
Os seis workflows ativos na VPS foram reativados localmente. Os dois webhooks
restantes (Instagram First Contact e Queue Attendance Analysis), inativos na VPS,
foram habilitados somente no Mac para disponibilizar esses módulos.
Produção não foi alterada. Os JSONs versionados continuam com `active: false`,
sem pinData nem staticData de produção, conforme a política do projeto.

WAHA não possui sessões e Evolution não possui instâncias locais: conectar um
número de testes pelo painel antes de testar mensagens reais. Não copiar a sessão
do número de produção para o Mac. O runner legado Meta está saudável, mas sem
autenticação Claude; não é dependência do Assistente 360 atual.
Não houve alteração de dados de usuários nem de código do Finance.

Validação após restauração: nove endpoints POST registrados nos oito workflows;
Brain executando a validação de contrato sem chamada a provedor externo;
Whisper retornando HTTP 200 numa transcrição de áudio sintético de um segundo;
build de produção do web concluído. Não houve envio de mensagens nem teste completo
de cada fluxo de negócio. Temporários de credenciais em texto puro são removidos
após a importação; as credenciais persistem cifradas no volume do n8n.

Configurações de agentes: [guia compartilhado](agents/CONFIGURACAO.md).

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

Go e `golangci-lint` também não precisam ser instalados no Mac: quando há arquivos
Go staged, o pre-commit prepara e usa `omni-go-tools` pelo Docker. Mantenha o Docker
Desktop ativo para executar esse hook.

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
