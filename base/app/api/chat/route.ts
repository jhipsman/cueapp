import Anthropic from "@anthropic-ai/sdk";
import type { BetaMessageParam } from "@anthropic-ai/sdk/resources/beta/messages/messages";
import { z } from "zod";
import { getUser } from "@/lib/supabase/server";
import { MODEL, SYSTEM_PROMPT, makeTools, nowMessage } from "@/lib/assistant";

export const maxDuration = 300;

const Body = z.object({
  messages: z
    .array(z.object({ role: z.enum(["user", "assistant"]), content: z.string().min(1).max(50_000) }))
    .min(1)
    .max(200),
  timeZone: z.string().max(64).default("UTC"),
});

/** What the browser receives: one JSON object per line. */
export type ChatEvent =
  | { type: "text"; text: string }
  | { type: "tool"; name: string }
  | { type: "error"; message: string }
  | { type: "done" };

const TOOL_LABELS: Record<string, string> = {
  search_memory: "Searching memory",
  read_item: "Reading",
  recent_items: "Looking at recent items",
  list_todos: "Checking to-dos",
  save: "Saving",
  update_item: "Updating",
  web_search: "Searching the web",
};

export async function POST(req: Request) {
  const { supabase, user } = await getUser();
  if (!user) return Response.json({ error: "Sign in first." }, { status: 401 });
  if (!process.env.ANTHROPIC_API_KEY) {
    return Response.json({ error: "The assistant needs ANTHROPIC_API_KEY set on the server." }, { status: 503 });
  }

  const parsed = Body.safeParse(await req.json().catch(() => null));
  if (!parsed.success) return Response.json({ error: "Bad request." }, { status: 400 });
  const { messages, timeZone } = parsed.data;
  if (messages.at(-1)?.role !== "user") return Response.json({ error: "Last message must be yours." }, { status: 400 });

  const history: BetaMessageParam[] = [
    ...messages,
    { role: "system", content: nowMessage(timeZone) },
  ];

  const client = new Anthropic();
  const encoder = new TextEncoder();

  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      const send = (e: ChatEvent) => controller.enqueue(encoder.encode(JSON.stringify(e) + "\n"));
      try {
        const runner = client.beta.messages.toolRunner({
          model: MODEL,
          max_tokens: 16000,
          system: [{ type: "text", text: SYSTEM_PROMPT, cache_control: { type: "ephemeral" } }],
          output_config: { effort: "medium" },
          tools: [...makeTools(supabase), { type: "web_search_20250305", name: "web_search", max_uses: 3 }],
          messages: history,
          max_iterations: 8,
          stream: true,
        });

        for await (const messageStream of runner) {
          for await (const event of messageStream) {
            if (event.type === "content_block_start") {
              const block = event.content_block;
              if (block.type === "tool_use" || block.type === "server_tool_use") {
                send({ type: "tool", name: TOOL_LABELS[block.name] ?? block.name });
              }
            } else if (event.type === "content_block_delta" && event.delta.type === "text_delta") {
              send({ type: "text", text: event.delta.text });
            }
          }
          const message = await messageStream.finalMessage();
          if (message.stop_reason === "refusal") {
            send({ type: "error", message: "I can't help with that one." });
            break;
          }
          if (message.stop_reason === "max_tokens" && message.content.some((b) => b.type === "tool_use")) {
            send({ type: "error", message: "That ran too long. Try asking in smaller pieces." });
            break;
          }
        }
        send({ type: "done" });
      } catch (err) {
        console.error("chat failed", err);
        const message =
          err instanceof Anthropic.RateLimitError
            ? "Too many requests right now. Try again in a moment."
            : err instanceof Anthropic.AuthenticationError
              ? "The Claude API key isn't working. Check ANTHROPIC_API_KEY."
              : err instanceof Anthropic.APIError
                ? `Claude API error (${err.status ?? "network"}). Try again.`
                : "Something went wrong. Try again.";
        send({ type: "error", message });
      } finally {
        controller.close();
      }
    },
  });

  return new Response(stream, {
    headers: { "content-type": "application/x-ndjson; charset=utf-8", "cache-control": "no-store" },
  });
}
