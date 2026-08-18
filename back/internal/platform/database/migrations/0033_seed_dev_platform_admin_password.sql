-- Retired: migrations must never reset an existing credential to facilitate local testing.
-- Kept as an append-only migration marker because this version may already be recorded as applied.

select 1;
