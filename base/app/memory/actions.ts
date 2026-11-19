"use server";

import { revalidatePath } from "next/cache";
import { getUser } from "@/lib/supabase/server";
import { insertItem, titleForUrl, updateItem, SPACES, type Space } from "@/lib/items";

async function db() {
  const { supabase, user } = await getUser();
  if (!user) throw new Error("Sign in first.");
  return { supabase, user };
}

function spaceOf(v: FormDataEntryValue | null): Space {
  return SPACES.includes(v as Space) ? (v as Space) : "personal";
}

function refresh() {
  revalidatePath("/");
  revalidatePath("/memory");
}

/** Quick capture: a URL becomes a link, anything else a note. */
export async function capture(form: FormData) {
  const { supabase } = await db();
  const text = String(form.get("text") ?? "").trim();
  if (!text) return;
  const space = spaceOf(form.get("space"));
  const asTodo = form.get("kind") === "todo";
  const isPrivate = form.get("private") === "on";

  if (asTodo) {
    await insertItem(supabase, { kind: "todo", space, title: text.slice(0, 500), private: isPrivate });
  } else if (/^https?:\/\/\S+$/i.test(text)) {
    await insertItem(supabase, { kind: "link", space, url: text, title: await titleForUrl(text), private: isPrivate });
  } else {
    const [first, ...rest] = text.split("\n");
    await insertItem(supabase, {
      kind: "note",
      space,
      title: first.slice(0, 200),
      body: rest.join("\n").trim() || (first.length > 200 ? first : ""),
      private: isPrivate,
    });
  }
  refresh();
}

export async function setDone(id: string, done: boolean) {
  const { supabase } = await db();
  await updateItem(supabase, id, { done });
  refresh();
}

export async function setPrivate(id: string, value: boolean) {
  const { supabase } = await db();
  await updateItem(supabase, id, { private: value });
  refresh();
}

export async function deleteItem(id: string) {
  const { supabase } = await db();
  const { data } = await supabase.from("items").select("file_path").eq("id", id).maybeSingle();
  if (data?.file_path) await supabase.storage.from("files").remove([data.file_path]);
  await supabase.from("items").delete().eq("id", id);
  refresh();
}

/** Called after the browser has uploaded the file to storage. */
export async function recordUpload(input: {
  path: string;
  name: string;
  type: string;
  size: number;
  text: string;
  space: Space;
  private: boolean;
}) {
  const { supabase, user } = await db();
  if (!input.path.startsWith(`${user.id}/`)) throw new Error("Bad upload path.");
  await insertItem(supabase, {
    kind: "file",
    space: spaceOf(input.space),
    title: input.name.slice(0, 300),
    body: input.text.slice(0, 200_000),
    file_path: input.path,
    file_type: input.type || null,
    file_size: input.size,
    private: input.private,
  });
  refresh();
}

/** A short-lived link to open an uploaded file. */
export async function fileLink(path: string) {
  const { supabase } = await db();
  const { data, error } = await supabase.storage.from("files").createSignedUrl(path, 60 * 10);
  if (error) throw new Error(error.message);
  return data.signedUrl;
}

export async function deleteAllWork() {
  const { supabase } = await db();
  const { data } = await supabase.from("items").select("file_path").eq("space", "work").not("file_path", "is", null);
  const paths = (data ?? []).map((r) => r.file_path as string);
  if (paths.length) await supabase.storage.from("files").remove(paths);
  await supabase.from("items").delete().eq("space", "work");
  refresh();
}
