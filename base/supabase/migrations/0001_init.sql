-- Base: memory (notes, links, files, to-dos) for one or more signed-in users.
-- Every row belongs to the user who made it; row-level security keeps each
-- user's rows to themselves.

create table public.items (
  id          uuid primary key default gen_random_uuid(),
  user_id     uuid not null default auth.uid() references auth.users (id) on delete cascade,
  kind        text not null check (kind in ('note', 'link', 'file', 'todo')),
  space       text not null default 'personal' check (space in ('personal', 'work')),
  title       text not null default '',
  body        text not null default '',
  url         text,
  file_path   text,
  file_type   text,
  file_size   bigint,
  due_at      timestamptz,
  done        boolean not null default false,
  -- "Keep out of AI": the assistant never reads or returns this item.
  private     boolean not null default false,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  search      tsvector generated always as (
    setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(body, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(url, '')), 'C')
  ) stored
);

create index items_search_idx on public.items using gin (search);
create index items_user_created_idx on public.items (user_id, created_at desc);
create index items_user_todo_idx on public.items (user_id, done, due_at) where kind = 'todo';

create or replace function public.touch_updated_at()
returns trigger
language plpgsql
set search_path = ''
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

create trigger items_touch_updated_at
before update on public.items
for each row execute function public.touch_updated_at();

alter table public.items enable row level security;

create policy "items: owner reads"   on public.items for select to authenticated using (user_id = (select auth.uid()));
create policy "items: owner inserts" on public.items for insert to authenticated with check (user_id = (select auth.uid()));
create policy "items: owner updates" on public.items for update to authenticated using (user_id = (select auth.uid())) with check (user_id = (select auth.uid()));
create policy "items: owner deletes" on public.items for delete to authenticated using (user_id = (select auth.uid()));

-- Ranked full-text search over the caller's own items (RLS still applies).
create or replace function public.search_items(
  q text,
  p_space text default null,
  p_kind text default null,
  p_include_private boolean default true,
  p_limit int default 20
)
returns setof public.items
language sql
stable
security invoker
set search_path = ''
as $$
  select i.*
  from public.items i
  where i.search @@ websearch_to_tsquery('english', q)
    and (p_space is null or i.space = p_space)
    and (p_kind is null or i.kind = p_kind)
    and (p_include_private or not i.private)
  order by ts_rank(i.search, websearch_to_tsquery('english', q)) desc, i.created_at desc
  limit least(greatest(p_limit, 1), 100);
$$;

-- Private bucket for uploads. Files live under <user id>/<file name>.
insert into storage.buckets (id, name, public, file_size_limit)
values ('files', 'files', false, 52428800)
on conflict (id) do nothing;

create policy "files: owner reads"   on storage.objects for select to authenticated using (bucket_id = 'files' and (storage.foldername(name))[1] = (select auth.uid())::text);
create policy "files: owner uploads" on storage.objects for insert to authenticated with check (bucket_id = 'files' and (storage.foldername(name))[1] = (select auth.uid())::text);
create policy "files: owner deletes" on storage.objects for delete to authenticated using (bucket_id = 'files' and (storage.foldername(name))[1] = (select auth.uid())::text);
