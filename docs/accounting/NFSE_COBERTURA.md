# CT-08 — NFS-e: cobertura e estado da integração

Data:2026-11-04. Estado: **parcial, adapter local simulado; emissão real desativada**.
Não há primeira emissão homologada/real. [Handoff](handoffs/CT-08.md).

## Direção do produto e piloto

O usuário confirmou MEI e empresas não MEI, inicialmente em Sergipe. Não escolheu
município/estabelecimento específico nem regime das empresas não MEI. Estar acima
do limite do MEI não confirma Simples Nacional, Lucro Presumido ou Lucro Real.

A arquitetura não fixa Sergipe, município ou MEI como única modalidade. O registry
comum admite adapters/versionamentos distintos. Cada estabelecimento precisa de
identidade, município, regime/vigência e cobertura confirmados antes de habilitar
emissão. Não se presume cobertura universal de prefeituras pela API nacional.
Preparação, diagnóstico e consulta financeira continuam disponíveis sem emissor.
O recorte local usa CPF/CNPJ numéricos e titularidade direta; outros formatos ou
representação por terceiro precisam de qualificação específica, não defaults.

## Escolha técnica inicial

| Caminho | Evidência consultada | Acesso / operações | Custo e decisão |
|---|---|---|---|
| API nacional SEFIN | [Manual oficial](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/manual-contribuintes-emissor-publico-api-sistema-nacional-nfs-e-v1-2-out2025.pdf) e [ambientes oficiais](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/apis-prod-restrita-e-producao) | Recebe DPS; consulta por identificador e chave. Consulta privada vinculada ao certificado/ator. Login gov.br/web não demonstra autorização API. | Caminho escolhido somente para o prototype local. Cobertura e credenciamento do piloto não confirmados. Não se declarou custo zero. |
| Provedor Focus NFe | [Introdução](https://doc.focusnfe.com.br/reference/introducao), [autenticação](https://doc.focusnfe.com.br/reference/autenticacao), [NFS-e Nacional](https://doc.focusnfe.com.br/reference/nfse-nacional) | Documenta JSON, assinatura/comunicação pelo serviço e token por empresa via Basic. Distingue nacional de integração municipal. | Alternativa a avaliar após escolher município/requisitos. Plano, preço e cobertura contratual não verificados; não contratado/implementado. Fornecedor pago exige autorização. |
| API municipal específica | Município ainda não selecionado | Exige documentação/cadastro próprios; sem adapter aplicável confirmado. | Decisão pendente, sem inferir prestador ou preço. |
| Automação genérica de portal | Não selecionada | Sessão pessoal Finance existente é preservada e não vira certificado/autorização empresarial. | Sem bypass de MFA/CAPTCHA, scraping autenticado ou navegador. |

A escolha definitiva da integração do piloto continua condicionada à cobertura.
Não habilitar um fornecedor por esta tabela nem executar comandos de publicação.

## Referência e lacunas oficiais

- [Documentação atual](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual)
  consultada em2026-11-04: lista pacote `NFSe-ESQUEMAS_XSD-v1.01-20260209`.
  A página informa atualização em31/10/2026; isso não confirma por si a versão
  efetivamente exigida para cada operação/estabelecimento.
- O arquivo do manual tem `v1-2-out2025` no nome, mas a extração consultada mostra
  histórico interno1.0/17-03-2025. Divergência registrada, não normalizada silenciosamente.
- Rotas de envio/consulta foram conferidas no manual. JSON de transporte, composição
  exata do parâmetro de DPS, códigos/regras e fixture XML do prototype ainda precisam
  de conferência com Swagger e XSD oficiais completos. O DTO local não está homologado.
- Download direto do ZIP XSD e PDF retornou403. O leitor web consultou texto do PDF,
  mas não forneceu binário ZIP; Swagger não ficou acessível. Não houve contorno da
  proteção, mirror não oficial ou reclassificação de fixture sintética como oficial.
- [Manifesto da coleta](nfse/fontes.json): hashes dos originais não obtidos são null.
  O teste de XSD executado usa **schema sintético**, não comprova conformidade fiscal.

## O que foi implementado

Adapter `accounting/connectors/nfse` implementa a porta CT-03 sem worker, fila ou
permissão fiscal concorrentes. Valida escopo/config/certificado/manifesto, trata XML
como dado, envia uma única vez e consulta DPS/NFS-e para reconciliação. Somente chave
coerente com XML, identidade/documento/valor/competência e validadores obrigatórios
permite resultado authorized. HTTP200 sozinho não autoriza;404/timeout/5xx permanecem
incertos. Não ocorre POST automático após ausência em consulta.

O transporte usa HTTPS fixo por ambiente, mTLS, verificação TLS, sem redirect e com
limites de bytes/tempo. O certificado fica só na porta do cofre/memória; a expiração
e identidade retornadas pelo provider são conferidas. Isso não implementa/revalida
sozinho cadeia ICP-Brasil, revogação, representação ou assinatura XMLDSig.

Manifesto imutável de preparação precisa existir **antes da autorização**: conta,
emitente, documento, versões de conteúdo/perfil/config, identificação DPS, hash do
original e referência privada. Seu producer canônico ainda não foi integrado. O
adapter não inventa sequência DPS pelo pagamento/UUID nem guarda XML em nova tabela.
O motor mantém seu remote_identity UUID local; não se afirma que ele seja o ID DPS
oficial. Consulta usa o ID DPS do manifesto persistido e vinculado ao snapshot.

`SchemaValidator` executa xmllint offline com pacote revisado/hash de todos os XSD,
sem entidades/imports externos, timeout e verificação de alterações. Não acompanha
pacote oficial nem instala dependência no runtime. Antes de habilitar, qualificar
schema, assinatura e disponibilizar o validador no ambiente da API.

O produto continua **sem registrar o connector**: configuração/preparação assinada,
cofre de certificado e validadores oficiais são portas obrigatórias não ligadas.
Apenas os testes usam manifests/certificados/HTTP/schema/signature verifier sintéticos.
Não há implementação falsa dessas portas registrada em produção.

## Ambientes e artefatos

- Homologação externa precisa de autorização específica, configuração revisada e
  HomologationAuthorized. Produção exige homologação comprovada + aprovação separada.
  Todos começam indisponíveis; uma flag de UI/intenção não cria autorização fiscal.
- Fixture local usa fake_local e resultados simulated=true. Não confundir com nota
  autorizada na SEFIN. Nenhuma chamada autenticada a serviço fiscal foi realizada.
- XML validado retorna intacto ao Storage canônico pelo motor. O adapter fornece um
  recibo técnico JSON, **não um protocolo fiscal inventado**. DANFSe/PDF não foi
  implementado nem simulado como documento oficial.
- Download `/v1/accounting/issuers/{issuer}/executions/{id}/artifacts/{artifact}`
  exige RBAC+grant, conta/emitente/execution/artifact, confere tamanho/hash e revalida
  acesso após leitura. Body privado, attachment, no-store, sem exposição de storage key.

## Pré-requisitos para sair do estado parcial

1. Selecionar estabelecimento/município/regime e confirmar o caminho nacional ou
   municipal/provedor aplicável — para MEI e não MEI conforme cada caso.
2. Obter pacote oficial e Swagger acessíveis; conferir hashes, versão, campos/códigos,
   identificador DPS, certificado/representação e regras de negócio vigentes.
3. Integrar preparação assinada/manifesto imutável ao documento/revisão canônicos e
   cofre empresarial. Assinatura, cadeia/revogação e credenciamento precisam ser
   validados; não reutilizar cookie/senha pessoal nem criar SDK/cofre paralelo.
4. Repetir fixtures contra XSD/assinatura oficiais e autorizar homologação externa.
   Só então qualificar o cenário M2 no CT-10 e considerar registro do conector.
