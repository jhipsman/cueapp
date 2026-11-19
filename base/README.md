# Base

Your home base: one app, with one assistant, for your day, your memory, your work and your life. It runs on Vercel with Supabase, and installs to your phone's home screen.

This is **step 1**: the shell, the assistant and memory.

## What's here

- **Home**: a greeting, the assistant chat, open to-dos, quick capture, and what you saved recently.
- **Assistant**: Claude Haiku 5.5 (`claude-haiku-5-5`, fast and inexpensive) with tools over your memory. It can search, read, save notes, links and to-dos, and mark them done or reschedule them. It can also search the web. It knows the current time in your time zone, so "remind me Friday at 10" gets a real due date. The conversation stays on your device until you press Clear.
- **Memory**: notes, links (the title is filled in for you), files and to-dos, each in **Personal** or **Work**. It has full-text search and filters. Text files (`.txt`, `.md`, `.csv`, `.json`, …) are searchable once uploaded. Other files keep their name for now.
- **Privacy**: any item can be marked **Keep out of AI**, so the assistant never sees it. Work has a **Delete all work data** button. Sign-in is by emailed link, and only to addresses listed in `BASE_ALLOWED_EMAILS`. Row-level security keeps each account's data to itself.

## Layout

```
app/                 pages (Home, Memory, login) and the /api/chat stream
components/          Chat, Capture, ItemRow, Nav
lib/assistant.ts     the assistant's system prompt and tools
lib/items.ts         reading and writing memory
lib/supabase/        Supabase clients for the server and the browser
proxy.ts             keeps you signed in and sends signed-out visitors to /login
supabase/migrations  the database: items table, search, file storage, security rules
```

## Set up

1. **Supabase**: create a project and run `supabase/migrations/0001_init.sql` in the SQL editor. Under Authentication > URL Configuration, set the Site URL to your Vercel address and add `https://<your-app>/auth/callback` as a redirect URL.
2. **Vercel**: import the repository with **Root Directory** set to `base`, and set the environment variables in `.env.example`.
3. Open the site on your phone, then use Share > **Add to Home Screen**.

Run it on your own computer with `npm install`, `.env.local` filled in from `.env.example`, and `npm run dev`.

## Next steps

2. **Today**: your calendar (personal, plus a private link to your work calendar), an email-in address for forwarding, reminders, and a morning summary.
3. **Money**: Stripe income, subscriptions and bills.
4. **Build**: ask for a feature, a Claude Code session builds it, and you merge the pull request from your phone.
5. **Interests**: fantasy football, workouts, and anything else.
