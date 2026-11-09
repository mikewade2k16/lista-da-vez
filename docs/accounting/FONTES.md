# Fontes oficiais para CT-01, CT-06, CT-08, CT-09 e CT-11

Referências identificadas na pesquisa de 2026-11-03. Revalidar documentos, URLs, versões, vigência e condições de uso na execução. Esta lista não é parecer fiscal nem comprovação de cobertura de uma empresa específica.

| Uso | Fonte oficial | O que coletar |
|---|---|---|
| NFS-e | https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual | Manuais vigentes, anexos de domínio/leiaute, XSD e regras de validação |
| Ambientes NFS-e | https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/apis-prod-restrita-e-producao | Endpoints documentados de homologação/produção e requisitos de integração |
| NF-e | https://www.nfe.fazenda.gov.br/portal/ | MOC da NF-e pertinente, notas técnicas vigentes, esquemas, tabelas e cronogramas; não confundir NF-e com NFCom/NFAg/NF-e ABI |
| Receita Federal | https://www.gov.br/receitafederal/pt-br/acesso-a-informacao/legislacao/legislacao | Legislação federal e acesso ao sistema Normas |
| Pesquisa normativa | https://www.gov.br/receitafederal/pt-br/acesso-a-informacao/legislacao/como-fazer-suas-pesquisas-no-sistema-normas | Método de pesquisa e identificação de atos oficiais |
| Simples Nacional | https://www8.receita.fazenda.gov.br/SimplesNacional/ | Legislação e manuais oficiais do regime efetivamente confirmado |
| CFC | https://cfc.org.br/tecnica/normas-brasileiras-de-contabilidade/normas-completas/ | Normas pertinentes ao porte/atividade e versões aplicáveis |
| Legislação federal | https://www.planalto.gov.br/ | Textos oficiais e alterações, incluindo LC 116, LC 123 e legislação da reforma aplicável |
| Legislação estadual | Portal oficial da SEFAZ da UF do emitente e CONFAZ | ICMS, credenciamento, regras da operação e documentos fiscais pertinentes |
| Legislação municipal | Portal oficial da prefeitura/finanças do município do emitente | ISS, cadastros, regras locais e integração utilizada |

Planalto/portais fiscais podem bloquear coleta automatizada ou redirecionar. Registrar erro e permitir upload manual de cópia oficial com origem verificável. Não contornar proteção nem usar resultado de busca/resumo de IA como substituto do texto integral revisado.

## Curadoria mínima

Começar pelos cenários de CT-01. Para cada fonte registrar órgão, título, URL, versão, hash, publicação, vigência verificada, jurisdição, data da coleta e revisor. Preservar documento original em storage autorizado, limites de tamanho/páginas e referência do trecho. Material privado fica separado do corpus público curado; nunca publicar documento do tenant por inferência.

Importação → extração → rascunho de versão → revisão → publicação atômica. Manter versões anteriores necessárias para responder sobre competências antigas. Mudança de lei/leiaute gera revisão e revalidação dos casos; nunca altera código de cálculo ou emite notas automaticamente.

Testes automáticos devem usar fixtures sintéticas/minimizadas e snapshots licenciados; acesso vivo a fontes é uma coleta controlada, não dependência de uma suíte determinística. Nenhum certificado, CPF real, token, login ou sessão deve entrar no repositório.

## Referências de tecnologia para avaliações futuras

- Jeff código/licença: https://github.com/firelex/jeff
- Jeff base/model card: https://huggingface.co/mstrasser/jeff-base
- Adaptador de seleção de tools: https://jeffhub.ai/adapters/tools
- MCP: https://modelcontextprotocol.io/docs/learn/architecture
- pgvector: https://github.com/pgvector/pgvector

Esses itens são candidatos, não dependências do piloto. CT-13 revalida versões, disponibilidade, licenças por artefato e métricas em português. Documentação/licenças de código, pesos, adapters e dados podem diferir.

## Seleção inicial CT-06 — 2026-11-04

Duas seleções oficiais mínimas conferidas, informativas, com origem/hash dos bytes
selecionados e limites de uso em [manifesto](corpus/manifest.json). Não são cópias
integrais nem legislação curada para cálculo. [Roteiro de carga](corpus/README.md)
exige conta/emitente autenticados, storage disponível e revisão explícita. Nenhum
corpus foi publicado em conta real; testes usam fixtures sintéticas identificadas.

## Revalidação NFS-e CT-08 — 2026-11-04

Referências, versões observadas e falhas de coleta estão em
[nfse/fontes.json](nfse/fontes.json) e [cobertura](NFSE_COBERTURA.md). PDF lido por
extrator público; ZIP/Swagger não coletados integralmente. Não há hash original
fictício nem alegação de validação contra XSD oficial. Cópia oficial fornecida por
canal autorizado pode desbloquear a qualificação; não contornar403.

## Monitoramento inicial CT-12 — 2026-11-05

A documentação atual do portal NFS-e referencia o manual de contribuintes API v1.2
(out2025),6páginas, como primeira origem candidata de coleta limitada. O guia web
v1.3 tem130páginas e não cabe no extrator atual de10páginas. URLs, decisão e limites
em [INGESTAO_E_BUSCA](INGESTAO_E_BUSCA.md). Pesquisa não é importação em conta nem
revisão de vigência fiscal. Nenhuma coleta foi habilitada/publicada automaticamente.

## Verificação tecnológica CT-13 — 2026-11-05

[Manifesto](experiments/CT-13-artifacts.json) registra revisões/URLs imutáveis e hashes
de metadados/cards/licenças públicas coletados. Código e pesos têm declarações de
licença distintas; pesos não foram baixados ou executados. Ver
[decisões e limites](experiments/CT-13-DECISOES.md); coleta não comprova qualidade
portuguesa, custo, proveniência integral do treino ou adoção no produto.
