"use client";

import { useEffect, useState } from "react";

/** Rendered in the browser so the greeting and date use the device's time zone. */
export function Greeting() {
  const [now, setNow] = useState<Date | null>(null);
  useEffect(() => setNow(new Date()), []);

  const h = now?.getHours() ?? 12;
  const part = h < 5 ? "Up late" : h < 12 ? "Good morning" : h < 17 ? "Good afternoon" : "Good evening";

  return (
    <header className="mb-5">
      <p className="text-sm text-muted">
        {now ? now.toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" }) : " "}
      </p>
      <h1 className="text-2xl font-semibold tracking-tight">{now ? part : " "}</h1>
    </header>
  );
}
