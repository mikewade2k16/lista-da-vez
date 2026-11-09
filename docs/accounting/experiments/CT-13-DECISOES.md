# CT-13 — decisões de adoção

Data: 2026-11-05. Estado: avaliação local parcial; nenhuma tecnologia opcional
adotada e nenhum aceite do usuário presumido. M1 ainda não tem aceite funcional
nem métricas de uso real. O pedido autorizou investigar/desenvolver CT-13/14,
não instalar modelos pesados, gastar API ou ativar recursos em produção.

## Benchmark reproduzível

Critérios fixados em CT-13-criteria.json antes da execução:98%de rota exata,
zero tool em pedidos proibidos, zero preparação inesperada e nenhuma IA extra em
consulta determinística. Meta exploratória p95total250ms só vale para futuro teste
no mesmo hardware; não é SLA do produto. Threshold de probabilidade não foi calibrado
porque nenhum classificador opcional foi executado.

Dataset sintético português:3exemplos de calibração separados e40holdout distribuídos
em consulta determinística, busca, explicação, preparação, ambiguidade e proibição.
Nenhuma conversa/dado de conta foi usado. Harness em finance/plus_routing_experiment_test.go
chama o roteador CT-05 real; não executa tools, acesso de conta, cálculo ou rede.
Critérios, dataset, código hash, matriz de confusão, casos individuais e hardware
Go/container estão em CT-13-baseline.json. Nenhum ajuste foi feito no default.

Resultado:36/40(90%)rotas exatas; zero seleção proibida e zero preparação inesperada.
Quatro pedidos compostos receberam somente a primeira rota (leitura/busca/explicação/
revisão), quando o oracle esperava esclarecimento. São gaps de compreensão, não
prova de mutação fiscal ou bypass de autorização. Resolver em tarefa própria e
repetir avaliação com holdout novo; não aumentar a nota afinando neste teste.

p50 das médias por caso45,560µs; p95das médias353,774µs,100repetições/caso com -race.
São médias de classificação local instrumentada, não latência de nota/chat/produção.
Custo de hosting não medido=null; chamadas IA=0. Nenhum custo/economia de Jeff ou
modelo comercial foi medido. Simulações timeout/OOM/unknown-tool/baixa confiança
exercitam fallback isolado para baseline; não representam Jeff instalado ou adapter
optional já implementado no runtime.

Repetição: em back/, CT13_BENCHMARK_OUTPUT em arquivo temporário e
`go test -race -count=1 ./internal/modules/finance -run TestCT13`.
Requer apenas Go/imagem de ferramentas, sem PostgreSQL, provedor ou credencial.

## Artefatos, licenças e limites

Manifesto CT-13-artifacts.json registra URLs imutáveis, revisões resolvidas e SHA256
de LICENSE/README/metadata/cards coletados. **Pesos não baixados**; hash dos pesos
é null, não hash fictício. Nenhum código remoto foi executado.

- [Código Jeff](https://github.com/firelex/jeff): licença MIT verificada no LICENSE
  da revisão fixada; versão do pacote registrada separadamente. Não extrapolar MIT.
- [Base jeff-base](https://huggingface.co/mstrasser/jeff-base): revisionv1.3 resolvida
  por API pública; card declara pesos Apache-2.0 e foco inglês/texto. Não há evidência
  Omni de qualidade em português; não interpretar modelo pequeno como contador.
- [Adapter tools](https://jeffhub.ai/adapters/tools): revisionv1.3 também resolvida
  no Hub, card/licença Apache-2.0 registrados. Seleciona próximo tool; não preenche
  argumentos. Dados declarados incluem geração/verificação por outros modelos;
  disponibilidade/card não provam qualidade, direitos de qualquer novo dataset ou
  segurança do uso. Bases/adapters precisam de compatibilidade de revisão.
- Base referencia dados públicos/sintéticos com licenças distintas e origens da
  versão anterior, sem dataset de treinamento integral publicado. Revisar proveniência
  por artefato antes de instalação/uso. Declaração do mantenedor não é auditoria legal.
- [MCP](https://modelcontextprotocol.io/docs/2026-07-28/learn/architecture): protocolo
  de integração, não classificador nem fonte de autorização. Não foi instalado SDK
  ou servidor, e nenhuma tool do banco/cofre/shell foi exposta.

Números publicados pelos fornecedores usam dados/hardware diferentes; não foram
copiados como resultado Omni. A tabela de comparação é deliberadamente incompleta:

| Alternativa | Medido aqui | Latência/custo real | Decisão |
|---|---|---|---|
| Roteador CT-05 |40casos/classificação local | Tempos instrumentados, hosting desconhecido | Manter default; tratar gaps de múltiplos pedidos |
| Jeff + adapter tools | Metadados/licença/disponibilidade | Não executado/null | Experimentar depois, sem instalar agora |
| LLM configurado como roteador | Nenhuma chamada paga | Não executado/null | Experimentar só com modelo/credencial/orçamento autorizados |
| Escalada para modelo mais caro | Nenhum ganho demonstrado | Não medido | Não ativar |

## MCP: decisão independente

Não construir servidor sem consumidor concreto aprovado. Candidato inicial seria
somente knowledge.search.v1 sobre a mesma porta já autenticada, nunca SQL bruto.
Mapear sessão→Principal/conta/emitente, schemas fechados, RBAC/grants, vigência,
limites, revogação, requestId/auditoria e texto recuperado como dado. Body MCP não
concede conta/permissão; autenticação do transporte não substitui autorização da
operação. Sem consumidor definido não há implementação nem teste MCP de auth;
registrado como não executado, não aprovado. Estimar esforço após escolher cliente
concreto e versão/protocolo; custo atual desconhecido.

## Fine-tuning: decisão independente

Não treinar nesta etapa. Faltam falhas recorrentes reais isoladas de contrato/
retrieval/prompt, dataset consentido/licenciado/minimizado por conta, orçamento e
avaliação independente. Quatro gaps sintéticos do roteador não justificam treino.
Atualização de legislação pertence à curadoria/RAG/regras vigentes, não à memória
de pesos. Nenhum dado privado exportado/treinado e nenhum custo contratado.

## Gates de qualquer experimento futuro

Aceite M1, consumidor/caso definido, recursos autorizados, artefatos fixados,
calibração em split próprio, comparação no mesmo hardware/holdout, custo total
(incluindo hosting) e política por conta versionada. Adapter opcional exigirá
kill switch, timeout, threshold, fallback e schema/registry; baixa confiança nunca
executa operação nem remove autorização/confirm. Até lá baseline fica intacto.
CT-15 registra o trabalho, incluindo correção/avaliação de pedidos múltiplos.
