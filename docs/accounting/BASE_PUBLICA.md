# Base pública do chat — coleta, revisão e busca (CT-15 fase C)

Data: 2026-11-09. Estado: implementado e testado localmente em bancos descartáveis;
**nenhum conteúdo novo publicado na sua conta/ambiente**. Publicar é ação sua.

## Para que serve

O chat do Finance responde perguntas fiscais citando trechos de fontes oficiais, sem
IA. Antes havia só 5 resumos curtos (migration 0318). Agora há um caminho para coletar
documentos oficiais inteiros, revisar e publicar.

## Fluxo (tela Manage → Base pública, só administrador da plataforma)

1. **Coletar**: baixa a fonte do catálogo, extrai o texto, divide em trechos de até
   ~1.500 caracteres com página e seção. Fica em **rascunho**, invisível no chat.
   Mesmo arquivo (hash igual) não cria nova versão.
2. **Revisar**: ler os trechos, excluir os ruins, **exportar o texto** (.md) para outras
   IAs e para o contador e registrar cada revisão (dono, IA, contador; aprovado, pediu
   ajustes ou reprovado). Revisões são imutáveis (histórico).
3. **Publicar**: exige a **sua aprovação** como última decisão sua. A versão anterior da
   mesma fonte sai do chat na mesma transação. IA e contador não bloqueiam a publicação,
   mas aparecem no chat ("contador pendente/aprovado") em cada fonte citada.
4. **Descartar**: coleta não publicada pode ser descartada; o histórico continua.

Material publicado é imutável (trigger 0317); retirar é por status, nunca apagando.

## Catálogo fechado (código: `knowledge.PublicCatalog`)

| Chave | Fonte | Tipo | Coleta real 2026-11-09 |
|---|---|---|---|
| simples-perguntao-mei | Perguntas e Respostas MEI/SIMEI (Receita) | PDF | 45 trechos |
| simples-perguntao-sn | Perguntas e Respostas Simples Nacional (Receita) | PDF | 207 trechos |
| lc-123-2006 | LC 123/2006 no Planalto | página | **falhou**: site não respondeu (sem contorno) |
| nfse-guia-emissor-web | Guia do Emissor Nacional Web NFS-e (130 páginas) | PDF | 153 trechos |
| nfse-manual-api | Manual da API NFS-e | PDF | 11 trechos |
| mei-dasn-servico | Serviço gov.br DASN-SIMEI | página | 7 trechos |
| mei-teto-memp | MEMP: teto do MEI | página | 6 trechos |

Fora do catálogo, com motivo: FAQ NFS-e v1.2 (exige login gov.br), Resolução CGSN 140
em normas.receita (texto carregado por script; precisa de outra fonte oficial), ISS de
Aracaju (sem fonte oficial pública localizada). Incluir fonte nova exige revisar em
código: oficial, pública, sem login e aceitando download automático.

Segurança da coleta: HTTPS 443, IP público validado e fixado, sem redirect/proxy/
compressão, até 16 MiB, PDF até 400 páginas, sem OCR (PDF digitalizado é recusado em
vez de publicado parcial). HTML vira texto puro (scripts, formulários e navegação
descartados; nunca renderizado como HTML). Sumários com pontilhado, número de página e
rodapé padrão do gov.br são descartados. Bloqueio/login é falha visível, sem contorno.
O binário original não é guardado: ficam URL, hash SHA-256, tamanho e data.

## Busca sem IA

Cada termo vira um conceito: siglas e palavras curtas casam exatamente (DAS não vira
"da"), palavras longas pelo radical (declarar ↔ declaração) e grupos de sinônimos de
vocabulário ("imposto do MEI" → DAS/guia/boleto). Ordenação por quantos conceitos o
trecho cobre e depois relevância; com 2+ conceitos exige 2 cobertos, evitando trecho
sem relação. Sinônimos são só vocabulário, nunca valor ou regra fiscal.

Medição (coleta real em banco descartável, publicação automática só para medir):
perguntas congeladas antes da coleta em
`experiments/CT-15-BASE-PUBLICA-holdout.json`, 22 casos (3 sem fonte esperada):

| Método | Acerto nos 3 primeiros |
|---|---|
| Busca anterior (OR simples) | 19/22 |
| Busca por conceitos | 22/22 |

Resultado em `evaluations/BASE-PUBLICA-COLETA-2026-11-09.json`. É um conjunto pequeno
e foi o primeiro uso: não prova cobertura fiscal ampla. Próxima medição exige outro
conjunto congelado. Embeddings/pgvector seguem não adotados (decisão: sem custo agora).

## O que depende de você

- Abrir Manage → Base pública, coletar, revisar, exportar para IAs/contador, registrar
  as revisões e publicar. Até lá, o chat continua só com os 5 resumos iniciais.
- Tentar a LC 123 de novo mais tarde (Planalto fora do ar na coleta).
- Indicar fontes oficiais de Sergipe/Aracaju (ISS) e da Resolução CGSN 140 acessíveis.
