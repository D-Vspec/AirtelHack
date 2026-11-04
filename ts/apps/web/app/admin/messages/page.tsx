"use client";

import { useEffect, useState } from "react";
import type { SentMessage } from "@udyam/shared";
import { Card, Spinner } from "@/components/ui";
import { api } from "@/lib/journey";

/** What the mock WhatsApp, SMS and email adapters sent (notify.sent_messages). */
export default function Messages() {
  const [msgs, setMsgs] = useState<SentMessage[] | null>(null);
  useEffect(() => {
    const load = () => api.notify.messages().then((r) => setMsgs(r.messages)).catch(() => {});
    void load();
    const id = setInterval(load, 4000);
    return () => clearInterval(id);
  }, []);
  return (
    <>
      <h1 className="mb-3 text-xl font-bold">Messages sent</h1>
      {!msgs && <Spinner />}
      {msgs?.length === 0 && <Card>No messages yet.</Card>}
      <ul className="grid gap-2 md:grid-cols-2">
        {msgs?.map((m) => (
          <li key={m.id}>
            <Card>
              <div className="flex flex-wrap justify-between gap-2 text-xs text-gray-600">
                <span className="font-mono">{m.leadId}</span>
                <span>{m.channel} · {m.language} · {m.templateKey}</span>
              </div>
              <p className="mt-1">{m.body}</p>
            </Card>
          </li>
        ))}
      </ul>
    </>
  );
}
