-- PostgreSQL expands SELECT * when the view is created, so columns added to the
-- backing queue table later are not exposed automatically through the public view.
create or replace view public.operation_service_history as
    select * from queue.operation_service_history;
