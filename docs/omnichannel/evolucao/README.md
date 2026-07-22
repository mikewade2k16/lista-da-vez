# Especificações executáveis — evolução omnichannel E0–E10

Este diretório transforma o `PLANO_TECNICO_EVOLUCAO.md` em contratos de execução. O plano
explica **o que** será entregue e em qual ordem; estes arquivos fixam **como**, **onde**, **por
quê**, **quem pode escrever em cada área** e **como provar que terminou**.

## Ordem de leitura obrigatória

Todo agente executor recebe, nesta ordem:

1. `AGENT.md` da raiz;
2. a skill `principios-engenharia`;
3. a skill `omnichannel-hibrido`;
4. `CONTRATO_EXECUCAO_AGENTES.md`;
5. `MATRIZ_DEPENDENCIAS.md`;
6. `CATALOGO_DESPACHO_AGENTES.md`, somente na seção do pacote;
7. somente a spec da fase e o pacote atômico que executará;
8. os `AGENT.md` locais dos diretórios que estiver autorizado a alterar.

Receber a spec de uma fase **não autoriza implementar a fase inteira**. A unidade normal de
delegação é um pacote `E<n>-<área>-<número>`, definido dentro da spec. Integração e aceite final
ficam com o agente orquestrador/revisor.

## Arquivos

| Documento | Finalidade |
|---|---|
| `CONTRATO_EXECUCAO_AGENTES.md` | regras comuns, limites, segurança, banco, Go, front, n8n e handoff |
| `MATRIZ_DEPENDENCIAS.md` | ordem, owners, paralelismo permitido, conflitos e gates |
| `TEMPLATE_PACOTE_TAREFA.md` | prompt fechado para despachar um agente executor |
| `CATALOGO_DESPACHO_AGENTES.md` | allowlists de escrita e pontos de integração por pacote |
| `CHECKLIST_REVISAO.md` | revisão independente e critérios de rejeição automática |
| `E0_OWNERSHIP_WORKFLOWS.md` | isolamento de workflows e guardas cross-module |
| `E1_PILOTO_WHATSAPP.md` | fechar texto, mídia, reply, fromMe, status, histórico e UX do piloto |
| `E2_CEREBRO_N8N_V2.md` | debounce durável, contexto, contrato estruturado e multi-turno |
| `E3_MULTIMODAL.md` | transcrição, visão e extração segura de documentos |
| `E4_CRM_ATRIBUICAO.md` | contato 360°, identidade, origem, landing page e deduplicação |
| `E5_HANDOFF_OPERACIONAL.md` | resumo de handoff, filas, SLA, tomada e transferência |
| `E6_TOOLS_CONHECIMENTO.md` | ferramentas autorizadas, RAG, auditoria e política de execução |
| `E7_WHATSAPP_CLOUD_API.md` | adapter oficial da Meta e migração controlada por número |
| `E8_INSTAGRAM.md` | DM, comentários, resposta privada e moderação no inbox único |
| `E9_HARDENING_ESCALA.md` | segurança, LGPD, observabilidade, custo, capacidade e recuperação |
| `E10_ROLLOUT.md` | shadow mode, piloto, expansão, rollback e aceite operacional |

## Resultado demonstrável primeiro

O primeiro corte visível não espera todas as fases. A sequência mínima é:

1. E0 concluída: `E0-OWN-01`, `E0-GUARD-02`, `E0-QA-03`, `E0-INT-04` e `E0-DOC-05`;
2. executar `E1-DB-01`, `E1-BE-02`, `E1-BE-03`, `E1-API-04` e `E1-FE-05` nessa ordem;
3. usar `E1-QA-06` para demonstrar em tela texto, reply, mídia, `fromMe`, paginação e erro;
4. só então ligar a IA de `E2` em modo shadow.

Essa sequência entrega algo confiável no inbox sem deixar o n8n enviar ao canal nem tocar
workflows de Automação, Calendário ou Operação.

## Regra de status

| Status | Quem pode aplicar | Significado |
|---|---|---|
| `DRAFT` | autor da spec | contrato ainda pode mudar |
| `READY` | orquestrador | dependências e decisões resolvidas; pacote pode ser disparado |
| `IN_PROGRESS` | executor | trabalho começou dentro da allowlist |
| `BLOCKED` | executor + revisor | bloqueio objetivo documentado; nenhuma expansão de escopo |
| `IMPLEMENTED` | executor | código entregue e testes do pacote passaram |
| `VERIFIED` | revisor | diff, testes e critérios independentes conferidos |
| `DONE` | orquestrador | fase integrada, documentada e demonstrável |

Um executor nunca marca a própria fase como `VERIFIED` ou `DONE`.
