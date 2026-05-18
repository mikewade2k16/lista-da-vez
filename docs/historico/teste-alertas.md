# 🧪 Guia de Teste: Sistema de Alertas Dinâmicos

## ✅ Estrutura Pronta (100% implementada)

- ✅ Backend: 3 migrations, model, service, HTTP endpoints
- ✅ Frontend: Store, workspace, componentes de display
- ✅ Documentação: AGENT.md (módulos) + AGENTS.md (componentes)
- ✅ Compilação: `go build ./...` sem erros

## 🚀 Para Ativar e Testar (5 minutos)

### 1️⃣ Executar Migrations

```bash
cd back
go run ./cmd/migrate/main.go up
```

**Resultado esperado:**
- 3 migrations aplicadas: `0046_alert_rule_definitions`, `0047_alert_instances_display_snapshot`, `0048_alert_instances_consultant_name`
- Backfill automático de regra padrão `long_open_service` para cada tenant

### 2️⃣ Iniciar Backend

```bash
# No terminal back
go run ./cmd/main.go
```

**Ou compilar primeiro:**
```bash
go build -o fila-backend ./cmd/main.go
./fila-backend
```

### 3️⃣ Iniciar Frontend

```bash
# Em outro terminal
cd web
npm run dev
```

### 4️⃣ Acessar Página de Alertas

```
http://localhost:3000/alertas (ou a porta do seu Nuxt)
```

## 📋 Smoke Test Manual

### Cenário 1: Criar Regra de Banner
1. Clique "+ Nova regra"
2. Preencha:
   - **Nome**: "Atendimento muito longo"
   - **Gatilho**: "Atendimento longo"
   - **Limite**: 5 minutos
   - **Display**: Banner
   - **Cor**: Vermelho
   - **Título**: "⚠️ {consultant} em atendimento há {elapsed}"
   - **Corpo**: "Faça um check-in!"
   - **Interação**: Confirmação
   - **Opções**: "Ainda está acontecendo" | "Esqueci de fechar"
3. Clique "Salvar"
4. **Esperado**: Regra aparece na tabela; status "Ativa"

### Cenário 2: Aplicar Agora (Retroatividade)
1. Clique ⚡ (botão "Aplicar agora") da regra
2. **Esperado**: Toast mostrando quantos alertas foram gerados
3. Vá para página `/operacao`
4. **Esperado**: Vê banner vermelho no topo se há atendimentos > 5min

### Cenário 3: Editar Regra
1. Clique ✎ (editar) da regra
2. Mude **Cor** para "Azul"
3. Mude **Display** para "Modal centralizado"
4. Clique "Salvar"
5. **Esperado**: Regra atualiza na tabela; próximos alertas usam nova config

### Cenário 4: Outros Display Types
- **Corner Popup**: Flutua no canto inferior direito (não-bloqueante)
- **Center Modal**: Modal centralizado bloqueante com backdrop
- **Fullscreen**: Tela inteira com gradiente (máxima agressividade)

Crie uma regra para cada tipo e veja a diferença visual!

### Cenário 5: Responder a Alerta
1. Na operação, um alerta está ativo (banner/popup/modal)
2. Clique em uma das opções de resposta
3. **Esperado**: POST `/v1/alerts/{id}/respond` é feito; alerta some de todos os displays

## 🔍 Verificações Extras

### Backend Logs
Procure por:
- `POST /v1/alerts/rules` → 201 Created
- `POST /v1/alerts/rules/{id}/apply-now` → scanner rodou
- `POST /v1/alerts/{id}/respond` → resposta persistida

### Frontend Console
- Não há erros TypeScript (vérser `npm run build`)
- Alertas carregam sem erro de API

### Database
```sql
-- Verificar regras criadas
SELECT name, trigger_type, display_kind, is_active FROM alert_rule_definitions;

-- Verificar alertas criados
SELECT headline, display_kind, status FROM alert_instances ORDER BY created_at DESC;
```

## 🎨 Variáveis de Template Suportadas

- `{consultant}` → nome do consultor ou "Consultor"
- `{elapsed}` → tempo decorrido formatado (ex: "1h5m")
- `{threshold}` → valor do threshold em minutos
- `{store}` → nome da loja (reservado para futuro)

**Exemplo de template:**
```
Título: "Alerta! {consultant} já está há {elapsed}"
Corpo: "Limite: {threshold} minutos"
```

**Resultado:**
```
Alerta! João já está há 1h5m
Limite: 25 minutos
```

## ✅ Tudo OK?

Se chegou até aqui:
- ✅ Backend compila sem erros
- ✅ Migrations executadas
- ✅ Regras criáveis e persistem
- ✅ Alertas aparecem no display escolhido
- ✅ Resposta funciona
- ✅ Histórico carrega

**Próximos passos opcionais:**
1. Testes unitários para CRUD
2. Testes de integração para retroatividade
3. Testes de E2E (Cypress/Playwright)
4. Performance: 100+ regras simultâneas
5. Acessibilidade: keyboard navigation, screen readers

---

**Tempo estimado de setup completo**: ~10 minutos (migrations + build + primeiras regras)

Qualquer dúvida, check os AGENT.md dos módulos ou AGENTS.md dos componentes! 🚀
