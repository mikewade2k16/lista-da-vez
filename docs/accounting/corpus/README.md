# Corpus inicial CT-06

Duas seleções oficiais curtas e atribuídas foram conferidas em 2026-11-04:
[documentação de produção NFS-e](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual)
e [método de pesquisa da Receita Federal](https://www.gov.br/receitafederal/pt-br/acesso-a-informacao/legislacao/como-fazer-suas-pesquisas-no-sistema-normas).
O manifesto registra os hashes dos bytes selecionados, origem e limites de uso.
Não registra hash do HTML/PDF integral, porque esses originais não foram coletados.

Este corpus é informativo de navegação/metodologia; não cobre cálculo, alíquotas,
regime ou aplicabilidade de município/empresa. Vigência continua desconhecida.
Os testes de negócio usam fixtures **sintéticas**, não legislação inventada.

Carga: um usuário autenticado escolhe conta e emitente, cria a base privada na
seção Fontes, importa cada TXT com título/origem/metadados do manifesto, confere a
revisão, publica a versão e ativa a base. O arquivo permanece byte a byte no
storage privado. Não há script que invente tenant ou publique em conta real.
Sem storage disponível, importação informa indisponibilidade e não publica.

Perguntas mínimas após carga autorizada:

| Termos FTS | Trecho esperado | Resultado de aplicabilidade |
|---|---|---|
| `guias manuais` | Seleção NFS-e | Informativo, vigência não verificada |
| `versões ato` | Seleção Receita | Informativo, contexto não confirmado |
| `alíquota interestadual` | Nenhum | insufficient_evidence |

FTS simple não faz expansão semântica ou normalização linguística de pergunta
completa. Selecionar termos presentes; CT-07 deve abster-se quando não há evidência.
A base inicial não substitui a curadoria dos atos/manuais integrais específicos do
piloto, após conhecer jurisdição/regime e verificar direitos e vigência.
