-- Frente B Roadmap-B2 — link tasks <-> roadmap modules
--
-- Permite vincular uma task a um modulo do roadmap (roadmap_module_id) e
-- marcar tasks que devem aparecer no dashboard /roadmap (pinned_to_roadmap).
--
-- Default: ambas NULL/false; tasks existentes continuam sem vinculo.
-- ON DELETE SET NULL no FK: apagar modulo do roadmap nao apaga tasks vinculadas.

alter table tasks.tasks
    add column if not exists roadmap_module_id uuid references roadmap.modules(id) on delete set null,
    add column if not exists pinned_to_roadmap boolean not null default false;

create index if not exists tasks_tasks_roadmap_module_idx
    on tasks.tasks (roadmap_module_id)
    where roadmap_module_id is not null;

create index if not exists tasks_tasks_pinned_to_roadmap_idx
    on tasks.tasks (account_id, pinned_to_roadmap)
    where pinned_to_roadmap = true;
