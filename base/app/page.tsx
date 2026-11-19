import Link from "next/link";
import { Page } from "@/components/Nav";
import { Chat } from "@/components/Chat";
import { Capture } from "@/components/Capture";
import { ItemRow } from "@/components/ItemRow";
import { Greeting } from "@/components/Greeting";
import { getUser } from "@/lib/supabase/server";
import { findItems, openTodos } from "@/lib/items";

export default async function Home() {
  const { supabase } = await getUser();
  const [todos, recent] = await Promise.all([openTodos(supabase, { limit: 8 }), findItems(supabase, { limit: 5 })]);
  const recentNonTodo = recent.filter((i) => i.kind !== "todo").slice(0, 4);

  return (
    <Page>
      <Greeting />

      <Chat />

      <section className="mt-8">
        <div className="mb-1 flex items-baseline justify-between">
          <h2 className="font-semibold">To-dos</h2>
          <Link href="/memory?kind=todo" className="text-sm text-muted">All</Link>
        </div>
        {todos.length === 0 ? (
          <p className="py-3 text-sm text-muted">Nothing open. Add one below or ask Base.</p>
        ) : (
          <ul className="divide-y divide-line">
            {todos.map((t) => (
              <ItemRow key={t.id} item={t} compact />
            ))}
          </ul>
        )}
      </section>

      <section className="mt-6">
        <h2 className="mb-3 font-semibold">Capture</h2>
        <Capture />
      </section>

      {recentNonTodo.length > 0 && (
        <section className="mt-8">
          <div className="mb-1 flex items-baseline justify-between">
            <h2 className="font-semibold">Recently saved</h2>
            <Link href="/memory" className="text-sm text-muted">Memory</Link>
          </div>
          <ul className="divide-y divide-line">
            {recentNonTodo.map((i) => (
              <ItemRow key={i.id} item={i} compact />
            ))}
          </ul>
        </section>
      )}
    </Page>
  );
}
