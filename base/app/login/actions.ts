"use server";

import { headers } from "next/headers";
import { createClient } from "@/lib/supabase/server";

export type LoginState = { status: "idle" | "sent" | "error"; message?: string };

function allowed(email: string) {
  const list = (process.env.BASE_ALLOWED_EMAILS ?? "")
    .split(",")
    .map((e) => e.trim().toLowerCase())
    .filter(Boolean);
  // With no list set, nobody can sign in: Base is private by default.
  return list.includes(email);
}

export async function sendLink(_prev: LoginState, form: FormData): Promise<LoginState> {
  const email = String(form.get("email") ?? "").trim().toLowerCase();
  if (!email.includes("@")) return { status: "error", message: "Enter your email address." };

  // Same answer either way, so the page doesn't reveal who has access.
  if (!allowed(email)) return { status: "sent" };

  const h = await headers();
  const origin = h.get("origin") ?? `https://${h.get("host")}`;
  const supabase = await createClient();
  const { error } = await supabase.auth.signInWithOtp({
    email,
    options: { emailRedirectTo: `${origin}/auth/callback` },
  });
  if (error) return { status: "error", message: error.message };
  return { status: "sent" };
}
