# Contábil + Crow Plus — execução por agentes

## Prioridade atual — 2026-11-04

O usuário confirmou como foco central **emissão de notas de serviços pelo chat,
reaproveitando emitente, clientes, serviços e valores recorrentes, perguntando só
as lacunas**. Comprovantes externos, despesas e orientação fiscal via RAG também
fazem parte do produto. Produtos/catálogo/NF-e de mercadorias (CT-09/M3) continuam
adiados. Ver [escopo confirmado](ESCOPO_INICIAL.md),
[aceite CT-10](ACEITE_PILOTO.md) e [handoff CT-10](handoffs/CT-10.md).
O usuário ainda não testou nem aprovou a funcionalidade (2026-11-05).
A base passou apenas nos testes automáticos locais executados; jornada conversacional completa,
ambiente do painel atualizado e emissão real ainda estão pendentes.

Versão inicial: 2026-11-03. Este pacote contém especificações para execução futura; não declara funcionalidades implementadas. O usuário autorizou preparar os prompts e cadastrar o backlog em uma página Tasks chamada **Contábil + Crow Plus**. Conta de destino/autenticação devem ser resolvidas pelo contexto autenticado, nunca por inferência no banco.

## Pendências acumuladas — CT-15

[CT-15 — registro final e contínuo](prompts/CT-15.md) reúne as pendências discutidas,
MEI/não MEI, emissão, testes do usuário/código/layout/funcionalidade e otimizações.
Atualizar ao concluir cada tarefa ou receber feedback, sem substituir documentos
fontes ou assumir aprovação. O plano agora tem **15 prompts**; os14originais foram
preservados. CT-15 é mantido desde já e consolidado no fim, sem travar tarefas
independentes. Não há lembrete agendado nem task externa criada por este arquivo.

## Como distribuir

Cada arquivo em `prompts/` é um prompt completo: copiar todo o conteúdo para um agente com acesso ao repositório. Não enviar apenas título/resumo. O agente lê também os documentos apontados no prompt. Os prompts são a fonte canônica deste planejamento; `backlog.tasks.json` é uma exportação derivada para cadastro, não um endpoint de importação nativo.

Primeiro atribuir CT-01 (escopo e diagnóstico, documental) e depois CT-02 (contratos, documental). A execução do fluxo de diagnóstico é responsabilidade de CT-03/04/05. Essa separação elimina a dependência circular de construir o diagnóstico antes de definir seu modelo.

Após CT-02, distribuir por ondas. Não dar o mesmo arquivo a dois agentes ao mesmo tempo. Quem integra coordena migrations e arquivos de composição compartilhados. Não criar branches automaticamente: essa ação exige aprovação específica conforme AGENTS.md.

## Entregas para uso rápido

| Marco | Conteúdo utilizável | Dependências |
|---|---|---|
| M1 — uso assistido | Cadastro guiado, pendências, consulta financeira sem IA, preparação existente e orientação com fontes | CT-01 a CT-07 + aceite M1 de CT-10 |
| M2 — NFS-e | Conector e fluxo completo de emissão controlada para o caso homologado | M1 + CT-08 + aceite NFS-e de CT-10 |
| M3 — NF-e | Fluxo completo para o primeiro cenário fiscal de produtos explicitamente suportado | CT-03/04/05 + CT-09 + aceite NF-e de CT-10 |
| M4 — obrigações | Apuração limitada às regras implementadas e integração com pagamentos | CT-11 e requalificação CT-10 |
| Evolução | Ingestão automática, busca híbrida, avaliação Jeff/MCP e expansão do Crow | CT-12 a CT-14 |

M1 não espera por NF-e, Jeff, MCP, pgvector, fine-tuning ou escrituração completa. M2/M3 passam primeiro em ambiente local simulado; homologação externa e produção ficam identificadas separadamente e requerem acesso/autorização específicos. Nunca apresentar emissão simulada como nota autorizada.

## Ordem e paralelismo

| Prompt completo | Prioridade | Dependências para integrar |
|---|---|---|
| [CT-01 — Diagnóstico e cenários](prompts/CT-01.md) | P0 | Nenhuma; tarefa documental |
| [CT-02 — Contratos e ownership](prompts/CT-02.md) | P0 | CT-01; tarefa documental |
| [CT-03 — Domínio e motor fiscal](prompts/CT-03.md) | P0 | CT-02 |
| [CT-04 — Interface Crow Plus](prompts/CT-04.md) | P0 | CT-03 |
| [CT-05 — Roteamento e tools](prompts/CT-05.md) | P0 | CT-03; busca habilitada após CT-06 |
| [CT-06 — Conhecimento e fontes](prompts/CT-06.md) | P0 | CT-02 |
| [CT-07 — Respostas fundamentadas](prompts/CT-07.md) | P0 | CT-04, CT-05, CT-06 |
| [CT-08 — NFS-e](prompts/CT-08.md) | P0 | CT-03, CT-04, CT-05; adapter pode começar após CT-03 |
| [CT-09 — NF-e](prompts/CT-09.md) | P0 | CT-03, CT-04, CT-05; adapter pode começar após CT-03 |
| [CT-10 — Qualificação por marco](prompts/CT-10.md) | P0 | CT-04/05/06/07 para M1; CT-08 para M2; CT-09 para M3 |
| [CT-11 — Obrigações e pagamentos](prompts/CT-11.md) | P1 | CT-07 e aceite M1 de CT-10 |
| [CT-12 — Ingestão e busca híbrida](prompts/CT-12.md) | P1 | CT-06, CT-07 |
| [CT-13 — Jeff, modelos, MCP e fine-tuning](prompts/CT-13.md) | P2 | CT-07 e aceite/métricas M1 de CT-10 |
| [CT-14 — Expansão por módulo](prompts/CT-14.md) | P2 | CT-07, aceite M1 CT-10 e decisões CT-13 |
| [CT-15 — Pendências e qualificação final](prompts/CT-15.md) | P1 | Consultar artefatos CT-01–14; manutenção contínua não espera conclusão de todos |

1. CT-01 → CT-02: definições curtas e objetivas; não implementar framework nem investigar assuntos fora do piloto.
2. Após CT-02: CT-03 (domínio) e CT-06 (conhecimento) podem avançar em paralelo; migrations são serializadas.
3. Após CT-03: CT-04 (UI) e CT-05 (orquestrador) podem avançar em paralelo usando o contrato CT-02. CT-08 e CT-09 podem desenvolver adaptadores separados depois da fundação fiscal de CT-03; integração final depende de CT-04/05.
4. CT-07 após CT-04/05/06. CT-10 acompanha os aceites por marco, sem obrigar M1 a esperar emissão.
5. CT-11 após M1; CT-12 após CT-06/07; CT-13 após métricas de M1; CT-14 após qualificação do Plus e resultados relevantes de CT-13.

O limite prático de paralelismo é a independência dos arquivos/contratos. CT-08 e CT-09 não escrevem ambos o motor de emissão: esse motor pertence à CT-03. Mudança de contrato volta ao coordenador CT-02 antes de integração.

## Coordenação mínima

- Um executor por tarefa; um integrador responsável por app.go, catálogos, gating, ERD e reserva de migrations. Pode ser o próprio usuário ou um agente designado; este documento não cria agentes automaticamente.
- CT-02 registra contratos versionados, matriz de permissões e ownership. Não é necessário pedir aprovação para cada detalhe já dentro do escopo atribuído.
- Handoff de cada tarefa em `handoffs/CT-XX.md`: arquivos, contratos, validações, limitações e estado. Não inventar timestamps, custos, benchmark ou teste executado.
- Aceites verificados sobre alterações integradas, não somente em checkouts isolados. Publicação Git/deploy não está autorizada por este pacote.
- A base local tem alterações prévias do usuário; não limpar, resetar ou sobrescrever essas alterações.

## Fontes e materiais

Ler [CONTRATOS.md](CONTRATOS.md) para as decisões iniciais, [FONTES.md](FONTES.md) para coleta oficial e [PUBLICACAO_TASKS.md](PUBLICACAO_TASKS.md) para cadastro autenticado. O índice detalhado está no manifesto `backlog.tasks.json`; os 15 prompts em `prompts/` contêm especificações, restrições e testes de aceite.

## Limite do produto inicial

O primeiro Contábil é fiscal e assistivo: diagnóstico, evidências, documentos e integração financeira. Plano de contas, partidas dobradas, razão, balancete, escrituração e obrigações acessórias completas exigem planejamento próprio. Não anunciar essas capacidades como prontas.

## CT-11 — estrutura inicial, 2026-11-05

Usuário autorizou desenvolver a estrutura para MEI e empresas não MEI, deixando
regras para depois. [Cobertura](OBRIGACOES_COBERTURA.md) e
[handoff](handoffs/CT-11.md): pendências, memória/revisão/histórico e vínculo Finance
transacional testados. Nenhum cálculo fiscal real, vencimento, guia, declaração
transmitida ou pagamento bancário habilitado. CT-10 continua sem aceite do usuário.

## CT-12 — ingestão e busca, 2026-11-05

[Ingestão e decisão de busca](INGESTAO_E_BUSCA.md): coleta opt-in de origem permitida,
recuperação por jobs/checkpoints e nova versão aguardando revisão. Avaliação portuguesa
sustentou manter FTS atual; sem pgvector/embeddings. [Handoff](handoffs/CT-12.md) e
[evidências](evaluations/CT-12.json). Nenhuma subscrição ativada na conta; smoke vivo,
layout/aceite e evolução do catálogo/retrieval permanecem no CT-15.

## CT-13/14 — avaliação e preparação, 2026-11-05

[CT-13](experiments/CT-13-DECISOES.md): baseline português36/40, sem adoção de Jeff,
MCP, escalada ou fine-tuning; comparação com modelos reais pendente.
[CT-14](CALENDAR_EXPANSAO.md): Calendar inventariado e matriz preparada; integração
bloqueada pelo aceite M1 ainda ausente. Nenhum adapter/opt-in novo implementado.
Handoffs [13](handoffs/CT-13.md) e [14](handoffs/CT-14.md), evidência
[local](evaluations/CT-13-14.json) e pendências P15-36/37 preservam os limites.
