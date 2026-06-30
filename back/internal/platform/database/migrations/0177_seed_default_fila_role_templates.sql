-- 0177_seed_default_fila_role_templates.sql
--
-- Seeda os 6 papeis-padrao operacionais da Fila como TEMPLATES GLOBAIS editaveis
-- (core.role_templates + core.role_template_permissions), is_system=false,
-- module_id='core', is_locked=false, sort_order=200. Antes desta migration esses
-- papeis so existiam como core.roles POR CONTA (criados pela 0176). Agora viram
-- catalogo: aparecem no novo painel de "papeis-padrao gerenciaveis" e TODA conta
-- nova os clona via cloneRoleTemplates (admin_repository.go) ao ser criada.
--
-- Pareia com a 0176: os mesmos 6 codigos e EXATAMENTE o mesmo mapa de permissoes
-- workspace.* por papel (coarse_to_perm copiado fiel). A 0176 materializou nas
-- contas existentes; esta 0177 materializa no MOLDE para as proximas contas.
--
-- IDEMPOTENTE e ADITIVO (rodar 2x = no-op):
--   * core.role_templates: ON CONFLICT (id) DO NOTHING — NAO sobrescreve template
--     ja existente. 'queue.consultant' ja e template de modulo (is_system=true)
--     vindo de module.RoleTemplates/SyncCatalog; o ON CONFLICT o PROTEGE (nao
--     reescreve is_system para false). 'queue.supervisor' (se existir) idem.
--     So criamos owner/director/marketing/manager/store_terminal (e consultant
--     so se ainda nao existir).
--   * core.role_template_permissions: ON CONFLICT DO NOTHING — preenche so o que
--     falta, nunca remove permissao existente.
--
-- O migrator roda o arquivo INTEIRO no boot (SEM marcadores goose). SQL plano,
-- schema sempre qualificado (core.*). FK module_id -> core.modules(id): o modulo
-- 'core' ja existe (migration 0175 garante o insert idempotente antes deste boot).

-- ----------------------------------------------------------------------------
-- 1) Templates: os 6 papeis-padrao da Fila como catalogo GLOBAL editavel.
--    is_system=false (gerenciaveis no painel), module_id='core', sort_order=200.
--    ON CONFLICT (id) DO NOTHING preserva o que ja existe (inclusive os de
--    sistema queue.consultant/queue.supervisor — NAO os rebaixa).
-- ----------------------------------------------------------------------------
insert into core.role_templates (id, module_id, label, description, is_system, is_locked, sort_order)
values
  ('queue.owner',          'core', 'Proprietario da Fila', 'Responsavel pela operacao de Fila do cliente, com visao total das lojas.',          false, false, 200),
  ('queue.director',       'core', 'Diretoria da Fila',    'Acompanha a operacao consolidada da Fila com leitura executiva cross-loja.',       false, false, 200),
  ('queue.marketing',      'core', 'Marketing da Fila',    'Enxerga a operacao integrada das lojas, com foco em leitura e campanhas.',         false, false, 200),
  ('queue.manager',        'core', 'Gerente da Fila',      'Acompanha consultores e a operacao da loja sob sua responsabilidade.',             false, false, 200),
  ('queue.consultant',     'core', 'Consultor',            'Acesso operacional basico para trabalhar a fila e os atendimentos da propria loja.', false, false, 200),
  ('queue.store_terminal', 'core', 'Terminal de Loja',     'Acesso fixo do computador da loja para tocar a operacao da propria unidade.',      false, false, 200)
on conflict (id) do nothing;

-- ----------------------------------------------------------------------------
-- 2) Permissoes workspace.* por papel (MESMO mapa fiel da 0176).
--    code_to_coarse mapeia o id do template -> o "coarse" (perfil) e
--    coarse_to_perm e o conjunto de permissoes de cada perfil. ON CONFLICT DO
--    NOTHING preenche so o que falta (nao remove perms ja seedadas por modulo
--    em templates de sistema, ex.: queue.consultant).
-- ----------------------------------------------------------------------------
with code_to_coarse(template_id, coarse) as (
  values
    ('queue.owner', 'owner'), ('queue.director', 'director'), ('queue.marketing', 'marketing'),
    ('queue.manager', 'manager'), ('queue.consultant', 'consultant'), ('queue.store_terminal', 'store_terminal')
),
coarse_to_perm(coarse, permission_key) as (
  values
    ('consultant', 'workspace.operacao.view'), ('consultant', 'workspace.operacao.edit'),
    ('store_terminal', 'workspace.operacao.view'), ('store_terminal', 'workspace.operacao.edit'),
    ('store_terminal', 'workspace.consultor.view'), ('store_terminal', 'workspace.ranking.view'),
    ('store_terminal', 'workspace.dados.view'), ('store_terminal', 'workspace.inteligencia.view'),
    ('store_terminal', 'workspace.relatorios.view'), ('store_terminal', 'workspace.alertas.view'),
    ('manager', 'workspace.operacao.view'), ('manager', 'workspace.operacao.edit'),
    ('manager', 'workspace.alertas.view'), ('manager', 'workspace.erp.view'),
    ('manager', 'workspace.feedback.view'), ('manager', 'workspace.feedback.edit'),
    ('manager', 'workspace.multiloja.view'),
    ('marketing', 'workspace.operacao.view'), ('marketing', 'workspace.erp.view'),
    ('marketing', 'workspace.multiloja.view'), ('marketing', 'workspace.campanhas.view'),
    ('marketing', 'workspace.campanhas.edit'),
    ('director', 'workspace.operacao.view'), ('director', 'workspace.erp.view'),
    ('director', 'workspace.multiloja.view'),
    ('owner', 'workspace.operacao.view'), ('owner', 'workspace.operacao.edit'),
    ('owner', 'workspace.consultor.view'), ('owner', 'workspace.ranking.view'),
    ('owner', 'workspace.dados.view'), ('owner', 'workspace.inteligencia.view'),
    ('owner', 'workspace.relatorios.view'), ('owner', 'workspace.campanhas.view'),
    ('owner', 'workspace.campanhas.edit'), ('owner', 'workspace.clientes.view'),
    ('owner', 'workspace.clientes.edit'), ('owner', 'workspace.multiloja.view'),
    ('owner', 'workspace.multiloja.edit'), ('owner', 'workspace.usuarios.view'),
    ('owner', 'workspace.usuarios.edit'), ('owner', 'workspace.manage.view'),
    ('owner', 'workspace.configuracoes.view'), ('owner', 'workspace.configuracoes.edit'),
    ('owner', 'workspace.themes.view'), ('owner', 'workspace.alertas.view'),
    ('owner', 'workspace.alertas.edit'), ('owner', 'workspace.feedback.view'),
    ('owner', 'workspace.feedback.edit'), ('owner', 'workspace.tools.view'),
    ('owner', 'workspace.erp.view'), ('owner', 'workspace.erp.edit')
)
insert into core.role_template_permissions (role_template_id, permission_key)
select ctc.template_id, ctp.permission_key
from code_to_coarse ctc
join coarse_to_perm ctp on ctp.coarse = ctc.coarse
join core.role_templates rt on rt.id = ctc.template_id
join core.permissions p on p.key = ctp.permission_key and p.deprecated_at is null
on conflict (role_template_id, permission_key) do nothing;
