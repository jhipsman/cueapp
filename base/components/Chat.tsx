"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { ChatEvent } from "@/app/api/chat/route";

type Msg = { role: "user" | "assistant"; content: string };
const STORE_KEY = "base.chat.v1";
const SUGGESTIONS = ["What's on my plate today?", "Remind me to call the bank Friday at 10", "What did I save about…"];

/** The assistant. The conversation is kept on this device until you clear it. */
export function Chat() {
  const [msgs, setMsgs] = useState<Msg[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [activity, setActivity] = useState("");
  const [error, setError] = useState("");
  const endRef = useRef<HTMLDivElement>(null);
  const router = useRouter();

  useEffect(() => {
    try {
      const saved = localStorage.getItem(STORE_KEY);
      if (saved) setMsgs(JSON.parse(saved));
    } catch {}
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem(STORE_KEY, JSON.stringify(msgs.slice(-60)));
    } catch {}
    endRef.current?.scrollIntoView({ block: "end" });
  }, [msgs]);

  async function send(text: string) {
    text = text.trim();
    if (!text || busy) return;
    setError("");
    setInput("");
    const history: Msg[] = [...msgs, { role: "user", content: text }];
    setMsgs([...history, { role: "assistant", content: "" }]);
    setBusy(true);
    let changed = false;

    try {
      const res = await fetch("/api/chat", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          messages: history.filter((m) => m.content).slice(-40),
          timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        }),
      });
      if (!res.ok || !res.body) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error ?? `Request failed (${res.status}).`);
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() ?? "";
        for (const line of lines) {
          if (!line) continue;
          const e = JSON.parse(line) as ChatEvent;
          if (e.type === "text") {
            setActivity("");
            setMsgs((m) => {
              const copy = m.slice();
              const last = copy[copy.length - 1];
              copy[copy.length - 1] = { ...last, content: last.content + e.text };
              return copy;
            });
          } else if (e.type === "tool") {
            setActivity(e.name);
            if (e.name === "Saving" || e.name === "Updating") changed = true;
          } else if (e.type === "error") {
            setError(e.message);
          }
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setBusy(false);
      setActivity("");
      // Drop an empty reply so the next request doesn't send it.
      setMsgs((m) => (m.at(-1)?.role === "assistant" && !m.at(-1)?.content ? m.slice(0, -1) : m));
      if (changed) router.refresh();
    }
  }

  return (
    <section className="rounded-2xl border border-line bg-surface">
      <div className="max-h-[55dvh] overflow-y-auto px-4 pt-4">
        {msgs.length === 0 ? (
          <div className="pb-2">
            <p className="text-muted">Ask anything, or tell me something to remember.</p>
            <div className="mt-3 flex flex-wrap gap-2">
              {SUGGESTIONS.map((s) => (
                <button
                  key={s}
                  onClick={() => (s.endsWith("…") ? setInput(s.slice(0, -1) + " ") : send(s))}
                  className="rounded-full border border-line px-3 py-1 text-sm text-muted"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-3 pb-2">
            {msgs.map((m, i) => (
              <div
                key={i}
                className={
                  m.role === "user"
                    ? "ml-10 self-end rounded-2xl rounded-br-md bg-accent px-3.5 py-2 text-accent-fg"
                    : "mr-6 whitespace-pre-wrap leading-relaxed"
                }
              >
                {m.content || (busy && i === msgs.length - 1 ? <span className="text-muted">{activity || "Thinking"}…</span> : null)}
              </div>
            ))}
            {busy && activity && msgs.at(-1)?.content && <p className="text-sm text-muted">{activity}…</p>}
          </div>
        )}
        {error && <p className="pb-2 text-sm text-danger">{error}</p>}
        <div ref={endRef} />
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          send(input);
        }}
        className="flex items-end gap-2 border-t border-line p-2"
      >
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send(input);
            }
          }}
          rows={1}
          placeholder="Ask Base…"
          className="max-h-40 min-h-10 flex-1 resize-none bg-transparent px-2 py-2 outline-none placeholder:text-muted"
        />
        {msgs.length > 0 && !busy && (
          <button type="button" onClick={() => setMsgs([])} className="px-2 py-2 text-sm text-muted">
            Clear
          </button>
        )}
        <button
          disabled={busy || !input.trim()}
          className="rounded-xl bg-accent px-4 py-2 font-medium text-accent-fg disabled:opacity-50"
        >
          Send
        </button>
      </form>
    </section>
  );
}
