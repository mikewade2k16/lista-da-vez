-- Roadmap-B3 -- tarefas vinculadas a um modulo do roadmap devem aparecer no /roadmap.
--
-- A migration 0116 criou dois campos separados: roadmap_module_id e pinned_to_roadmap.
-- Na pratica da UI, selecionar um modulo ja significa "mostrar essa task no roadmap".
-- Este backfill corrige tasks criadas nesse intervalo que ficaram vinculadas, mas invisiveis.

update tasks.tasks
   set pinned_to_roadmap = true,
       updated_at = now()
 where roadmap_module_id is not null
   and pinned_to_roadmap = false;
