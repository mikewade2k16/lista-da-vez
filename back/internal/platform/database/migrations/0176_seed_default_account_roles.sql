-- 0176_seed_default_account_roles.sql
--
-- Materializa os 6 papeis-padrao operacionais (o "coarse legado": owner/director/
-- marketing/manager/consultant/store_terminal) como core.roles EDITAVEIS por
-- conta-CLIENTE: cria os que faltam, marca is_default=true, corrige labels clonados
-- errados ('Supervisor de Fila') e garante as permissoes workspace.* (o MESMO mapa
-- fiel da 0175 / access.defaultRolePermissionMap). Assim o coarse vira DADO (papel
-- por account que o cliente edita/remove no painel), nao mais enum hardcoded.
--
-- Decisao do dono (2026-06-30): TODO cliente ativo (is_agency=false) recebe os 6,
-- inclusive contas sem o modulo Fila — as permissoes sao workspace.* do modulo `core`
-- (sempre habilitado), entao sao validas/editaveis no painel em qualquer conta.
--
-- ADITIVO e idempotente (rodar 2x = no-op): NAO remove papeis, NAO sobrescreve edicao
-- do cliente (ON CONFLICT DO NOTHING na criacao; label so e corrigido quando ainda e o
-- clone 'Supervisor de Fila'; permissoes via ON CONFLICT DO NOTHING). NAO trava
-- (is_locked) os papeis de Fila — o cliente pode remove-los; o safeguard de "conta sem
-- dono" e o core.owner (ja is_locked). NAO toca conta-agencia, papeis core.* nem papeis
-- de outros modulos.
--
-- O migrator roda o arquivo INTEIRO no boot (sem marcadores goose). SQL plano, schema
-- sempre qualificado (core.*).

-- ----------------------------------------------------------------------------
-- 1) Cria os 6 papeis-padrao que faltam em cada conta-cliente ativa.
--    ON CONFLICT (account_id, code) DO NOTHING: nao sobrescreve papel ja existente
--    (preserva edicoes). is_default=true; is_locked=false (removivel).
-- ----------------------------------------------------------------------------
with role_defs(code, label, description, template_id) as (
  values
    ('queue.owner',          'Proprietario da Fila', 'Responsavel pela operacao de Fila do cliente, com visao total das lojas.',          'queue.supervisor'),
    ('queue.director',       'Diretoria da Fila',    'Acompanha a operacao consolidada da Fila com leitura executiva cross-loja.',       'queue.supervisor'),
    ('queue.marketing',      'Marketing da Fila',    'Enxerga a operacao integrada das lojas, com foco em leitura e campanhas.',         'queue.consultant'),
    ('queue.manager',        'Gerente da Fila',      'Acompanha consultores e a operacao da loja sob sua responsabilidade.',             'queue.supervisor'),
    ('queue.consultant',     'Consultor',            'Acesso operacional basico para trabalhar a fila e os atendimentos da propria loja.', 'queue.consultant'),
    ('queue.store_terminal', 'Terminal de Loja',     'Acesso fixo do computador da loja para tocar a operacao da propria unidade.',      'queue.supervisor')
),
target_accounts as (
  select id from core.accounts where is_active = true and is_agency = false
)
insert into core.roles (account_id, cloned_from_template_id, code, label, description, is_default, is_locked)
select ta.id, rd.template_id, rd.code, rd.label, rd.description, true, false
from target_accounts ta
cross join role_defs rd
on conflict (account_id, code) do nothing;

-- ----------------------------------------------------------------------------
-- 2) Marca is_default=true nos 6 papeis JA existentes das contas-cliente.
-- ----------------------------------------------------------------------------
update core.roles r
set is_default = true,
    updated_at = now()
from (values
  ('queue.owner'), ('queue.director'), ('queue.marketing'),
  ('queue.manager'), ('queue.consultant'), ('queue.store_terminal')
) as d(code)
where lower(r.code) = d.code
  and r.is_default = false
  and exists (
    select 1 from core.accounts a
    where a.id = r.account_id and a.is_active = true and a.is_agency = false
  );

-- ----------------------------------------------------------------------------
-- 3) Corrige labels clonados errados ('Supervisor de Fila', vindo do enroll que
--    clonava o template queue.supervisor) para o canonico — SO onde ainda e o clone,
--    preservando renome feito pelo cliente.
-- ----------------------------------------------------------------------------
update core.roles r
set label = rd.label,
    updated_at = now()
from (values
  ('queue.owner',          'Proprietario da Fila'),
  ('queue.director',       'Diretoria da Fila'),
  ('queue.manager',        'Gerente da Fila'),
  ('queue.store_terminal', 'Terminal de Loja')
) as rd(code, label)
where lower(r.code) = rd.code
  and r.label = 'Supervisor de Fila';

-- ----------------------------------------------------------------------------
-- 4) Garante as permissoes workspace.* dos 6 papeis (MESMO mapa fiel da 0175).
--    ON CONFLICT DO NOTHING: nao toca o que ja existe; preenche so os recem-criados.
-- ----------------------------------------------------------------------------
with code_to_coarse(code, coarse) as (
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
insert into core.role_permissions (role_id, permission_key)
select r.id, ctp.permission_key
from core.roles r
join code_to_coarse ctc on lower(r.code) = ctc.code
join coarse_to_perm ctp on ctp.coarse = ctc.coarse
join core.accounts a on a.id = r.account_id and a.is_active = true and a.is_agency = false
on conflict (role_id, permission_key) do nothing;
