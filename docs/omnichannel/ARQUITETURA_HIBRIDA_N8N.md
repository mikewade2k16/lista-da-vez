# Arquitetura híbrida do atendimento omnichannel

Decisão de 2026-07-20: Go/PostgreSQL são a fonte de verdade e o n8n é o executor
configurável da inteligência. Nenhum workflow envia diretamente para Evolution, Meta
WhatsApp Cloud ou Instagram.

Roteiro executivo e critérios de aceite:
[`PLANO_TECNICO_EVOLUCAO.md`](PLANO_TECNICO_EVOLUCAO.md).

## Responsabilidades

| Go/PostgreSQL | n8n |
|---|---|
| Webhooks e adapters de canal | Debounce e agrupamento |
| Dedupe e idempotência | Montagem evolutiva do contexto |
| Contato único, identidades e CRM | Chamada ao modelo configurado |
| Mensagens, mídias e estados | Transcrição e visão |
| Setores, filas, responsáveis e RBAC | Uso de tools autorizadas pelo Go |
| Auditoria, custo e retenção | Decisão `continue_ai` ou `handoff` |
| Outbox e envio final | Produção de resposta estruturada |

Fluxo obrigatório:

```text
Canal -> webhook Go -> dedupe/persistência/CRM -> n8n -> validação Go
      -> estado/roteamento/outbox Go -> adapter do canal
```

O retorno do n8n é input não confiável. O Go valida o JSON Schema, confere IDs de
setor/fila/tool e só então grava estado ou cria envio no outbox.

## Configuração da IA

O painel existente de agentes continua sendo a única configuração por conta:

- `messaging.ai_agents`: liga/desliga e chave cifrada;
- `messaging.ai_agent_versions`: provider, modelo, temperatura, camadas de prompt e
  schema de saída versionado;
- `messaging.ai_runs`: auditoria, tokens, custo, latência e falhas, sem prompt/chave;
- `messaging.ai_collect_field_defs`: campos que a triagem deve coletar.

`OMNI_AI_EXECUTOR=native|n8n` é somente uma chave operacional de rollout. No modo
`n8n`, a chave crua é decifrada pelo Go e enviada apenas na chamada server-to-server.
O workflow `Omnichannel Brain` desliga a persistência de execuções e nunca devolve a
chave. O Go revalida a saída. `native` permanece como rollback até o fluxo n8n passar
pelos smokes reais.

## CRM e atribuição

O contato é único no tenant. Canais são identidades desse contato, não cadastros
separados:

- `messaging.contacts`: entidade CRM, lifecycle, tags e campos customizados;
- `messaging.contact_identities`: WhatsApp/Instagram + provider + conta externa;
- `messaging.contact_touchpoints`: origem cronológica (DM, landing page, campanha,
  comentário etc.);
- `messaging.contact_notes`: notas humanas persistidas.

No WhatsApp, o webhook já cria/vincula automaticamente contato, identidade e
touchpoint na mesma transação idempotente da mensagem. O contexto autoritativo informa
à IA se o contato já existia e canal/provider/origem. `relationship_status=customer`
deve vir de uma regra ou integração autoritativa (CRM/ERP), nunca de adivinhação do
modelo.

O primeiro ciclo multi-turno também já está ligado: `reply_draft` válido vira mensagem
PENDING no outbox do Go. Enquanto `needs_human=false`, a conversa permanece `ai_active`
e a próxima mensagem volta à IA; quando `needs_human=true`, a resposta de transição é
enfileirada e a conversa segue para roteamento humano. Falha de IA ou de outbox faz
fail-open para a fila.

Para landing pages, o receptor do Site deverá encaminhar `landing_page_id`, campanha e
UTMs ao serviço de atribuição. Horário de primeira/última interação vem do timestamp do
evento, não do relógio do workflow.

## Canais e migração segura

1. Evolution é o adapter atual de piloto.
2. Meta WhatsApp Cloud será outro `channel.Provider`; a troca acontece por número no
   painel, com janela controlada e nunca com dois providers ativos no mesmo número.
3. Webhooks oficiais da Meta entram primeiro no Go para assinatura, dedupe e
   persistência. O n8n nunca é o endpoint público autoritativo do canal.
4. Instagram usa o mesmo inbox/CRM, mas terá orquestração própria para DM e comentários,
   chamando o mesmo cérebro estruturado. Resposta automática a comentário começa com
   allowlist, limite por post e opção de aprovação humana.

## Segurança e portabilidade dos workflows

- JSON versionado deve ter sempre `pinData={}` e `staticData=null`;
- credenciais exportadas podem conter apenas referência `{id,name}`, nunca segredo;
- o novo cérebro não usa credential do n8n: provider/modelo/chave vêm do banco/Go;
- execuções de sucesso, erro, progresso e manuais ficam sem persistência no workflow;
- limpar o arquivo atual não remove dados de commits antigos: eventual limpeza do
  histórico Git exige decisão explícita e rotação dos segredos afetados.

Esta arquitetura vale somente para os workflows do omnichannel. `workflow-whatsapp.json`
e WAHA pertencem ao módulo `automation`; Calendar e Operação também possuem workflows
próprios. Uma tarefa deste módulo nunca edita, importa, exporta, ativa ou desativa workflow
de outro owner. Validações em scripts compartilhados devem ser escopadas pelos ids do módulo.

## Próximas entregas

1. Trocar o booleano temporário `needs_human` por decisão explícita versionada
   (`continue_ai|handoff`) e definir teto de turnos/timeout.
2. Debounce, áudio, visão e tools no n8n, mantendo callbacks validados no Go.
3. Painel CRM com identidades, touchpoints, lifecycle, tags e notas persistidas.
4. Atribuição de landing pages/UTMs integrada ao receptor do Site.
5. Adapter Meta WhatsApp Cloud e migração por número.
6. Adapter Meta Instagram + workflow específico de DM/comentários.
