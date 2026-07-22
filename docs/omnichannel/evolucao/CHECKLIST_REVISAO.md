# Checklist independente de revisão

O revisor não corrige silenciosamente o pacote. Ele classifica cada item como `PASS`, `FAIL`,
`N/A` com evidência; `FAIL` volta ao executor/owner com arquivo, linha, contrato violado e teste que
deveria provar a correção.

## 1. Escopo

- [ ] diff contém somente a allowlist do pacote;
- [ ] mudanças prévias do usuário foram preservadas;
- [ ] nenhum workflow/WAHA/módulo fora do owner mudou;
- [ ] não houve commit, push, deploy, import ou ativação não autorizados;
- [ ] integração serializada (`module.go`, registry, migration, workflow) foi feita pelo pacote INT.

## 2. Banco

- [ ] migration é nova, número reservado, append-only, schema-qualified e sem Down incompatível;
- [ ] não duplica tabela/coluna/regra existente;
- [ ] `account_id`, FKs, CHECKs, timestamps e índices correspondem ao contrato;
- [ ] backfill é idempotente, tenant-safe e compatível com rolling deploy;
- [ ] migration em banco vazio e upgrade foram provados;
- [ ] query hot path tem paginação/índice e não faz N+1 óbvio.

## 3. Backend

- [ ] handler/service/store/adapter/worker permanecem separados;
- [ ] request limitado, erros estáveis e sem PII/segredo;
- [ ] toda query/mutação aplica tenant e recurso fora do escopo retorna 404;
- [ ] dedupe/idempotency e transação impedem side effect duplicado;
- [ ] realtime/n8n/outbox só ocorrem após commit;
- [ ] corrida IA×humano e resultado atrasado têm teste;
- [ ] retry diferencia temporário/permanente e possui dead-letter/observabilidade;
- [ ] capability/policy é validada no Go imediatamente antes do side effect.

## 4. n8n

- [ ] somente o JSON/ID próprio mudou;
- [ ] export faz parse e mantém ID/nome/contrato esperados;
- [ ] zero node de canal, SQL, segredo, PII, pin data ou estado definitivo;
- [ ] input/output versionados e validados por fixture;
- [ ] retry usa call/dispatch ID estável;
- [ ] workflow desligado/timeout/schema inválido cai no fallback especificado.

## 5. Frontend

- [ ] somente API Go é chamada;
- [ ] tipos não usam `any` e capability vem do backend;
- [ ] loading/empty/error/retry/pending/success estão cobertos;
- [ ] 409/404/401 e perda de realtime reconciliam estado autoritativo;
- [ ] ação perigosa tem confirmação, permissão e feedback;
- [ ] teclado/foco/labels e largura reduzida foram verificados;
- [ ] nenhum segredo/URL assinada fica em store, localStorage, log ou DOM duradouro.

## 6. Segurança e privacidade

- [ ] teste cross-tenant positivo e negativo;
- [ ] assinatura/replay/SSRF/path/MIME/tamanho aplicáveis foram testados;
- [ ] logs, traces, errors, fixtures e exports não contêm PII/segredo;
- [ ] output IA/tool/documento é tratado como não confiável;
- [ ] consentimento/opt-out/janela/rate limit são aplicados quando cabíveis;
- [ ] retenção/auditoria/custo recebem os novos dados.

## 7. Evidência e aceite

- [ ] todos os comandos da spec foram executados e os resultados estão no handoff;
- [ ] `git diff --check` passou;
- [ ] critérios felizes, duplicata/retry, falha/fallback e tenant foram provados;
- [ ] comportamento é observável em API/banco/UI, não só inferido do código;
- [ ] rollback local está descrito e não depende de apagar migration/dado;
- [ ] docs/AGENT/ESTADO foram atualizados somente pelo pacote de integração/fechamento.

## 8. Rejeição automática

Rejeitar sem “aprovar com ressalva” se houver:

- envio direto do n8n ao canal ou SQL no workflow;
- `account_id` confiado do body/modelo;
- segredo em texto puro/export/log;
- alteração em WAHA ou workflow de outro módulo;
- migration antiga editada;
- segundo estado/CRM/outbox para o mesmo fato;
- resposta IA após humano assumir;
- teste ausente para dedupe/cross-tenant no hot path;
- conclusão declarada sem executar a prova requerida.

