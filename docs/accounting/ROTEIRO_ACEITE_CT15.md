# Roteiro único de aceite — CT-15, pacotes 1 a 8

Para: você, testando no ambiente local antes de qualquer commit/deploy.
Estado: código e testes técnicos entregues em 2026-11-06; **nada está aprovado** até
você marcar cada item. Marque `[x] Aprovado` ou `[x] Reprovado` e anote o que viu.
Use só dados fictícios. Não informe senha no chat; não use certificado real.

## Antes de começar

1. API local atualizada (aplica as migrations 0319–0321 ao subir):
   `docker compose -f docker-compose.yml -f compose.mac-local.yaml up -d --build api`
2. Entrar no painel, abrir **Finance** e o **Crow Assistant Plus** (chat principal).
3. Dados sintéticos sugeridos:
   - Planilhas na conta: duas de **2026-11** (ex.: “Casa novembro” e “Estúdio
     novembro”) e uma de **2026-10**, com algumas saídas e entradas.
   - Perfil fiscal pessoal completo com dados fictícios (Finance → perfil).
   - Um comprovante fictício (PDF/imagem de recibo de transporte de R$ 32,50).
4. Teste em desktop e mobile, tema claro e escuro, quando indicado.

## Pacote 1 — Busca com fontes sem IA (P15-38)

IA desligada. No chat, perguntar “qual o limite de faturamento do MEI?”.
- Esperado: trecho de fonte pública com origem/versão, sem chamar IA; a ressalva de
  que o conteúdo é informativo. Pergunta sem fonte (“qual a alíquota de ISS de
  Marte?”) → aviso de que não há fonte publicada, não uma resposta inventada.
- [ ] Aprovado  [ ] Reprovado — notas:

## Pacote 2 — Base fiscal inicial (P15-22/P15-33)

Perguntar sobre DASN-SIMEI, NFS-e e Simples Nacional.
- Esperado: respostas citam as 5 sínteses públicas iniciais; nenhuma apresenta
  valor como regra de cálculo. Cobertura ampla continua pendente.
- [ ] Aprovado  [ ] Reprovado — notas:

## Pacote 3 — Entendimento e continuidade (P15-02/P15-06/P15-36)

1. “Crow, você pode ver minha planilha por favor?” → consulta, sem formulário.
2. “faz uma nota pra mim” → pergunta só o cliente.
3. Sem responder, “quanto eu gastei esse mês?” → mostra a consulta e lembra
   “seguimos com a nota: Para qual cliente…”. O texto **não** vira nome do cliente.
4. “mostra meu saldo e depois faz uma nota” → pede para escolher o primeiro.
5. Responder o nome do cliente; depois “deixa pra lá” → mantém o que foi salvo.
6. “vamos continuar” → repete a próxima pergunta (CPF/CNPJ).
7. “saldo de outubro” sem mês no chat → pergunta o período/usa o ano do chat.
8. “como emitir nota fiscal?” → busca de fontes, não preparação.
- [ ] Aprovado  [ ] Reprovado — notas:

## Pacote 4 — Clientes recorrentes e novos (P15-03/P15-04)

1. “faz uma nota de 500 pro João Teste” → não pergunta cliente nem valor; pede CPF/CNPJ.
2. Informar CPF fictício válido, competência e serviço → resumo e “Salvei João Teste”.
3. “faz outra nota pro João Teste” → abre **outro** rascunho e diz “Usei o cadastro salvo
   de João Teste: CPF/CNPJ final …, serviço …, valor R$ 500,00”; pergunta só competência.
4. “nota de 650 para João Teste” + competência → “Atualizei o cadastro (versão 2)”.
5. Criar “João Segundo” e depois “criar nota pro João” → lista numerada; responder “2”.
6. Com outro usuário da mesma conta: “nota pro João Teste” → não reaproveita nada.
- [ ] Aprovado  [ ] Reprovado — notas:

## Pacote 5 — Planilha do chat independente da página (P15-26)

1. Página do Finance na conta A; seletor do chat na conta B.
2. No chat: “resumo de outubro de 2026” → lê a planilha de B; a página continua em A.
3. “resumo 2026-11” com duas planilhas em B → lista numerada; responder o número.
4. “listar saídas” → usa a planilha escolhida no chat.
5. Mês sem planilha (“resumo 2026-09”) → aviso, sem erro.
6. Trocar a página para C → o chat continua em B.
- [ ] Aprovado  [ ] Reprovado — notas:

## Pacote 6 — Comprovante → lançamento (P15-05/P15-18)

1. Anexar o comprovante fictício pelo chat e conferir valor/data extraídos.
2. “lança esse comprovante na planilha” → proposta com planilha, “saída pessoal”,
   descrição e se vai criar ou vincular a um lançamento de mesmo valor. Nada gravado ainda.
3. “é despesa da empresa” → proposta muda para empresarial.
4. “sim” → “Lancei/Vinculei…”; conferir na planilha (efetivado na data do comprovante).
5. Pedir “lançar comprovante” de novo → “já está lançado”.
6. Com usuário sem permissão de editar planilhas: “sim” → recusa, nada gravado.
7. “criar nota” depois → não reaproveita o comprovante de despesa como nota.
- [ ] Aprovado  [ ] Reprovado — notas:

## Pacote 7 — Partes fiscais técnicas (verificação técnica, sem tela)

Não há tela nova. Itens verificados por testes automatizados e documentos:
- XSD oficial NFS-e 1.01 versionado com hash (docs/accounting/nfse/xsd).
- Produtor de DPS sem assinatura valida contra a estrutura oficial (achado: padrão da
  série no XSD oficial precisa ser conferido na homologação).
- Regras de obrigação: rascunho → ativação por outra pessoa → aposentadoria (API).
Dependem de você: escolha da biblioteca de assinatura XML, certificado (A1 de teste
ou real), dados reais do emitente, credenciamento/homologação e regras oficiais.
- [ ] Ciente das pendências  — notas:

## Pacote 8 — Qualidade, segurança e desempenho

- Rodada integrada: 296 testes Go passaram com -race em bancos descartáveis, 0 falhas;
  skips restantes só para arquivos privados de OCR e R2 ao vivo (não executados).
- Lint Go Finance/Accounting/App: 0 ocorrências (eram 6). vue-tsc: 410 diagnósticos
  antigos, nenhum nos arquivos destes fluxos.
- Desempenho local do serviço (sem HTTP): p95 ≤ 12 ms com 240 planilhas e 500 clientes
  (evaluations/P15-PACOTE8-DESEMPENHO-2026-11-06.json). Não é SLA.
- Visual: conferir nas etapas acima desktop/mobile, temas, teclado (Tab/Enter/Esc),
  rolagem do chat e mensagens longas (listas numeradas, propostas).
- [ ] Aprovado  [ ] Reprovado — notas:

## Fase B — Comprovantes, retenção e clientes (2026-11-09)

1. Finance → configuração (engrenagem) → **Comprovantes enviados**: o comprovante do
   pacote 6 aparece com “Lançado como saída…” e “Original guardado até …”.
2. Apagar a linha na planilha e voltar à lista → aviso “O lançamento … foi apagado”.
3. **Baixar original** → abre o mesmo arquivo enviado. **Apagar original agora** → pede
   confirmação; depois mostra “apagado manualmente”, valor e data continuam.
4. **Retenção dos originais** → padrão 3 meses; mudar para 6 meses, salvar, ver histórico.
   Usuário sem permissão de configuração vê o prazo mas não salva.
5. Perfil → dados fiscais → **Clientes salvos** → editar valor do João Teste → “Salvo como
   versão N”; Histórico mostra “editado na tela”.
- [ ] Aprovado  [ ] Reprovado — notas:

## Fase C — Base pública do chat (2026-11-09)

1. Menu Manage → **Base pública** (só administrador da plataforma).
2. **Coletar todas as fontes**. Esperado: 6 coletadas; LC 123 pode falhar se o Planalto não
   responder (mensagem clara, sem contorno).
3. Abrir uma fonte → Trechos → excluir um trecho ruim → **Exportar texto para revisão**
   (arquivo .md) → mandar às outras IAs/contador.
4. Registrar revisões (IA, contador) e a sua (dono, aprovado) → **Publicar**.
5. No chat: “qual o prazo para pagar o DAS?” → trecho do Perguntão com página e linha
   “Revisão: dono aprovado · … · contador …”.
6. Pergunta sem fonte (“restituição do IRPF com dependentes”) → aviso de que não há fonte.
- [ ] Aprovado  [ ] Reprovado — notas:

## Depois do teste

Envie os itens reprovados com o passo e o texto que apareceu. Aprovação de um pacote
não aprova emissão real, cálculo de imposto nem deploy.
