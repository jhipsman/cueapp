"use client";

import { useTransition } from "react";
import { deleteItem, fileLink, setDone, setPrivate } from "@/app/memory/actions";
import type { Item } from "@/lib/items";

const KIND_LABEL: Record<Item["kind"], string> = { note: "Note", link: "Link", file: "File", todo: "To-do" };

export function ItemRow({ item, compact = false }: { item: Item; compact?: boolean }) {
  const [pending, start] = useTransition();

  return (
    <li className={`flex gap-3 py-3 ${pending ? "opacity-50" : ""}`}>
      {item.kind === "todo" ? (
        <input
          type="checkbox"
          checked={item.done}
          onChange={(e) => start(() => setDone(item.id, e.target.checked))}
          className="mt-1 h-4 w-4 shrink-0 accent-[var(--accent)]"
          aria-label="Done"
        />
      ) : (
        <span className="mt-0.5 w-10 shrink-0 text-xs text-muted">{KIND_LABEL[item.kind]}</span>
      )}

      <div className="min-w-0 flex-1">
        <div className={`break-words ${item.done ? "text-muted line-through" : ""}`}>
          {item.kind === "link" && item.url ? (
            <a href={item.url} target="_blank" rel="noreferrer" className="underline decoration-line underline-offset-4">
              {item.title || item.url}
            </a>
          ) : item.kind === "file" && item.file_path ? (
            <button
              className="text-left underline decoration-line underline-offset-4"
              onClick={() => {
                // Open the tab now, while the tap still counts as a user action,
                // or phones block it as a popup once the link arrives.
                const tab = window.open("", "_blank");
                start(async () => {
                  const url = await fileLink(item.file_path!);
                  if (tab) tab.location.href = url;
                  else window.location.href = url;
                });
              }}
            >
              {item.title}
            </button>
          ) : (
            item.title || item.body.slice(0, 120)
          )}
        </div>
        {!compact && item.kind === "note" && item.body && (
          <p className="mt-1 line-clamp-3 whitespace-pre-wrap text-sm text-muted">{item.body}</p>
        )}
        <div className="mt-1 flex flex-wrap gap-x-3 text-xs text-muted">
          {item.space === "work" && <span className="text-work">Work</span>}
          {item.due_at && <span>Due {formatDue(item.due_at)}</span>}
          {item.private && <span>Kept out of AI</span>}
          {!compact && <span>{new Date(item.created_at).toLocaleDateString()}</span>}
        </div>
      </div>

      {!compact && (
        <details className="relative shrink-0 text-sm">
          <summary className="cursor-pointer list-none px-2 text-muted" aria-label="More">⋯</summary>
          <div className="absolute right-0 z-10 mt-1 flex w-44 flex-col rounded-xl border border-line bg-surface p-1 shadow-lg">
            <button className="rounded-lg px-3 py-2 text-left hover:bg-bg" onClick={() => start(() => setPrivate(item.id, !item.private))}>
              {item.private ? "Let AI see this" : "Keep out of AI"}
            </button>
            <button
              className="rounded-lg px-3 py-2 text-left text-danger hover:bg-bg"
              onClick={() => confirm("Delete this for good?") && start(() => deleteItem(item.id))}
            >
              Delete
            </button>
          </div>
        </details>
      )}
    </li>
  );
}

function formatDue(iso: string) {
  const d = new Date(iso);
  const today = new Date();
  const sameDay = d.toDateString() === today.toDateString();
  const time = d.getHours() || d.getMinutes() ? d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }) : "";
  if (sameDay) return time ? `today ${time}` : "today";
  return d.toLocaleDateString([], { weekday: "short", month: "short", day: "numeric" }) + (time ? ` ${time}` : "");
}
