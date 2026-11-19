"use client";

import { useActionState } from "react";
import { sendLink, type LoginState } from "./actions";

export default function LoginPage() {
  const [state, action, pending] = useActionState<LoginState, FormData>(sendLink, { status: "idle" });

  return (
    <main className="mx-auto flex min-h-dvh max-w-sm flex-col justify-center px-4">
      <div className="mb-8 flex items-center gap-3">
        <span className="grid h-10 w-10 place-items-center rounded-xl bg-accent text-lg font-semibold text-accent-fg">B</span>
        <h1 className="text-2xl font-semibold tracking-tight">Base</h1>
      </div>

      {state.status === "sent" ? (
        <p className="rounded-xl border border-line bg-surface p-4 text-sm leading-relaxed">
          If that address has access, a sign-in link is on its way. Open it on this device.
        </p>
      ) : (
        <form action={action} className="flex flex-col gap-3">
          <label className="text-sm text-muted" htmlFor="email">Email</label>
          <input
            id="email"
            name="email"
            type="email"
            autoComplete="email"
            required
            className="rounded-xl border border-line bg-surface px-4 py-3 outline-none focus:border-accent"
          />
          <button
            disabled={pending}
            className="rounded-xl bg-accent px-4 py-3 font-medium text-accent-fg disabled:opacity-60"
          >
            {pending ? "Sending…" : "Email me a sign-in link"}
          </button>
          {state.status === "error" && <p className="text-sm text-danger">{state.message}</p>}
        </form>
      )}
    </main>
  );
}
