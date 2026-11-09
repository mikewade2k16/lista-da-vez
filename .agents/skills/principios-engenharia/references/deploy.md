# Deploy

Leia ao fazer deploy, alterar script/workflow de deploy, ou adicionar variável de ambiente. Os detalhes de infra abaixo são padrões comuns — adapte aos do seu projeto.

## Build fora do servidor de produção (registry)
Publicar as imagens num registry e o servidor só faz `pull` + `up -d --no-build` — nunca builda. Dois caminhos para a mesma esteira de imagens: (1) rápido — build na máquina local e push (dia a dia); (2) completo/rastreável — build no CI com gate de testes e tag por commit (release formal).

- **Por quê:** builds pesados (ex.: o build do Nuxt pede ~4GB de heap) sobrecarregam um servidor pequeno com produção no ar (OOM). `push`/`pull` é incremental (só camadas que mudaram) — resolve o "deploy rápido" sem buildar no servidor.
- Nunca reintroduzir `up -d --build` no servidor de produção como caminho padrão.

## Variável de ambiente nova precisa de DUAS adições
Adicionar tanto no arquivo de env do ambiente (`.env.production` no servidor) QUANTO na seção `environment` do compose. Sem a segunda, o container não recebe a variável (o compose não propaga o arquivo `.env` inteiro automaticamente). Toda variável que afeta network (`*_PORT`, `*_URL`, `*_BASE`) deve viver em arquivo versionado por um `.env.example`, nunca em shell export — qualquer rebuild que crie um shell novo perde a config.

## Reverse proxy: restart, não reload, após mudar upstream
Alguns proxies (ex.: Caddy) mantêm cache do upstream antigo no reload. Para garantir, fazer `restart` do container do proxy. Sintoma clássico de não fazer isso: o site continua mostrando a versão antiga após o deploy.

## Banco certo antes de migrar
Antes de rodar qualquer migrate local, verificar a porta/URL do banco (`echo $DATABASE_URL`). Caso real: duas instâncias de Postgres na mesma máquina em portas diferentes; o `.env` apontava para a errada e as migrations foram aplicadas no banco errado.


## Preparação e transporte do deploy (incidentes de 2026-11-04)

- Antes de copiar qualquer arquivo, criar o diretório pai no destino com `mkdir -p` e exigir sucesso. Isso vale para `docker cp` na validação local e para arquivos remotos. Não presumir que a imagem anterior contém as pastas dos fontes novos.
- Validar ferramentas do transporte selecionado antes do build; dependência ausente deve falhar antes de gastar minutos compilando. No transporte SSH de imagens, exigir cliente SFTP.
- Arquivos grandes precisam de retomada e tentativas limitadas. Neste projeto, usar SFTP `reput`, até três tentativas por execução, para parcial identificado por tag + SHA256. Não repetir build por uma falha de rede nem repetir automaticamente rollout, migrations ou importações.
- Criar e verificar permissão de escrita do diretório remoto antes da transferência; enviar para `.part`, validar SHA256 e somente então renomear e executar `docker load`. Nunca consumir arquivo incompleto ou ignorar erro de integridade.
- Preservar o arquivo local em caso de falha e informar como reusar `-ImageArchive` no `deploy-pull.ps1`, com mesma tag, ambiente e opções do deploy original. Falha de SHA256 exige investigar/retransferir o parcial exato; não repetir `reput` indefinidamente sobre conteúdo corrompido.
- Validar localmente sucesso, interrupção com retomada e esgotamento das tentativas sem tocar VPS. Isso verifica o controle de fluxo; não comprova estabilidade da rede nem garante sucesso de todo deploy. Acesso remoto e publicação continuam dependendo de autorização explícita.
