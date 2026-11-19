"use client";

import { useTransition } from "react";
import { deleteAllWork } from "@/app/memory/actions";

export function DeleteWorkButton() {
  const [pending, start] = useTransition();
  return (
    <div className="mt-10 border-t border-line pt-6 text-center">
      <button
        disabled={pending}
        onClick={() =>
          confirm("Delete every work note, link, file and to-do? This can't be undone.") && start(() => deleteAllWork())
        }
        className="text-sm text-danger disabled:opacity-60"
      >
        {pending ? "Deleting…" : "Delete all work data"}
      </button>
    </div>
  );
}
