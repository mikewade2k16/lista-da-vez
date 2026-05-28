alter table if exists user_feedback
	add column if not exists closed_at timestamptz;

create index if not exists user_feedback_closed_at_idx on user_feedback (closed_at desc);

alter table if exists feedback_messages
	add column if not exists image_path text not null default '',
	add column if not exists image_content_type text not null default '',
	add column if not exists image_size_bytes integer not null default 0,
	add column if not exists image_expires_at timestamptz;

create index if not exists feedback_messages_image_expires_at_idx
	on feedback_messages (image_expires_at)
	where image_path <> '' and image_expires_at is not null;