-- Permite consultor permanecer "orfao" (sem loja) quando a loja e deletada.
-- Motivacao: deletar uma loja nao deve apagar o cadastro dos consultores; eles
-- podem ser realocados para outra loja. Apos esta migration, store_id em
-- consultants pode ser NULL e a FK aplica ON DELETE SET NULL.

alter table queue.consultants
    alter column store_id drop not null;

alter table queue.consultants
    drop constraint if exists queue_consultants_store_fk;

alter table queue.consultants
    drop constraint if exists consultants_store_id_fkey;

alter table queue.consultants
    add constraint consultants_store_id_fkey
    foreign key (store_id) references queue.stores(id) on delete set null;
