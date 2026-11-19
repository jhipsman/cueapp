"use client";

import { useRef, useState, useTransition } from "react";
import { capture, recordUpload } from "@/app/memory/actions";
import { createClient } from "@/lib/supabase/client";
import type { Space } from "@/lib/items";

const TEXT_TYPES = /^(text\/|application\/(json|xml|csv|x-yaml|yaml|markdown))/;
const TEXT_EXT = /\.(txt|md|markdown|csv|tsv|json|ya?ml|xml|html?|log|ics|vtt|srt)$/i;

/** Capture a note, link or to-do, or upload files, into personal or work. */
export function Capture({ defaultSpace = "personal" }: { defaultSpace?: Space }) {
  const [space, setSpace] = useState<Space>(defaultSpace);
  const [asTodo, setAsTodo] = useState(false);
  const [isPrivate, setPrivate] = useState(false);
  const [status, setStatus] = useState("");
  const [pending, start] = useTransition();
  const formRef = useRef<HTMLFormElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  async function upload(files: FileList | null) {
    if (!files?.length) return;
    const supabase = createClient();
    const { data } = await supabase.auth.getUser();
    if (!data.user) return setStatus("Sign in again to upload.");
    for (const file of Array.from(files)) {
      setStatus(`Uploading ${file.name}…`);
      const safe = file.name.replace(/[^\w.\-]+/g, "_");
      const path = `${data.user.id}/${crypto.randomUUID()}-${safe}`;
      const { error } = await supabase.storage.from("files").upload(path, file, { contentType: file.type || undefined });
      if (error) return setStatus(`Couldn't upload ${file.name}: ${error.message}`);
      // Text files become searchable right away. Other types keep just their name for now.
      const text =
        (TEXT_TYPES.test(file.type) || TEXT_EXT.test(file.name)) && file.size < 2_000_000 ? await file.text() : "";
      await recordUpload({ path, name: file.name, type: file.type, size: file.size, text, space, private: isPrivate });
    }
    setStatus("");
    if (fileRef.current) fileRef.current.value = "";
  }

  return (
    <form
      ref={formRef}
      action={(fd) =>
        start(async () => {
          await capture(fd);
          formRef.current?.reset();
        })
      }
      className="rounded-2xl border border-line bg-surface p-3"
    >
      <input type="hidden" name="space" value={space} />
      <input type="hidden" name="kind" value={asTodo ? "todo" : ""} />
      {isPrivate && <input type="hidden" name="private" value="on" />}
      <textarea
        name="text"
        rows={2}
        placeholder={asTodo ? "Something to do…" : "A note, an idea, or paste a link…"}
        className="w-full resize-none bg-transparent px-1 py-1 outline-none placeholder:text-muted"
        onKeyDown={(e) => {
          if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) formRef.current?.requestSubmit();
        }}
      />
      <div className="mt-2 flex flex-wrap items-center gap-2 text-sm">
        <Toggle on={space === "work"} onClick={() => setSpace(space === "work" ? "personal" : "work")}>
          {space === "work" ? "Work" : "Personal"}
        </Toggle>
        <Toggle on={asTodo} onClick={() => setAsTodo(!asTodo)}>To-do</Toggle>
        <Toggle on={isPrivate} onClick={() => setPrivate(!isPrivate)} title="The assistant never sees private items">
          Keep out of AI
        </Toggle>
        <button
          type="button"
          onClick={() => fileRef.current?.click()}
          className="rounded-full border border-line px-3 py-1 text-muted"
        >
          Upload
        </button>
        <input ref={fileRef} type="file" multiple hidden onChange={(e) => upload(e.target.files)} />
        <button disabled={pending} className="ml-auto rounded-full bg-accent px-4 py-1.5 font-medium text-accent-fg disabled:opacity-60">
          Save
        </button>
      </div>
      {status && <p className="mt-2 text-sm text-muted">{status}</p>}
    </form>
  );
}

function Toggle({
  on,
  onClick,
  title,
  children,
}: {
  on: boolean;
  onClick: () => void;
  title?: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      title={title}
      aria-pressed={on}
      onClick={onClick}
      className={`rounded-full border px-3 py-1 ${on ? "border-accent text-accent" : "border-line text-muted"}`}
    >
      {children}
    </button>
  );
}
