import type { SupabaseClient } from "@supabase/supabase-js";

export const KINDS = ["note", "link", "file", "todo"] as const;
export const SPACES = ["personal", "work"] as const;
export type Kind = (typeof KINDS)[number];
export type Space = (typeof SPACES)[number];

export type Item = {
  id: string;
  kind: Kind;
  space: Space;
  title: string;
  body: string;
  url: string | null;
  file_path: string | null;
  file_type: string | null;
  file_size: number | null;
  due_at: string | null;
  done: boolean;
  private: boolean;
  created_at: string;
  updated_at: string;
};

const COLUMNS =
  "id, kind, space, title, body, url, file_path, file_type, file_size, due_at, done, private, created_at, updated_at";

export type ItemFilter = {
  q?: string;
  space?: Space;
  kind?: Kind;
  /** False when the assistant is asking: private items are never returned. */
  includePrivate?: boolean;
  limit?: number;
};

export async function findItems(db: SupabaseClient, f: ItemFilter = {}): Promise<Item[]> {
  const includePrivate = f.includePrivate ?? true;
  const limit = Math.min(Math.max(f.limit ?? 50, 1), 100);
  const q = f.q?.trim();

  if (q) {
    const { data, error } = await db.rpc("search_items", {
      q,
      p_space: f.space ?? null,
      p_kind: f.kind ?? null,
      p_include_private: includePrivate,
      p_limit: limit,
    });
    if (error) throw new Error(error.message);
    return (data ?? []) as Item[];
  }

  let query = db.from("items").select(COLUMNS).order("created_at", { ascending: false }).limit(limit);
  if (f.space) query = query.eq("space", f.space);
  if (f.kind) query = query.eq("kind", f.kind);
  if (!includePrivate) query = query.eq("private", false);
  const { data, error } = await query;
  if (error) throw new Error(error.message);
  return (data ?? []) as Item[];
}

export async function openTodos(
  db: SupabaseClient,
  opts: { space?: Space; includePrivate?: boolean; limit?: number } = {},
): Promise<Item[]> {
  let query = db
    .from("items")
    .select(COLUMNS)
    .eq("kind", "todo")
    .eq("done", false)
    .order("due_at", { ascending: true, nullsFirst: false })
    .order("created_at", { ascending: false })
    .limit(opts.limit ?? 50);
  if (opts.space) query = query.eq("space", opts.space);
  if (opts.includePrivate === false) query = query.eq("private", false);
  const { data, error } = await query;
  if (error) throw new Error(error.message);
  return (data ?? []) as Item[];
}

export type NewItem = {
  kind: Kind;
  space?: Space;
  title?: string;
  body?: string;
  url?: string | null;
  due_at?: string | null;
  private?: boolean;
  file_path?: string | null;
  file_type?: string | null;
  file_size?: number | null;
};

export async function insertItem(db: SupabaseClient, item: NewItem): Promise<Item> {
  const { data, error } = await db.from("items").insert(item).select(COLUMNS).single();
  if (error) throw new Error(error.message);
  return data as Item;
}

export async function updateItem(
  db: SupabaseClient,
  id: string,
  patch: Partial<Pick<Item, "title" | "body" | "url" | "due_at" | "done" | "space" | "private">>,
  opts: { includePrivate?: boolean } = {},
): Promise<Item | null> {
  let query = db.from("items").update(patch).eq("id", id);
  if (opts.includePrivate === false) query = query.eq("private", false);
  const { data, error } = await query.select(COLUMNS).maybeSingle();
  if (error) throw new Error(error.message);
  return (data as Item) ?? null;
}

/** Best-effort title for a link: the page's <title>, else the host name. */
export async function titleForUrl(url: string): Promise<string> {
  let host = url;
  try {
    host = new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url;
  }
  try {
    const res = await fetch(url, {
      signal: AbortSignal.timeout(4000),
      headers: { "user-agent": "Mozilla/5.0 (compatible; Base/0.1)" },
      redirect: "follow",
    });
    const type = res.headers.get("content-type") ?? "";
    if (!res.ok || !type.includes("text/html")) return host;
    const html = (await res.text()).slice(0, 200_000);
    const m = html.match(/<title[^>]*>([^<]{1,300})<\/title>/i);
    const title = m?.[1]
      ?.replace(/&amp;/g, "&")
      .replace(/&#39;|&apos;/g, "'")
      .replace(/&quot;/g, '"')
      .replace(/\s+/g, " ")
      .trim();
    return title || host;
  } catch {
    return host;
  }
}
