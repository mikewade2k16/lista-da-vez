-- Grupos nunca devem carregar o pushName do participante como se fosse o nome do grupo.
-- Os registros legados foram gravados como direct_message antes da identificação @g.us;
-- o nome oficial passa a ser preenchido pelo GroupMetadataProvider do canal.
UPDATE messaging.conversations
SET contact_name = NULL,
    contact_phone = NULL,
    contact_id = NULL,
    extracted_fields = jsonb_set(
      COALESCE(extracted_fields, '{}'::jsonb),
      '{source_kind}',
      '"group_message"'::jsonb,
      true
    ),
    updated_at = now()
WHERE lower(trim(external_id)) LIKE '%@g.us'
  AND (
    extracted_fields IS NULL
    OR extracted_fields->>'source_kind' IS DISTINCT FROM 'group_message'
  );
