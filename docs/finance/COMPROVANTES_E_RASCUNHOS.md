# Comprovante → rascunho de NFS-e

Implementação local de 2026-11-02. O usuário envia uma imagem de comprovante para preparar uma nota; não precisa enviar uma nota fiscal existente.

## Como usar

1. Finance → Crow Assistant → **Enviar comprovante e preparar nota**.
2. Escolher PDF, JPEG, PNG, TXT, CSV ou OFX de até8MB; clicar **Ler documento e preencher rascunho**. Imagens: até24megapixels. PDF: até10páginas.
3. Conferir valor, data de pagamento, pagador e recebedor extraídos. O rascunho já fica salvo no banco. Reenviar os mesmos bytes abre o mesmo rascunho e preserva correções manuais.
4. Completar tomador/CPF-CNPJ, competência e descrição do serviço. Selecionar um template copia seus campos para o rascunho; os dados do emitente continuam no perfil fiscal.
5. **Salvar rascunho**; recarregar mantém os dados. O seletor mostra os10 documentos pessoais mais recentes. Os avisos de campos faltantes refletem a versão salva.
6. **Consultar IA sobre este rascunho** prepara uma pergunta no campo do chat. Revisar e clicar Enviar. Requer configurar/ativar o modelo no Finance; OCR e edição manual funcionam sem chave de IA.

O botão de consulta não envia automaticamente, não modifica o perfil e não emite nota. A execução no portal fiscal e a aprovação vinculada à emissão continuam pendentes.

## Regras de extração

OCR local Tesseract com idiomas português/inglês e segmentação por colunas. Não envia a imagem a um provedor externo nem consome créditos de IA. Fonte: [documentação oficial do Tesseract](https://tesseract-ocr.github.io/tessdoc/Command-Line-Usage).

A extração estruturada inicial reconhece comprovantes de transferência: valores `R$`, data brasileira textual/numérica e seções Origem/Destino. Valores diferentes no documento ficam sem sugestão, para escolha manual. Outros layouts podem produzir somente texto e campos vazios; não é um extrator universal de documentos fiscais. PDF, JPEG/PNG, TXT, CSV e OFX são aceitos. HEIC e documentos Office não são aceitos nesta etapa.

Pagador e recebedor não são automaticamente atribuídos a emitente/tomador. A data do pagamento não é usada como competência. CNPJ do banco no rodapé não é usado como documento do tomador. O usuário escolhe/preenche os papéis fiscais; a IA recebe instrução de tratar texto de documento como dados não confiáveis, sem executar suas instruções.

## Persistência e segurança

Migration0306: `finance.document_drafts`, propriedade por `account_id` + `Principal.UserID`, chave única de hash por conta/usuário, índice de atualização. Rotas sob membership, módulo Finance e `finance.sheets.view`, para autoatendimento pessoal:

- GET `/v1/finance/documents`:10mais recentes.
- POST `/v1/finance/documents/extract`:multipart com único arquivo `file` (`image` aceito por compatibilidade).
- PUT `/v1/finance/documents/{id}`:`fields` + `expectedUpdatedAt`. Versão velha409; documento alheio404.

O documento original e as páginas renderizadas são processados em memória/pipe e descartados. Persistem nome, hash, texto OCR e campos editados; não persistir imagem/base64 em logs, conversa ou Git. O contexto de IA inclui até3rascunhos pessoais recentes com texto limitado; a consulta explícita também inclui campos do rascunho selecionado. Sem credenciais de portal.

Corpo, formato real, dimensões/pixels e saída OCR limitados; prazo35segundos, no máximo2processos OCR simultâneos por API. Sem shell com dados do usuário. Valores, datas, tamanhos e vínculo ao template validados no backend. Não há alteração em planilhas pelo upload.

O Dockerfile principal e o do Mac instalam Tesseract/português e Poppler. Para API executada nativamente, instalar o mesmo runtime; ausência retorna503 acionável. Nenhuma variável secreta nova.

## Testes e evidências

- `document_test.go`: valor/data, nomes nas seções, ausência de atribuições fiscais inventadas, valores ambíguos e imagens inválidas. `TestDocumentRealImageOCR` é opt-in por `FINANCE_OCR_TEST_IMAGE`; arquivo privado fica fora do Git e nunca é impresso pelo teste.
- `document_integration_test.go`: PostgreSQL descartável, migration reaplicável, gravação/recarga, conflito de versão, deduplicação preservando revisão e isolamento de conta/usuário. Usar `FINANCE_TEST_DATABASE_URL` com banco `finance_test*`.
- `useFinanceDocuments.test.ts`: hidratação autoritativa, resposta de upload atrasada após troca de conta, falha de salvamento preservando draft e versão usada no PUT.
- Comprovante real fornecido pelo usuário: teste OCR real aprovado; reconheceu R$22.000,00 e08/10/2026, pagador e recebedor. Não extraiu competência/tomador arbitrários nem documento bancário como fiscal.
- UI Chrome local: escolha/upload da imagem real, extração, preenchimento e salvamento de edição manual aprovados. Rascunho real mantido para revisão do usuário; marca temporária QA removida após verificação.

Não foi emitida nota, nem acessado portal autenticado. Resultado de OCR exige conferência humana, especialmente nomes e valores. Chave/modelo ausentes continuam impedindo conversa real com IA, mas não a leitura local do comprovante.

Verificação final:18testes Vitest Finance passaram; Go Finance/App e integração PostgreSQL passaram; teste OCR com o arquivo real passou. GolangCI-Lint Finance/App:0issues. ESLint dos novos arquivos:0erros/avisos. Typecheck global continua com dívida anterior fora do Finance, sem diagnóstico nos novos arquivos. Build API local aprovado; sem build web completo, commit, push ou deploy. A observação temporária de QA foi removida pela interface e salva; rascunho do comprovante mantido para o usuário.

## PDF e extratos de texto (continuação)

PDFs são lidos por página com Poppler. Texto nativo é usado diretamente; páginas com pouco texto são renderizadas com dimensão limitada e passam pelo OCR local. Prazo total90s, até10páginas e32.000bytes de texto; documentos acima desses limites são rejeitados com orientação para dividir. Não truncar o extrato silenciosamente. PDFs que exigem senha para abrir ou não permitem cópia/extração são rejeitados; criptografia de permissões que permite leitura/cópia não impede a leitura autorizada.

Centavos em fonte menor/elevada são recompostos apenas quando a geometria das palavras confirma o formato. Evita interpretar12 com centavos00 como1200. Datas brasileiras por extenso e rótulos de comprovante de cartão também são reconhecidos; isso não atribui tomador/emitente fiscal.

TXT/CSV/OFX aceitam UTF-8 ou Windows-1252, com limites de texto e rejeição de binários. OFX exige marcador de formato. O texto é disponibilizado para revisão/chat; aceitar múltiplos lançamentos não significa gerar automaticamente uma nota para cada lançamento. Não executar macros, scripts ou instruções do documento.

Testes em `document_pdf_test.go`: PDF textual, OCR de cada página digitalizada, limite de páginas, arquivo corrompido, formatos comuns, codificação e centavos elevados. O teste opt-in `FINANCE_OCR_TEST_PDF` usa o comprovante privado fornecido sem copiá-lo para o Git. A leitura dele retornou valor12,00 e data31/10/2026; tomador e competência ficaram sem inferência fiscal.

Referências técnicas: [Poppler no Debian](https://packages.debian.org/en/bookworm/arm64/poppler-utils), [manual pdftoppm](https://manpages.debian.org/unstable/poppler-utils/pdftoppm.1.en.html).

Validação PDF concluída na UI Chrome: seletor mostrou PDF/TXT/CSV/OFX, upload do comprovante enviado retornou `Documento lido. Rascunho salvo`; valor12,00 e data31/10/2026, pagador e recebedor preenchidos. Rascunho mantido para o usuário; nenhuma nota emitida. Quatro testes Vitest do editor passaram, ESLint passou (teste validado com `--no-ignore`), testes Go Finance/App e teste real do PDF passaram. Testes SQL existentes foram pulados nesta rodada por ausência de banco descartável, sem mudanças no contrato de persistência. A execução adicional de GolangCI foi interrompida durante instalação/compilação para aliviar a concorrência no ambiente local; não registrar lint Go final como aprovado nesta extensão. A aprovação de lint0issues acima é histórica da etapa de imagens.

## Identificação de pagador/recebedor — débito e variantes

A leitura continua em duas etapas: OCR local converte imagem em texto; regras determinísticas convertem rótulos/seções em campos. O chat de IA não participa automaticamente dessa extração, e estar desativado não impede o OCR. Ativar o chat, por si só, não transforma o parser em extração semântica por IA.

Falha observada: comprovante de débito tinha os nomes legíveis no texto OCR, mas usava `Estabelecimento` e `Dados do titular do cartão`, rótulos que o parser anterior não reconhecia. `document_parties.go` agora reconhece esses cabeçalhos, aliases de pagador/remetente/recebedor/beneficiário e valores na mesma linha ou em linhas seguintes; normaliza maiúsculas/acentos dos rótulos preservando nomes originais. CPF/CNPJ, NSU, agência/conta, cartão e dados da instituição encerram a captura de nome, evitando misturar identificadores e CNPJ do banco com pessoas.

O CPF mascarado não pode ser reconstruído; permanece ausente. Titular/estabelecimento extraídos do comprovante continuam separados de emitente/tomador fiscal. Nenhum nome de usuário, empresa ou estabelecimento é hardcoded nas regras.

Reenvio de bytes iguais continua abrindo o rascunho já salvo, sem sobrescrever edições. Para aplicar regras novas a dados antigos: **Reidentificar pagador e recebedor** reanalisa somente o texto salvo e completa apenas nomes vazios. POST `/v1/finance/documents/{id}/parties` com `expectedUpdatedAt`, membership e usuário/conta da sessão; versão antiga409 e recurso alheio404. Não muda tomador, documento, competência, serviço, valor ou data; correções existentes são preservadas. Alteração real de nomes atualiza a versão e exige nova revisão.

Testes: `document_parties_test.go` cobre débito, inline/multilinha, variantes, proteção de identificadores e preservação de correções/fiscal. Teste real opt-in por `FINANCE_OCR_TEST_DEBIT_IMAGE` passou com o arquivo privado enviado, sem gravar fixture no Git. Teste PostgreSQL descartável cobre recuperação do OCR salvo, conflito e escopo;12testes frontend de documentos/revisão/exportação passaram. Próxima evolução possível: fallback semântico com IA configurada, evidência por campo e confirmação de ambiguidades; ainda não implementado.

Validação UI da recuperação concluída: selecionou o rascunho antigo `itabaiana.jpeg`, conferiu nomes vazios e acionou Reidentificar. A resposta persistida preencheu titular/pagador e estabelecimento/recebedor corretos do comprovante. Nenhum nome já corrigido, documento mascarado ou dado fiscal foi sobrescrito. Reinício do web local foi necessário para o novo botão aparecer após HMR; API/web voltaram a responder. Sem commit/push/deploy.

### Correção do teste de reenvio

A correção de rótulos inicialmente só atualizava rascunhos antigos pela ação manual. Isso não cobria o teste de reenviar o mesmo arquivo: o hash devolvia os campos antigos. Agora POST de extração de bytes conhecidos reaplica identificação de partes no OCR salvo, preenchendo só nomes vazios e preservando nomes manuais/ID/demais campos. O teste PostgreSQL cobre esse caminho explicitamente, sem chamar o OCR novamente.

Havia ainda um problema de seleção repetida: após leitura, o File reativo era limpo mas o input nativo mantinha o arquivo, impedindo `change` ao escolher os mesmos bytes. O input agora é zerado junto do estado após sucesso/troca de contexto; falha mantém o File disponível para repetir a operação.

Validação do reenvio na UI concluída com `luzia minimercado.jpeg`: rascunho antigo tinha nomes vazios; selecionar o mesmo arquivo e Ler documento preencheu ambos automaticamente, preservando valor8,99 e demais dados. O seletor foi limpo após sucesso. Selecionar novamente o arquivo idêntico voltou a habilitar Ler documento, e a segunda leitura concluiu sem duplicar o rascunho. Go Finance/App com PostgreSQL descartável e5testes do composable passaram; ESLint sem erros. Typecheck global permanece com dívida anterior fora da feature.

## CPF/CNPJ por parte e formulário compacto — 2026-11-02

A identificação anterior extraía nomes, mas descartava CPF/CNPJ. Agora `payerDocument`/`payeeDocument` acompanham a parte identificada e são persistidos no JSON existente, sem migration. A recuperação de rascunho antigo (Atualizar leitura ou reenviar o arquivo) preenche somente campos vazios. Documentos mascarados, como o CPF deste comprovante, permanecem desconhecidos; o CNPJ do banco no rodapé não é atribuído a nenhuma parte.

Na interface, selecionar **Usar como tomador** copia nome e documento da parte escolhida para os campos fiscais editáveis. A escolha não implica que a transação seja prestação de serviço. Trocar para uma parte sem documento limpa o documento do tomador anterior. Competência e emitente não são inferidos. Salvar continua necessário.

O envio e suas ações ficam em uma linha responsiva; nomes/documentos são agrupados por parte. Tributação, observações e explicações ficam recolhidas. As informações de revisão e emissão indisponível continuam acessíveis.

Validação: teste OCR real de `energisaasasa.jpeg` passou com valor 98,15, data 2026-10-17, recebedor e seu CNPJ completo, pagador e CPF vazio. Nenhuma imagem privada foi adicionada ao Git. Suítes Finance e App em Go passaram; testes Vitest de documentos/revisão/exportação: 13 aprovados; ESLint dos componentes alterados passou.

Reprodução futura: abrir Finance → assistente → Enviar comprovante; enviar o pagamento Pix com origem/destino e CNPJ (ou clicar Atualizar leitura no rascunho antigo); conferir CPF/CNPJ de cada parte e ausência do CNPJ bancário. Escolher a parte correta como tomador, conferir os campos copiados, salvar e recarregar. Repetir escolhendo uma parte sem documento: o documento anterior deve ficar vazio. Testar CPF/CNPJ tanto inline quanto em linha separada e CPF mascarado.

Validação visual: formulário compacto observado no Chrome. A recuperação do CNPJ no rascunho antigo não foi concluída pela UI porque o navegador estava em uso; extração real e seleção do tomador foram verificadas por testes locais. Por orientação do usuário, testes pelo Work/navegador ficam suspensos até solicitação explícita. O typecheck global segue com erros preexistentes fora deste fluxo; não houve diagnóstico nos arquivos alterados.

Teste complementar em Postgres descartável `finance_test_party_docs`: persistência e recuperação do documento do recebedor, conflitos de versão e isolamento por conta/usuário passaram na suíte Finance. Banco de teste removido ao final.

Seletor de arquivo (2026-11-03): removido o filtro nativo `accept` para evitar bloquear a seleção por associação de formato do navegador/SO. O composable continua validando formato/extensão e 8 MB antes de enviar, e o backend valida o conteúdo real. Validação do seletor no navegador pendente, conforme preferência de não usar Work.

## Imagem de baixa resolução — 2026-11-03

O comprovante `comprovante.jpeg` tem apenas 211×1280 pixels. O OCR anterior não identificava o valor e reutilizava o texto incompleto no reenvio. A preparação agora amplia imagens pequenas em memória, respeitando os limites de dimensões/área. A leitura tolera o símbolo monetário reconhecido como AS/RS somente junto do rótulo explícito de valor. Valores divergentes continuam exigindo revisão.

O teste OCR real `TestRealSmallReceiptOCR` passou com 57,58, 2026-10-15, HOSTGATOR, nome do pagador e CNPJ do recebedor. Papéis fiscais e competência continuam sem atribuição automática. Imagem e texto privados não entram no Git.

Para atualizar uma leitura antiga, **selecionar e reenviar o mesmo arquivo** em Ler documento. Se o texto extraído anterior está incompleto, a API refaz a leitura e atualiza texto/campos vazios no mesmo rascunho, com controle de versão. Não sobrescreve campos preenchidos manualmente. O botão Atualizar leitura apenas reanalisa texto salvo; não substitui esse reenvio, porque o original não é armazenado.

Validação sem Work/navegador: teste OCR real e suíte Finance em Go. Regressão Postgres cobre reenvio de OCR incompleto, substituição do texto e preservação de valor/tomador manuais.
