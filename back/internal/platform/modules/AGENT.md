# AGENT — platform/modules

## Escopo

Pacote `back/internal/platform/modules/`. Module Registry da plataforma
multi-tenant: interfaces que cada modulo plugavel implementa, Registry que
sincroniza catalogo no banco e constroi modulos com dependencias resolvidas.

Branch: `refactor/multi-tenant-core`. Documento mestre:
`~/.claude/plans/preciso-que-analise-nosso-ancient-orbit.md` secao B.

## Pecas

- `module.go` — interfaces `Module`, `Handle`; structs `Metadata`,
  `Dependencies`, `PermissionDef`, `RoleTemplateDef`. Tudo que um modulo
  declara para a plataforma.
- `registry.go` — `Registry` com `MustRegister`, `SyncCatalog` e `Build`.
  Define `CatalogRepository` (interface) e os Row structs que ele consome.
- `catalog_postgres.go` — `PostgresCatalogRepository` implementando
  `CatalogRepository`. Faz upsert em `core.modules`, `core.permissions`,
  `core.role_templates` e `core.role_template_permissions`.

## Fluxo no boot

```go
registry := modules.NewRegistry(logger)
registry.MustRegister(core.New())
// futuros: registry.MustRegister(contacts.New()), queue.New(), finance.New(), ...

if err := registry.SyncCatalog(ctx, modules.NewPostgresCatalogRepository(pool)); err != nil { ... }

handles, err := registry.Build(modules.Dependencies{
    Pool:           pool,
    Logger:         logger,
    Bus:            bus,
    AuthMiddleware: authMiddleware,
})

for _, h := range handles {
    h.RegisterRoutes(mux)
    h.RegisterEventHandlers(bus)
}
```

## Regras inegociaveis (vide CONTRACT_FREEZE.md)

`SyncCatalog`:

1. **Cria** novas permissoes e templates.
2. **Atualiza** apenas `label`, `description`, `sort_order` e dependencias de
   modulos/permissoes existentes.
3. **Marca como deprecated** (deprecated_at = now()) as keys que sumiram do
   catalogo. **NUNCA executa DELETE automatico.**
4. **NAO toca em** `core.roles`, `core.role_permissions` (pertencem a Account).
5. **NAO sobrescreve** `core.role_template_permissions` de templates ja
   existentes — templates sao versionados; para mudar permissoes de um template
   ja usado, criar template novo (id diferente).

`Registry.Build`:

- Valida `RequiresModules` antes de construir (falha se faltar).
- Ordena por `Metadata.SortOrder` para boot deterministico.
- Loga cada modulo construido com schema e is_core.

`Module.ID()`:

- Estavel para sempre — vira chave em `core.modules` e referencia em
  `core.account_modules`. Renomear quebra historico.
- Convencao: lowercase, snake-case, curto. Ex: `core`, `queue`, `finance`,
  `contacts`, `tasks`, `omni`, `site`, `bio`.

## Comportamento sem feature-flag

`CORE_V2_ENABLED=false` (default em prod): `app.go` nao constroi o Registry.
Modulos legados continuam funcionando pelo wiring antigo. Nada acontece neste
pacote.

`CORE_V2_ENABLED=true`: Registry e instanciado, SyncCatalog roda, modulos do
registry sao construidos e expostos.

## Quando atualizar este AGENT.md

- Quando trocar/adicionar campo em `Module`, `Handle`, `Dependencies`.
- Quando mudar regra do `SyncCatalog`.
- Quando registrar primeiro modulo satelite (Fase 6) — atualizar lista de
  modulos esperados.
