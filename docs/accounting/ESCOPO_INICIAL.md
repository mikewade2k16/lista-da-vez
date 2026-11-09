# Escopo inicial confirmado — 2026-11-04

Decisão expressa do usuário durante o início do CT-09. Esta atualização de
prioridade prevalece sobre a ordem original dos 14 prompts; não declara novas
capacidades implementadas nem altera contratos de API.

## Refinamento do foco — CT-10, 2026-11-04

A prioridade central confirmada pelo usuário é **emitir notas de serviços pelo
chat, quando solicitado**, para clientes recorrentes e novos. Reutilizar cadastro
do emitente, destinatários, serviços e valores recorrentes já persistidos; perguntar
apenas o que estiver ausente, conflitante ou desatualizado. Para uma operação nova,
o usuário informa empresa/valor e o sistema completa com dados válidos já salvos;
se faltarem dados obrigatórios ou houver homônimos, perguntar de forma pontual.
Identificação cadastral não deve ser usada para inventar descrição de serviço,
regime, alíquota ou valor. Reutilização depende de registros efetivamente existentes.

O objetivo é uma jornada simples, com resumo da operação e autorização de envio
no chat, sem recadastrar tudo nem navegar por vários formulários. Revisão e envio
continuam atos distintos no motor; a interface poderá coordená-los com pouca fricção.
Não emitir por conversa ambígua ou tornar dados de um cliente acessíveis a outro.

Conhecimento fiscal via RAG e orientação contábil ampla também são objetivos do
produto, a aprimorar progressivamente. Fontes, vigência, contexto e limites de
cobertura precisam aparecer nas respostas; o RAG atual não comprova conhecimento
universal nem declarações/cálculos implementados. Comprovantes externos continuam
no escopo; catálogo/produtos e NF-e de mercadorias continuam adiados.

Critérios ainda não entregues integralmente: cadastro/reutilização empresarial de
clientes e recorrências, preenchimento conversacional dessas operações e emissão
real qualificada CT-08. CT-10 deve evidenciar esses gaps, não aprovar o produto
completo só porque diagnóstico, preparação ou busca funcionam isoladamente.

## Uso prioritário

O módulo deve tratar informações externas ao painel, sem depender de catálogo,
Cardápio, ERP ou estoque. Casos indicados pelo usuário:

- Enviar recibos de Uber, comprovantes de pagamentos, compras e outros documentos
  externos para organizar despesas e evidências.
- Receber ajuda para identificar o documento fiscal adequado e preparar sua
  emissão quando houver uma operação própria que a justifique.
- Pedir orientação sobre retirada de lucros e alternativas legais de redução de
  impostos, considerando dados e enquadramento efetivamente confirmados.
- Preparar informações e assistência para declarações de impostos e demais
  obrigações, com cobertura e prazos explicitamente suportados.

Comprovante de despesa, documento fiscal recebido e nota emitida pelo usuário são
objetos distintos. Não converter automaticamente uma compra ou recibo de Uber em
nota própria. A implementação deve identificar o objetivo do envio e a natureza da
operação antes de propor um lançamento, uma obrigação ou preparação de emissão.
OCR/modelo apenas propõe dados extraídos: original privado, origem, revisão humana,
permissões e rastreabilidade continuam necessários.

O produto deve atender MEI e empresas não MEI, inicialmente em Sergipe, com
arquitetura extensível a outras localidades. Não inferir Simples Nacional,
atividade, município, elegibilidade de despesa, imposto ou regra de distribuição
apenas pela descrição ou pelo porte informado. Orientações tributárias futuras
exigem fontes oficiais atuais e contexto confirmado; não tratar resposta do modelo
como cálculo/apuração ou declaração transmitida.

## Mudança de prioridade

CT-09 (cadastro fiscal de produtos/conector NF-e de mercadorias) fica **adiado por
decisão do usuário**, fora do escopo inicial. Não criar classificação de produtos,
integração de catálogo, estoque, séries/numeração NF-e ou dependência de ERP para
atender os casos acima. Retomar apenas quando o usuário recolocar essa frente no
escopo. O suporte estrutural anterior a itens/NF-e pode permanecer, sem promessa
de emissão qualificada. As pendências do CT-08 continuam registradas no AGENT.

Próximo passo da sequência: CT-10, qualificação do M1 assistido existente. M3 fica
adiado, sem impedir M1. A qualificação deve registrar quais casos acima já são
suportados e quais exigem contrato/implementação adicional. Não marcar o fluxo de
comprovantes externos como pronto apenas porque existe busca em bases fiscais.

## Lacunas a especificar e qualificar

- Entrada privada de comprovantes externos: formatos/limites, owner e retenção,
  extração revisável, deduplicação e classificação sem alterar o original.
- Vínculo com despesas/lançamentos e conciliação pelo owner Finance, evitando
  catálogo ou contabilidade paralelos; distinguir uso pessoal e empresarial.
- Assistência sobre lucro/tributação com contexto, vigência, fontes e limites
  claros; cálculo somente para regras implementadas e testadas.
- Definir as primeiras declarações/obrigações por regime, período e jurisdição;
  separar checklist, preparação, cálculo, entrega e recibo oficial. CT-11 é a
  frente existente a revisar para essa cobertura, não prova de entrega completa.

CT-12 trata ingestão de fontes de conhecimento: isso não substitui o fluxo de
comprovantes privados. Os gaps acima precisam de especificação e aceite próprios,
reutilizando Storage, Finance e o motor existentes. Nenhuma transmissão fiscal,
movimentação financeira ou publicação está autorizada por este documento.
