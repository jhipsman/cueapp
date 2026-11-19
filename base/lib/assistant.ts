import { betaZodTool } from "@anthropic-ai/sdk/helpers/beta/zod";
import type { SupabaseClient } from "@supabase/supabase-js";
import { z } from "zod";
import { findItems, insertItem, openTodos, titleForUrl, updateItem, type Item } from "./items";

export const MODEL = "claude-opus-5-5";

// Kept byte-for-byte stable so it caches. Anything that changes per request
// (the time, the user's time zone) goes in a system message at the end.
export const SYSTEM_PROMPT = `You are Base, a personal assistant that lives inside the user's own app. Base holds their memory: notes, links, uploaded files and to-dos, each in a "personal" or "work" space.

How to help:
- Look before you answer. When a question could touch anything they've saved (people, plans, projects, "what did I…", "when is…"), search memory first, and say where an answer came from.
- Save when asked, and when they clearly hand you something to keep ("remind me…", "note that…", "I need to…"). A to-do with a time gets a due_at. Put work things in the work space.
- Keep replies short and plain, like a capable friend texting back. Lists only when there are several items. No preamble.
- Dates: resolve "tomorrow", "Friday", "next week" against the current time you are given, in the user's time zone, and write due_at as an ISO 8601 timestamp with the offset.
- Some items are marked private and are hidden from you. If something seems missing, that may be why; don't guess at it.
- Use web search only for things outside their memory that need current information.`;

/** Fields the model sees for an item: no file paths or internal flags. */
function view(item: Item, bodyChars = 600) {
  return {
    id: item.id,
    kind: item.kind,
    space: item.space,
    title: item.title,
    ...(item.body ? { body: item.body.length > bodyChars ? item.body.slice(0, bodyChars) + "…" : item.body } : {}),
    ...(item.url ? { url: item.url } : {}),
    ...(item.file_type ? { file_type: item.file_type } : {}),
    ...(item.kind === "todo" ? { done: item.done, due_at: item.due_at } : {}),
    created_at: item.created_at,
  };
}

const json = (v: unknown) => JSON.stringify(v);
const space = z.enum(["personal", "work"]);
const kind = z.enum(["note", "link", "file", "todo"]);

/**
 * Tools bound to the signed-in user's database client, so row-level security
 * applies to every call. Private items are filtered out of every read and write.
 */
export function makeTools(db: SupabaseClient) {
  return [
    betaZodTool({
      name: "search_memory",
      description:
        "Full-text search over the user's notes, links, files and to-dos. Use short keyword queries; quotes and OR work. Returns the best matches with ids.",
      inputSchema: z.object({
        query: z.string().min(1).describe("Keywords, e.g. 'dentist' or '\"Q3 plan\" hiring'"),
        space: space.optional().describe("Limit to one space"),
        kind: kind.optional().describe("Limit to one kind"),
      }),
      eager_input_streaming: true,
      run: async ({ query, space, kind }) => {
        const items = await findItems(db, { q: query, space, kind, includePrivate: false, limit: 10 });
        return json(items.length ? items.map((i) => view(i)) : { results: [], note: "No matches." });
      },
    }),

    betaZodTool({
      name: "read_item",
      description: "Read one item in full, e.g. a long note or an uploaded file's text.",
      inputSchema: z.object({ id: z.string().uuid() }),
      eager_input_streaming: true,
      run: async ({ id }) => {
        const { data } = await db.from("items").select("*").eq("id", id).eq("private", false).maybeSingle();
        return data ? json(view(data as Item, 40_000)) : json({ error: "Not found." });
      },
    }),

    betaZodTool({
      name: "recent_items",
      description: "The most recently saved items, newest first.",
      inputSchema: z.object({
        space: space.optional(),
        kind: kind.optional(),
        limit: z.number().int().min(1).max(30).optional(),
      }),
      eager_input_streaming: true,
      run: async ({ space, kind, limit }) => {
        const items = await findItems(db, { space, kind, includePrivate: false, limit: limit ?? 10 });
        return json(items.map((i) => view(i, 200)));
      },
    }),

    betaZodTool({
      name: "list_todos",
      description: "Open (not done) to-dos, soonest due first, then undated ones.",
      inputSchema: z.object({ space: space.optional() }),
      eager_input_streaming: true,
      run: async ({ space }) => {
        const items = await openTodos(db, { space, includePrivate: false, limit: 50 });
        return json(items.map((i) => view(i, 200)));
      },
    }),

    betaZodTool({
      name: "save",
      description:
        "Save a note, link or to-do. For a link, give the url (title is looked up if missing). For a to-do, title is the task.",
      inputSchema: z.object({
        kind: z.enum(["note", "link", "todo"]),
        space: space.describe("work for anything about their job"),
        title: z.string().max(500).optional(),
        body: z.string().max(20_000).optional(),
        url: z.string().url().optional(),
        due_at: z.string().datetime({ offset: true }).optional().describe("To-dos only. ISO 8601 with offset"),
      }),
      eager_input_streaming: true,
      run: async (input) => {
        if (input.kind === "link" && !input.url) return json({ error: "A link needs a url." });
        const title = input.title ?? (input.url ? await titleForUrl(input.url) : "");
        const item = await insertItem(db, {
          kind: input.kind,
          space: input.space,
          title,
          body: input.body ?? "",
          url: input.url ?? null,
          due_at: input.kind === "todo" ? (input.due_at ?? null) : null,
        });
        return json({ saved: view(item) });
      },
    }),

    betaZodTool({
      name: "update_item",
      description: "Change an item: mark a to-do done or not done, reschedule it, retitle, or edit a note's text.",
      inputSchema: z.object({
        id: z.string().uuid(),
        done: z.boolean().optional(),
        due_at: z.string().datetime({ offset: true }).nullable().optional(),
        title: z.string().max(500).optional(),
        body: z.string().max(20_000).optional(),
        space: space.optional(),
      }),
      eager_input_streaming: true,
      run: async ({ id, ...patch }) => {
        const item = await updateItem(db, id, patch, { includePrivate: false });
        return item ? json({ updated: view(item) }) : json({ error: "Not found." });
      },
    }),
  ];
}

/** Per-request context, sent as a trailing system message. */
export function nowMessage(timeZone: string) {
  let tz = "UTC";
  try {
    new Intl.DateTimeFormat("en-US", { timeZone });
    tz = timeZone;
  } catch {}
  const now = new Date();
  const local = new Intl.DateTimeFormat("en-US", {
    timeZone: tz,
    dateStyle: "full",
    timeStyle: "short",
  }).format(now);
  return `Current time: ${local} (${tz}). UTC: ${now.toISOString()}.`;
}
