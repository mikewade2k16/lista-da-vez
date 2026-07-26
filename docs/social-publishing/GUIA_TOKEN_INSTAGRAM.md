# Guia de conexão do Instagram

Este guia explica como obter e usar o token aceito pelo módulo de Agendamento
de Postagens na rota `/postagens`.

O piloto atual usa a **Instagram API with Instagram Login** e o host
`graph.instagram.com`. Portanto, a credencial correta é um **Instagram User
Access Token** emitido para a conta profissional que será conectada.

Não use:

- senha do Instagram;
- App Secret da Meta;
- App Access Token;
- Facebook User Token;
- Facebook Page Token;
- System User Token.

Essas credenciais pertencem a outros fluxos e não atendem ao adapter atual.

## Antes de começar

São necessários:

- uma conta Instagram profissional do tipo **Business** ou **Creator**;
- acesso administrativo à conta que será conectada;
- uma conta no [Meta for Developers](https://developers.facebook.com/);
- um app Meta com o produto Instagram configurado para
  **Instagram API with Instagram Login**;
- as permissões descritas na próxima seção.

Com Instagram Login, a conta profissional não precisa estar vinculada a uma
Página do Facebook. Essa exigência pertence ao fluxo alternativo
**Instagram API with Facebook Login**, que não é usado pelo módulo atual.

Uma conta pessoal não pode ser conectada. Converta-a em Business ou Creator
antes de continuar.

## Permissões necessárias

O token precisa conceder exatamente as capacidades usadas pelo piloto:

| Permissão | Uso no módulo |
| --- | --- |
| `instagram_business_basic` | Validar o perfil, ler ID, usuário, tipo da conta e quantidade de mídias |
| `instagram_business_content_publish` | Criar o container da mídia e publicar a postagem |
| `instagram_business_manage_insights` | Coletar analytics da mídia publicada |

Use os nomes com o prefixo `instagram_business_`. Os nomes antigos, como
`business_basic` e `business_content_publish`, foram descontinuados pela Meta
em 27 de janeiro de 2025.

## Piloto manual: gerar e conectar o token

Este é o procedimento indicado somente para uma conta de teste controlada.

### 1. Preparar o app Meta

1. Abra o [Painel de Apps da Meta](https://developers.facebook.com/apps/).
2. Abra o app da plataforma ou crie um app da organização, preferencialmente
   com caso de uso Business.
3. Adicione o produto Instagram.
4. Escolha **API setup with Instagram login**.
5. Confira se o app solicita as três permissões listadas acima.

Os nomes e a posição dos itens podem mudar no painel da Meta. Use como
referência a seção oficial
[Get Started](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/get-started/).

### 2. Autorizar a conta de teste

Enquanto o app estiver em modo de desenvolvimento, a conta profissional usada
no teste precisa estar associada ao app como conta de teste/tester ou ser
administrada por uma pessoa com função no app.

1. Adicione a conta profissional na área de contas de teste ou funções do app.
2. Entre no Instagram com essa conta.
3. Aceite o convite ou a solicitação de acesso, quando a Meta apresentar essa
   etapa.
4. Volte para **API setup with Instagram login**.

Se a conta não aparecer para geração do token, normalmente falta aceitar o
convite, a conta ainda é pessoal ou o usuário autenticado não administra o
perfil.

### 3. Gerar o token

1. Na área **Generate access tokens**, escolha a conta profissional.
2. Clique em **Generate token**.
3. Entre diretamente no Instagram com a conta escolhida.
4. Autorize:
   `instagram_business_basic`,
   `instagram_business_content_publish` e
   `instagram_business_manage_insights`.
5. Copie somente o token gerado.

Não coloque o token em chat, tarefa, documento, captura de tela ou arquivo
`.env` do frontend.

### 4. Conectar no Omni

1. Acesse `/postagens`.
2. Se houver um seletor de contas, escolha primeiro o cliente da plataforma que
   será dono da conexão.
3. Abra a aba **Conexão**.
4. Cole o token no campo **Token de acesso**.
5. Clique em **Validar conexão**.
6. Confira o `@usuário`, o tipo da conta e o ID retornados.

A conexão é sempre individual. A opção **Todos os clientes** serve para visão
consolidada e não pode receber um token. Nunca reutilize o token de uma conta
Instagram em outro cliente da plataforma.

O valor é enviado uma única vez ao backend, cifrado com a chave de segredos do
Omni e não volta na resposta da API.

## Como o Omni valida a credencial

O adapter atual consulta o endpoint `/me` da versão configurada de
`graph.instagram.com` e solicita:

```text
user_id,username,account_type,media_count
```

A conexão só é aceita quando:

- o token é válido;
- o perfil possui ID e nome de usuário;
- `account_type` identifica uma conta Business ou Creator.

Depois da conexão, o módulo usa o mesmo Instagram User Access Token para:

- criar um container de imagem em `/{ig-user-id}/media`;
- publicar em `/{ig-user-id}/media_publish`;
- consultar o permalink;
- coletar métricas em `/{ig-media-id}/insights`.

A mídia informada na postagem precisa estar disponível em uma URL HTTPS pública
que os servidores da Meta consigam acessar.

## Token curto, token de longa duração e renovação

No fluxo OAuth, a autorização inicial produz um código que o backend troca por
um token de curta duração. Para agendamentos, esse token deve ser trocado no
servidor por um token de longa duração usando o fluxo oficial
`ig_exchange_token`.

Tokens de longa duração normalmente têm validade aproximada de 60 dias. A
aplicação deve usar o `expires_in` devolvido pela Meta para calcular
`expires_at`; a duração não deve ser fixada manualmente no código.

Um token de longa duração ainda válido pode ser renovado pelo fluxo oficial
`ig_refresh_token`. Se o token já estiver vencido, tiver sido revogado ou tiver
perdido uma permissão, a conta precisa passar novamente pela autorização.

Consulte sempre as referências vigentes da Meta:

- [Troca para token de longa duração](https://developers.facebook.com/docs/instagram-platform/reference/access_token/);
- [Renovação do token](https://developers.facebook.com/docs/instagram-platform/reference/refresh_access_token/).

### Limitação conhecida do piloto

O módulo atual armazena o token cifrado, mas **ainda não persiste a expiração e
não executa renovação automática**.

Consequências:

- a conexão técnica não deve ser tratada como permanente;
- postagens agendadas para depois do vencimento podem falhar;
- o operador precisa acompanhar a validade e reconectar a conta quando
  necessário;
- antes da liberação ampla, o backend deve armazenar `expires_at`, renovar o
  token com antecedência e avisar quando uma nova autorização for necessária.

## Produção: conexão por OAuth

O procedimento manual não é o onboarding final dos clientes.

Na produção:

1. a plataforma mantém um único app Meta aprovado;
2. o operador escolhe o cliente correto no Omni;
3. clica em **Conectar Instagram**;
4. o proprietário da conta entra no Instagram e concede as permissões;
5. a Meta devolve um código ao callback HTTPS do Omni;
6. o backend valida `state`, troca o código pelo token, obtém a credencial de
   longa duração e a cifra;
7. a conexão fica vinculada somente ao `account_id` selecionado.

O parâmetro `state` deve proteger contra CSRF e carregar uma referência
assinada e temporária para a conta e o usuário que iniciaram a conexão. O App
Secret e a troca do código nunca podem acontecer no navegador.

Para contas Instagram que a plataforma não possui ou administra, as permissões
precisam de **Advanced Access** e aprovação no **App Review**. Standard Access é
adequado apenas para contas próprias ou gerenciadas, adicionadas ao app para
desenvolvimento e teste.

Referências:

- [Business Login for Instagram](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/business-login/);
- [App Review](https://developers.facebook.com/docs/app-review/);
- [Instagram API oficial no Postman](https://www.postman.com/meta/instagram/overview).

## Segurança

- Trate o token como uma senha: quem o possui pode exercer as permissões
  concedidas.
- Nunca solicite que um cliente envie o token por WhatsApp, e-mail, chamado ou
  chat interno.
- Nunca registre token, código OAuth ou App Secret em logs.
- Mantenha o App Secret somente no backend.
- Use HTTPS na autorização, no callback e nas URLs de mídia.
- Não armazene a credencial no `localStorage` nem em estado persistente do
  frontend.
- Mostre no painel apenas metadados seguros, como os últimos quatro caracteres.
- Ao suspeitar de exposição, revogue a conexão e gere uma nova autorização.

O próprio usuário pode remover o acesso em Instagram:
**Configurações → Permissões de sites → Apps e sites → Ativos → Remover**.
Veja a orientação oficial em
[Apps e sites conectados ao Instagram](https://www.facebook.com/help/instagram/1144624522593085/).

## Solução de problemas

### “Token inválido” ou erro OAuth

- Confirme que foi gerado pelo fluxo **Instagram Login**.
- Não use token de Página, Facebook, app ou System User.
- Gere novamente o token entrando na conta Instagram correta.
- Verifique se ele venceu ou se o acesso do app foi removido.

### A conta não aparece no painel da Meta

- Confirme que é Business ou Creator.
- Confirme que o convite de tester foi aceito.
- Confirme que a pessoa autenticada administra a conta.
- Em produção, confirme que o app está aprovado e em modo Live.

### A conexão funciona, mas a publicação falha

- Confirme a permissão `instagram_business_content_publish`.
- Confirme que a URL da imagem é HTTPS, pública e não expira antes da
  publicação.
- Verifique se o acesso ao app foi revogado depois da conexão.
- Não tente corrigir reutilizando o token de outro perfil.

### Analytics vazio

- Confirme a permissão `instagram_business_manage_insights`.
- Aguarde a Meta processar a publicação e tente sincronizar novamente.
- Insights estão disponíveis somente para mídia pertencente à conta
  profissional autorizada.
- Algumas métricas de conta não são disponibilizadas para perfis com menos de
  100 seguidores.
- A Meta pode devolver um conjunto vazio quando a métrica não existe ou ainda
  não está disponível; vazio não deve ser convertido automaticamente em zero.

Consulte as limitações atuais na coleção oficial de
[Instagram Insights](https://www.postman.com/meta/instagram/folder/23987686-f659d7d1-d74c-44e4-9192-9b1e8694c511).

### Permissão concedida no teste, mas negada para um cliente

Standard Access não autoriza automaticamente contas de terceiros. Solicite
Advanced Access para cada permissão necessária, conclua o App Review e verifique
os requisitos de negócio e privacidade exibidos no painel da Meta.

## Referências oficiais

- [Instagram API with Instagram Login](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/);
- [Get Started](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/get-started/);
- [Business Login for Instagram](https://developers.facebook.com/docs/instagram-platform/instagram-api-with-instagram-login/business-login/);
- [Referência de access token](https://developers.facebook.com/docs/instagram-platform/reference/access_token/);
- [Referência de refresh token](https://developers.facebook.com/docs/instagram-platform/reference/refresh_access_token/);
- [Publicação de container na coleção oficial](https://www.postman.com/meta/instagram/request/23987686-299b176b-90aa-4d8a-b6cf-e6028fc69de5);
- [Insights na coleção oficial](https://www.postman.com/meta/instagram/folder/23987686-f659d7d1-d74c-44e4-9192-9b1e8694c511);
- [Workspace oficial da Instagram API](https://www.postman.com/meta/instagram/overview).
