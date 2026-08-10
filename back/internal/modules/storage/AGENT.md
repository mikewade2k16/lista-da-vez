# Modulo `storage`

## Escopo

Storage privado compartilhado da plataforma, inicialmente conectado ao Cloudflare R2 pela API
S3-compatible. O modulo centraliza todo acesso futuro de arquivos; modulos de produto nao recebem
credenciais R2 e nao escrevem diretamente no bucket.

## Contrato de custo

- Cloudflare R2 `Standard` apenas. `Infrequent Access` nao participa do free tier.
- Limites oficiais usados como teto de validacao (consulta de 2026-07-28): 10 GB-mes, 1 milhao de
  operacoes Classe A e 10 milhoes Classe B por mes.
- Defaults operacionais deixam 10% de folga: 9.000.000.000 bytes, 900.000 A e 9.000.000 B.
- Toda chamada ao provider reserva a operacao no PostgreSQL antes da chamada. Tentativa falha
  continua consumida no ledger, de modo conservador.
- O client S3 usa `aws.NopRetryer`: uma acao do service corresponde a no maximo uma tentativa R2.
- Arquivos menores que `MultipartThresholdBytes` usam `PutObject` unico. Arquivos grandes usam
  multipart S3 com partes fixas de `MultipartPartSizeBytes`; todas as partes, exceto a ultima,
  permanecem acima do minimo aceito pelo R2. Imagem, video, audio e documento possuem tetos
  independentes (default 25 MiB, maximo tecnico 512 MiB).
- Valores atuais: `MultipartThresholdBytes = 100 MiB` e `MultipartPartSizeBytes = 16 MiB`.
  Alterar qualquer um exige revisar custo Classe A, minimo de parte do R2, memoria do worker e testes.
- Antes de aceitar uma sessao multipart, o service reserva de uma vez `CreateMultipartUpload +
  quantidade de UploadPart + CompleteMultipartUpload`. Uma nova tentativa real de criar sessao,
  enviar parte ou concluir objeto exige reserva Classe A adicional antes da chamada ao provider.
- Parte confirmada possui ETag no PostgreSQL e nunca e reenviada. Repetir o mesmo numero de parte
  sem ETag e uma nova tentativa contabilizada; com ETag, a API devolve a confirmacao persistida sem
  chamar o R2 novamente.
- Reserva de bytes inclui objetos `pending` e `available`; concorrencia global usa advisory lock na
  mesma transacao. Ao atingir qualquer teto, o service falha fechado.
- Falha/timeout de `PutObject` e resultado ambiguo: o objeto permanece `pending` e os bytes ficam
  reservados ate uma reconciliacao futura provar ausencia ou remover o objeto. Nunca liberar no escuro.
- `connection-check` reconcilia por `HeadObject`: existente vira `available`; ausente so vira
  `failed` depois de transcorrer ao menos todo o `R2_UPLOAD_TIMEOUT`.
- O ciclo de faturamento e persistido em `storage.settings.billing_cycle_day` (1..28; inicialmente
  27 conforme o painel da conta). Consultas de operacoes usam esse inicio de ciclo, nunca o primeiro
  dia do mes por suposicao. Antes de cada novo upload, as metricas sao atualizadas sem cache.
- O cartao de armazenamento mostra `payloadSize`, igual ao total do painel Cloudflare; `metadataSize`
  aparece separado e e somado apenas ao total conservador usado para bloquear novos uploads.
- O teto instantaneo de 9 GB garante que a media dos picos diarios nao ultrapasse 9 GB-mes, desde
  que o bucket dedicado seja acessado exclusivamente pelo Omni.

## Integridade imutavel do original (regra inegociavel)

- Todo upload deve ser armazenado byte a byte como foi recebido. Nunca converter, transcodificar,
  recodificar, fazer remux, recomprimir, redimensionar ou alterar container, formato, extensao,
  codec, frame rate, resolucao, bitrate, canais de audio ou metadados do original.
- O original nunca pode ser sobrescrito por uma versao "otimizada". Nome, MIME e extensao devem
  descrever o arquivo recebido, sem fingir uma conversao que nao ocorreu.
- Thumbnail, poster, proxy, waveform ou qualquer derivado futuro deve ser um NOVO objeto, com
  identidade e custo proprios. O derivado nunca substitui o original em download, publicacao,
  exportacao ou integracao com Instagram, YouTube, TV etc.
- Validacao de MIME, limites e seguranca pode rejeitar o upload, mas nunca modificar seus bytes.
- Adaptadores de outros modulos nao podem transformar o arquivo antes de `Service.Upload`. Testes
  devem verificar que os bytes entregues ao provider sao exatamente os recebidos no boundary HTTP.

## Inicializacao segura

`POST /v1/storage/connection-check` e exclusivo de `platform_admin` e executa:

1. reserva uma operacao B e faz `HeadBucket`;
2. no primeiro vinculo, reserva uma operacao A e faz `ListObjectsV2(MaxKeys=1)`;
3. recusa bucket nao vazio por padrao; a excecao explicita de bootstrap apenas permite registrar
   um bucket dedicado ja populado, sem alterar seus objetos;
4. persiste account ID + bucket em `storage.provider_state`.

Depois do primeiro vinculo, trocar account/bucket por env falha com `ErrProviderMismatch`. O token
deve ter Object Read & Write limitado somente ao bucket dedicado. Console, Wrangler, scripts e
outros servicos nao podem reutilizar esse token/bucket: operacoes externas nao passam pelo ledger.

O free tier do R2 e compartilhado pela conta Cloudflare, nao isolado por bucket. Antes de cada
upload R2, o preflight combina as metricas account-wide da Cloudflare com as reservas locais ainda
nao refletidas pelo provider e falha fechado se Analytics estiver indisponivel ou algum teto puder
ser excedido. Os alertas de budget da Cloudflare sao apenas informativos: nao pausam nem limitam uso.

## API interna

`Service.Upload` e a porta sincrona para arquivos pequenos. `Service.StageMultipart` e a porta
duravel para arquivos grandes. Ambas exigem `accountId`, `sourceModule`, chave de idempotencia,
nome/MIME, tamanho e ator. Novas chaves R2 nascem sob
`accounts/{accountId}/{sourceModule}/{ano}/{id}--{arquivo}`. O R2 e plano; esses segmentos sao
prefixos de navegacao, nao diretorios reais. Chaves antigas continuam validas e nunca sao migradas
automaticamente. O storage possui allowlist base
para imagem, video, audio e documento; cada futuro adapter de dominio pode estreita-la. Nao ha URL
publica/presigned: escrita e leitura passam pelo service e reservam custo antes de chamar o R2.

Calendario e Tasks usam adapters hibridos na composition root: `uploads_enabled=false` delega a
gravacao aos storages locais originais e `true` usa R2 e seus limites por tipo. Os objetos R2 preservam os contratos
de URL `/uploads/calendar/...` e `/uploads/tasks/...`; GET passa pela API/R2 (incluindo Range de video)
e reserva Classe B. O file server global continua atendendo os caminhos legados em disco.

## Rotas administrativas

- `GET /v1/storage/status`: limites, ledger local e metricas account-wide reais da Cloudflare.
  Nunca apresentar o ledger local como se fosse consumo do provider.
- `GET /v1/storage/settings`: le os limites globais autoritativos.
- `PUT /v1/storage/settings`: altera os limites, sempre validando contra os tetos absolutos.
- `POST /v1/storage/connection-check`: valida conexao e inicializa o bucket dedicado.
- `POST /v1/storage/test-upload`: multipart `file`, exige conta autenticada, grava via
  `Service.Upload` com origem `storage_test` e `Idempotency-Key`.
- `GET /v1/storage/objects/{objectID}/content`: leitura privada tenant-scoped para validar/mostrar
  o objeto; reserva uma operacao Classe B antes do `GetObject`.

Todas exigem `platform_admin` e so existem com `CORE_V2_ENABLED=true`. A pagina
`/manage/storage` apresenta o consumo e usa o `OmniEntityDrawer` canonico com abas `Limites` e
`Teste de upload`; o estado global aparece sempre como switch binario `Upload para R2:
ativado|desativado` (nunca como botao de destino ambiguo). O teste aceita clique ou drag-and-drop,
mostra progresso e rele o objeto privado para preview.

## Persistencia

- `storage.provider_state`: vinculo imutavel do provider/bucket inicializado.
- `storage.monthly_usage`: ledger global mensal de A/B e bytes enviados.
- `storage.settings`: singleton autoritativo dos limites globais, por tipo, do seletor
  `uploads_enabled` e do dia inicial do ciclo de faturamento. Desligar novos uploads R2 nunca
  desliga a leitura remota existente.
- `storage.objects`: metadados tenant-scoped e estados `pending|available|failed|deleted`.
- `storage.multipart_uploads`: sessao, upload ID do provider, tamanho/quantidade de partes, estado
  e contadores de tentativas de criacao/conclusao.
- `storage.multipart_parts`: ETag, tamanho, numero e tentativas de cada parte confirmada.
- `storage.multipart_deliveries`: fila tenant-scoped do worker, path privado do staging, estado,
  tentativas, proxima execucao e ultimo erro sanitizado.

O binario nunca entra no PostgreSQL. `account_id` e repetido em todas as mutacoes de objeto.

## Env

- `R2_ENABLED` (default `false`)
- `R2_ACCOUNT_ID`, `R2_BUCKET`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`
- `R2_REQUEST_TIMEOUT` (default `30s`)
- `R2_UPLOAD_TIMEOUT` (default `15m`; timeout de `PutObject`, `UploadPart` e conclusao)
- `R2_ANALYTICS_API_TOKEN` (Bearer separado das credenciais S3, somente leitura)
- `R2_ALLOW_NONEMPTY_BUCKET_INITIALIZATION` (default `false`): excecao explicita de bootstrap
  para registrar no PostgreSQL um bucket dedicado ja populado. Nao lista, move, converte nem
  regrava objetos. Sem essa flag, a primeira conexao continua rejeitando bucket nao vazio.

Os limites nao sao env: ficam em `storage.settings` para mudanca auditada sem rebuild. Segredos
nunca entram em banco, resposta HTTP ou logs.

As operacoes e o armazenamento exibidos e usados no preflight vem da conta Cloudflare inteira
(local, producao e outros buckets). O ledger PostgreSQL e apenas a reserva conservadora imediata
entre atualizacoes externas. Sem Analytics confiavel, novos uploads R2 falham fechados; modo local
e leitura de objetos R2 existentes continuam disponiveis.

## Entrega duravel ao R2

### Fluxo completo do upload grande

1. O navegador envia o arquivo uma unica vez para a rota do modulo de origem, com progresso real
   navegador -> API. Calendario e Tasks continuam validando permissao, conta, MIME e teto ativo.
2. O handler nao materializa o video inteiro em memoria. O multipart HTTP usa arquivo temporario
   quando ultrapassa o buffer e entrega um `io.Reader` ao adapter hibrido.
3. Se o destino R2 estiver desligado, o adapter preserva exatamente o storage local legado. Se
   estiver ligado e o arquivo for grande, chama `Service.StageMultipart`.
4. `StageMultipart` copia exatamente `sizeBytes` para
   `UPLOADS_DIR/storage-staging/{id}.original`, com diretorio `0750` e arquivo `0600`. Tamanho
   diferente, stream incompleto ou MIME invalido elimina o staging e rejeita a operacao.
5. O service faz o preflight das metricas Cloudflare e dos limites locais, cria `storage.objects`
   como `pending`, reserva todas as operacoes planejadas e registra a sessao multipart. A chave de
   idempotencia impede criar dois objetos para o mesmo envio.
6. A fila e persistida em `storage.multipart_deliveries`. Somente depois de objeto, sessao e fila
   estarem autoritativos no PostgreSQL a API responde ao navegador. A partir desse ponto, retomar
   a entrega e responsabilidade exclusiva da plataforma.
7. O worker Go busca entregas `queued|retry` com `FOR UPDATE SKIP LOCKED`, marca `uploading` e cria
   a sessao no provider se ela ainda estiver em `creating`.
8. O worker abre o original do staging, posiciona o arquivo pelo offset de cada parte e envia
   somente os numeros que ainda nao possuem ETag em `storage.multipart_parts`. Cada buffer contem
   os bytes originais daquela faixa; nao existe conversao, remux ou transcodificacao.
9. Falha de rede, timeout ou resposta ambigua mantem staging, objeto e partes confirmadas. A fila
   vira `retry`, recebe `next_attempt_at` com backoff e o worker continua sozinho. Reinicio da API
   nao perde o estado porque PostgreSQL e volume persistente permanecem autoritativos.
10. Quando todas as partes e tamanhos conferem, o worker executa `CompleteMultipartUpload`, marca
    `storage.objects` como `available`, soma os bytes enviados e conclui a delivery.
11. O arquivo em staging so e removido depois de o PostgreSQL registrar a confirmacao integral do
    R2. Nunca apagar staging em timeout, erro ambiguo, parte incompleta ou antes do commit final.

### Leitura durante a entrega

- Um objeto `pending` com delivery ativa ja pode ser aberto pela mesma URL definitiva usada depois
  da conclusao. `ObjectMetadata` continua tenant-scoped e valida `sourceModule`.
- `GET` e `HEAD` leem o original do staging enquanto a delivery esta pendente. Range HTTP suporta
  faixa fechada, final aberto e suffix range, permitindo preview, seek e retomada de download sem
  esperar o R2.
- Depois de `available`, a mesma URL passa a ler o R2 e reserva Classe B normalmente. Nao ha troca
  de ID, nome ou URL quando o provider assume a leitura.
- Nunca exigir que o usuario selecione ou envie novamente um arquivo ja recebido integralmente
  pela API. O browser pode fechar depois da resposta; a fila continua no servidor.

### Fidelidade visual e binaria

- Preview local no navegador e leitura do staging sao apenas formas de exibir o mesmo original;
  nao geram uma nova versao do video.
- Preview, miniatura e viewer usam `object-fit: contain`, `object-position: center` e
  `aspect-ratio: auto`. A tela pode escalar proporcionalmente para caber no modal, mas nunca corta,
  estica ou altera o arquivo/resolucao original.
- Clicar em video abre o viewer modal compartilhado por Calendario e Tasks. Havendo varios
  arquivos, setas e teclado navegam pela lista. A reproducao nao abre fullscreen automaticamente.
- Poster/thumbnail futuro ou atual e objeto derivado separado. Ele nunca substitui o original em
  download, publicacao, exportacao ou integracao com Instagram, YouTube e TV.

### Reproducao Safari/iPhone e CDN

- iPhone/Safari e o cliente prioritario. Toda resposta de video R2 deve preservar ETag forte,
  `Content-Length`, `Accept-Ranges: bytes`, `Content-Range` e status `206` para faixas validas.
  `If-Range` diferente do ETag atual volta ao corpo completo; nunca servir uma faixa de outra versao.
- O player usa `playsinline`, `object-fit: contain`, proporcao automatica e nunca dispara fullscreen.
- As rotas privadas da API usam `Cache-Control: private, max-age=0, must-revalidate`. Cache CDN
  agressivo so pode existir em hostname dedicado com assinatura validada na borda antes do cache.
- Nunca ligar `r2.dev`. Um Custom Domain ligado diretamente ao bucket e publico; antes de ativa-lo,
  instalar Worker assinado (plano Free) ou WAF HMAC (Pro+) e provar que URL ausente/expirada falha.
- URLs de objetos sao imutaveis. O cache pode armazenar o original completo e responder Range a
  partir dele, mas nao pode comprimir, transcodificar, remuxar ou criar variantes do video.
