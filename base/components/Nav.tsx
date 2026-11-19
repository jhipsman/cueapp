"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const TABS = [
  { href: "/", label: "Home" },
  { href: "/memory", label: "Memory" },
];

/** Bottom tab bar on phones, top bar on wider screens. */
export function Nav() {
  const path = usePathname();
  return (
    <nav className="fixed inset-x-0 bottom-0 z-20 border-t border-line bg-bg/90 pb-[env(safe-area-inset-bottom)] backdrop-blur sm:static sm:border-t-0 sm:border-b sm:pb-0">
      <div className="mx-auto flex max-w-2xl items-center gap-1 px-4">
        <span className="mr-auto hidden py-3 font-semibold tracking-tight sm:block">Base</span>
        {TABS.map((t) => {
          const active = t.href === "/" ? path === "/" : path.startsWith(t.href);
          return (
            <Link
              key={t.href}
              href={t.href}
              className={`flex-1 py-3 text-center text-sm sm:flex-none sm:px-3 ${
                active ? "font-semibold text-fg" : "text-muted"
              }`}
            >
              {t.label}
            </Link>
          );
        })}
      </div>
    </nav>
  );
}

export function Page({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Nav />
      <main className="mx-auto max-w-2xl px-4 pt-[max(1rem,env(safe-area-inset-top))] pb-28 sm:pt-6 sm:pb-10">
        {children}
      </main>
    </>
  );
}
