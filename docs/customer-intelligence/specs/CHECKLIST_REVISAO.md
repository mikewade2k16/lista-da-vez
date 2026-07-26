# Checklist de revisão independente — Customer Intelligence

- **Status:** DRAFT — revisão independente/E2E ainda não concluídos
- **Uso:** obrigatório antes de `VERIFIED`

> Itens abaixo são critérios de aceite, não evidência de conclusão. Permanecem desmarcados até
> revisão independente com comandos, browser autenticado e, quando aplicável, ambiente integrado.

## 1. Escopo e preservação

- [ ] diff está dentro da allowlist;
- [ ] mudanças preexistentes foram preservadas;
- [ ] nenhum workflow/recurso de outro owner mudou;
- [ ] não houve commit, deploy ou runtime externo sem autorização;
- [ ] AGENTs/ERD/docs aplicáveis foram atualizados.

## 2. Arquitetura

- [ ] Omnichannel continua dono de FSM, lease, mensagem, outbox e sender;
- [ ] Customer Data possui apenas domínio determinístico;
- [ ] Customer Intelligence não escreve schema de fonte;
- [ ] integração usa interface pequena/adapter;
- [ ] sem SQL cross-module ou dual-write;
- [ ] falha da Inteligência preserva chat humano.

## 3. Customização e prompts

- [ ] comportamento seguro é configurável no painel/DB;
- [ ] não existe prompt de negócio hardcoded ou mega-prompt implícito;
- [ ] cada execução resolve `process_key`, definição, versão e binding;
- [ ] pipeline usa apenas processos registrados e mantém `ProcessResult`/run por etapa;
- [ ] triage intermediária não causa efeito nem é traduzida para `no_reply`;
- [ ] closure preserva resposta final e todos os gates do Omnichannel;
- [ ] camadas e precedência são determinísticas e visíveis;
- [ ] guardrail de plataforma não pode ser removido por tenant;
- [ ] draft não altera produção;
- [ ] publicação exige diff, teste, permissão e confirmação;
- [ ] versão publicada é imutável;
- [ ] rollback só troca binding e preserva histórico;
- [ ] variável é tipada e validada;
- [ ] schema, tools e fontes não são escolhidos livremente pelo prompt;
- [ ] eval cobre comportamento, schema, prompt injection, custo e latência;
- [ ] teste apenas estrutural não é apresentado como avaliação funcional com LLM;
- [ ] cada um dos treze processos possui threshold próprio aprovado, sem wildcard;
- [ ] Prompt Studio possui loading/error/dirty/stale/a11y/browser.

## 4. Banco

- [ ] migration nova, não renumerada;
- [ ] `account_id` e FKs/índices corretos;
- [ ] SQL schema-qualified/parametrizado;
- [ ] DDL e backfill pesado separados;
- [ ] backfill idempotente com watermark/checksum/exceções;
- [ ] lifecycle funcional e estado de migração legada são máquinas separadas;
- [ ] oito componentes do prompt legado terminam em mapping explícito ou pendência bloqueante;
- [ ] split legado triage/reply possui hash, diff e aceite humano antes de publicar;
- [ ] writer state impede dois escritores;
- [ ] conflict/supersede preserva evidência;
- [ ] retenção/exclusão invalida derivados;
- [ ] expiração limpa payload, preserva proveniência e registra auditoria metadata-only;
- [ ] legal hold ativo bloqueia retenção e só libera por transição auditada;
- [ ] painel de retenção separa criar draft de publicar, exige revisão/motivo/aprovação/confirmação
  e não reponta fontes existentes implicitamente;
- [ ] painel bloqueia versão obsoleta e cancela leitura/mutação se escopo ou permissão mudar;
- [ ] nenhum drop com FK/consumidor vivo.

## 5. Identidade e tenant

- [ ] owner/client/subject/relationship sem ambiguidade;
- [ ] agência/standalone seguem contrato;
- [ ] participante local não virou chamada síncrona no webhook;
- [ ] match fraco não faz merge;
- [ ] cross-client gera candidato restrito, não vazamento;
- [ ] merge possui undo;
- [ ] troca de conta limpa estado;
- [ ] recurso fora do escopo retorna 404.

## 5.1 CRM, interações offline e segmentos

- [ ] interação offline tem origem, finalidade, autor, tempo e vínculo de relacionamento;
- [ ] anexo/import exige intent, scan, idempotência, dry-run e relatório;
- [ ] segmento possui identidade estável e versões imutáveis;
- [ ] filtro de segmento usa AST/campos/operadores allowlisted, nunca SQL ou expressão livre;
- [ ] preview/materialização/run preservam tenant, policy, consentimento e versão;
- [ ] exportação/ativação de marketing exige permissão e finalidade próprias;
- [ ] policy de recomendação possui definição, versão, binding, resolução e rollback rastreáveis.

## 6. IA e envio

- [ ] IA produz proposta, não comando direto ao provider;
- [ ] resultado é schema-validado no Go;
- [ ] os treze outputs rejeitam campos desconhecidos/trailing JSON e os cinco writers headless
  aceitam somente referências presentes no contexto server-side e no mesmo escopo do banco;
- [ ] `ai_generation`/estado rejeitam resultado atrasado;
- [ ] outcome de aprendizado entra na transação aceita;
- [ ] mensagem automática percorre `PENDING -> outbox -> adapter`;
- [ ] efeito interno é idempotente;
- [ ] ambiguidade do provider é reconciliada/observável;
- [ ] timeout/falha/baixa confiança conforme policy faz fail-open;
- [ ] humano assumindo bloqueia IA.

## 7. Fontes e tools

- [ ] source/tool registrado e allowlisted;
- [ ] escopo da fonte está declarado;
- [ ] segredo não retorna ao front/log/workflow;
- [ ] paginação, timeout, retry e rate limit;
- [ ] SSRF/input externo tratados;
- [ ] disable diferencia ingestão, uso histórico, invalidação e retenção;
- [ ] aceitar `source.suggest` não configura, habilita, sincroniza nem solicita credencial;
- [ ] falha de fonte não bloqueia chat;
- [ ] escrita usa service do módulo owner.

## 8. Privacidade e portfólio

- [ ] finalidade, papéis e retenção registrados;
- [ ] context snapshot minimizado, protegido e com TTL;
- [ ] context snapshot expirado perde ciphertext/key/hash in-place, preserva apenas metadados
  mínimos referenciáveis e respeita legal hold direto ou herdado de observação;
- [ ] HMAC/criptografia para identificadores;
- [ ] exclusão/tombstone/backups/reingestão cobertos;
- [ ] observação protegida nasce mascarada e reveal exige permissão/motivo sem registrar valores;
- [ ] cross-client individual desabilitado por default;
- [ ] coorte mínima e supressão contra reidentificação;
- [ ] opt-out/consentimento vencem prompt.

## 9. Frontend

- [ ] menu, workspace, módulo, permissão e URL direta;
- [ ] front chama apenas API Go;
- [ ] estado reidratado da resposta autoritativa;
- [ ] loading, empty, error, retry e stale;
- [ ] mobile, tema, teclado, foco e labels;
- [ ] publish/rollback possuem feedback e confirmação;
- [ ] chave não reidrata;
- [ ] inbox não busca Inteligência quando desabilitada;
- [ ] áudio/transcrição e vídeo atuais aparecem como capability legada preservada até spec própria;
- [ ] `/crm`, `/automation` e `/inteligencia` não regrediram.

## 10. Rollout e rollback

- [ ] shadow aplica mesma governança de PII da produção;
- [ ] métricas por cliente/process/prompt/model/rollout;
- [ ] canary possui limite e kill switch;
- [ ] coorte canary é determinística para o mesmo tenant/relacionamento/canal e não selecionados
  permanecem sem efeito;
- [ ] rollback foi ensaiado;
- [ ] writer antigo não volta obsoleto;
- [ ] webhook deduplicado não é reprocessado;
- [ ] smoke cobre texto, mídia, áudio, reply, `fromMe`, duplicata, provider e handoff;
- [ ] vínculo canal→cliente, exceções e reparos são administráveis no painel do Omnichannel;
- [ ] deprecação possui tráfego zero e `Deprecation`/`Sunset`;
- [ ] remoção é pacote separado e aprovado.

## 11. Evidência final

- [ ] comandos e resultados anexados;
- [ ] critérios não provados declarados;
- [ ] `git diff --check` limpo no escopo;
- [ ] `git status --short` explicado;
- [ ] revisor independente assina `VERIFIED`;
- [ ] owner/orquestrador, e não o autor, decide `DONE`.

## Rejeição automática

Rejeitar se houver:

- sender fora do Omnichannel;
- segredo/PII em log, export ou front reidratado;
- prompt controlando permissão/FSM/SQL/tool livre;
- fonte de verdade local/n8n;
- dual-write sem cutover;
- match/compartilhamento cross-client silencioso;
- exclusão sem retenção/rollback;
- spec marcada `READY` com decisão material aberta.
