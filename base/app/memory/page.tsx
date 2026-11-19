import Link from "next/link";
import { Page } from "@/components/Nav";
import { Capture } from "@/components/Capture";
import { ItemRow } from "@/components/ItemRow";
import { DeleteWorkButton } from "@/components/DeleteWorkButton";
import { getUser } from "@/lib/supabase/server";
import { findItems, KINDS, SPACES, type Kind, type Space } from "@/lib/items";

export const metadata = { title: "Memory · Base" };

type Search = { q?: string; space?: string; kind?: string };

export default async function MemoryPage({ searchParams }: { searchParams: Promise<Search> }) {
  const sp = await searchParams;
  const space = SPACES.includes(sp.space as Space) ? (sp.space as Space) : undefined;
  const kind = KINDS.includes(sp.kind as Kind) ? (sp.kind as Kind) : undefined;
  const q = sp.q?.slice(0, 200);

  const { supabase } = await getUser();
  const items = await findItems(supabase, { q, space, kind, limit: 100 });

  const href = (patch: Partial<Search>) => {
    const next = { q, space, kind, ...patch };
    const params = new URLSearchParams(Object.entries(next).filter(([, v]) => v) as [string, string][]);
    const s = params.toString();
    return s ? `/memory?${s}` : "/memory";
  };

  return (
    <Page>
      <h1 className="mb-4 text-2xl font-semibold tracking-tight">Memory</h1>

      <Capture defaultSpace={space ?? "personal"} />

      <form action="/memory" className="mt-6">
        {space && <input type="hidden" name="space" value={space} />}
        {kind && <input type="hidden" name="kind" value={kind} />}
        <input
          name="q"
          defaultValue={q}
          placeholder="Search everything…"
          className="w-full rounded-xl border border-line bg-surface px-4 py-2.5 outline-none focus:border-accent"
        />
      </form>

      <div className="mt-3 flex flex-wrap gap-2 text-sm">
        <Chip href={href({ space: undefined })} on={!space}>All</Chip>
        <Chip href={href({ space: "personal" })} on={space === "personal"}>Personal</Chip>
        <Chip href={href({ space: "work" })} on={space === "work"}>Work</Chip>
        <span className="mx-1 w-px bg-line" />
        <Chip href={href({ kind: undefined })} on={!kind}>Everything</Chip>
        <Chip href={href({ kind: "note" })} on={kind === "note"}>Notes</Chip>
        <Chip href={href({ kind: "link" })} on={kind === "link"}>Links</Chip>
        <Chip href={href({ kind: "file" })} on={kind === "file"}>Files</Chip>
        <Chip href={href({ kind: "todo" })} on={kind === "todo"}>To-dos</Chip>
      </div>

      {items.length === 0 ? (
        <p className="mt-10 text-center text-muted">{q ? "Nothing matches that." : "Nothing saved yet."}</p>
      ) : (
        <ul className="mt-4 divide-y divide-line">
          {items.map((item) => (
            <ItemRow key={item.id} item={item} />
          ))}
        </ul>
      )}

      {space === "work" && <DeleteWorkButton />}
    </Page>
  );
}

function Chip({ href, on, children }: { href: string; on: boolean; children: React.ReactNode }) {
  return (
    <Link href={href} className={`rounded-full border px-3 py-1 ${on ? "border-accent text-accent" : "border-line text-muted"}`}>
      {children}
    </Link>
  );
}
