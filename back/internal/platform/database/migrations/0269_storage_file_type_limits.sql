-- Limites autoritativos por categoria de arquivo para o storage R2.
-- max_object_bytes permanece como teto tecnico compativel e acompanha o maior
-- limite especifico; a validacao de negocio usa a categoria do MIME detectado.

alter table storage.settings
    add column if not exists image_max_bytes bigint,
    add column if not exists video_max_bytes bigint,
    add column if not exists audio_max_bytes bigint,
    add column if not exists document_max_bytes bigint;

update storage.settings
set image_max_bytes = coalesce(image_max_bytes, least(max_object_bytes, 26214400)),
    video_max_bytes = coalesce(video_max_bytes, max_object_bytes),
    audio_max_bytes = coalesce(audio_max_bytes, least(max_object_bytes, 26214400)),
    document_max_bytes = coalesce(document_max_bytes, least(max_object_bytes, 26214400));

alter table storage.settings
    alter column image_max_bytes set default 26214400,
    alter column image_max_bytes set not null,
    alter column video_max_bytes set default 26214400,
    alter column video_max_bytes set not null,
    alter column audio_max_bytes set default 26214400,
    alter column audio_max_bytes set not null,
    alter column document_max_bytes set default 26214400,
    alter column document_max_bytes set not null;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'storage_settings_file_type_limits_check'
          and conrelid = 'storage.settings'::regclass
    ) then
        alter table storage.settings
            add constraint storage_settings_file_type_limits_check check (
                image_max_bytes > 0 and image_max_bytes <= 536870912 and
                video_max_bytes > 0 and video_max_bytes <= 536870912 and
                audio_max_bytes > 0 and audio_max_bytes <= 536870912 and
                document_max_bytes > 0 and document_max_bytes <= 536870912 and
                greatest(image_max_bytes, video_max_bytes, audio_max_bytes, document_max_bytes)
                    <= storage_limit_bytes
            );
    end if;
end $$;

comment on column storage.settings.image_max_bytes is 'Teto por imagem detectada, em bytes.';
comment on column storage.settings.video_max_bytes is 'Teto por video detectado, em bytes.';
comment on column storage.settings.audio_max_bytes is 'Teto por audio detectado, em bytes.';
comment on column storage.settings.document_max_bytes is 'Teto por documento permitido, em bytes.';
