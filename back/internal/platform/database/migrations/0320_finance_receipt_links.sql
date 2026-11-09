-- CT-15 pacote 6 (P15-05/P15-18): vínculo confirmado entre comprovante e lançamento.
-- Um comprovante liga no máximo um lançamento e cada lançamento no máximo um comprovante.
-- Linhas são regravadas pela edição da planilha, por isso não há FK para finance.lines:
-- a leitura revalida linha/valor e informa vínculo inválido em vez de esconder.
create table if not exists finance.receipt_links (
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    document_id uuid not null references finance.document_drafts(id) on delete cascade,
    sheet_id uuid not null references finance.sheets(id) on delete cascade,
    line_id uuid not null,
    kind text not null check (kind in ('entrada', 'saida')),
    scope text not null check (scope in ('pessoal', 'empresarial')),
    amount numeric(14, 2) not null check (amount > 0),
    created_line boolean not null,
    created_at timestamptz not null default clock_timestamp(),
    primary key (account_id, user_id, document_id),
    unique (account_id, sheet_id, line_id)
);

create index if not exists finance_receipt_links_sheet_idx
    on finance.receipt_links(account_id, sheet_id);
