"use client";

import { useEffect, useState } from "react";
import { inr, type Order } from "@udyam/shared";
import { Card, Spinner } from "@/components/ui";
import { api } from "@/lib/journey";

const TONE: Record<string, string> = {
  activated: "bg-green-100 text-green-800", failed: "bg-red-100 text-red-800", delayed: "bg-amber-100 text-amber-900",
};

/** Every order in the pipeline. Table on desktop, cards on phones. */
export default function AdminOrders() {
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [status, setStatus] = useState("");

  useEffect(() => {
    let stop = false;
    const load = () => api.tracker.orders(status ? { status } : {}).then((r) => { if (!stop) setOrders(r.orders); }).catch(() => {});
    void load();
    const id = setInterval(load, 4000);
    return () => { stop = true; clearInterval(id); };
  }, [status]);

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-bold">Orders</h1>
        <select value={status} onChange={(e) => setStatus(e.target.value)} className="min-h-10 rounded-lg border border-gray-300 bg-white px-2">
          <option value="">All statuses</option>
          {["kyc_pending", "kyc_passed", "install_scheduled", "installing", "activated", "delayed", "failed"].map((s) => <option key={s}>{s}</option>)}
        </select>
      </div>
      {!orders && <Spinner />}
      {orders && orders.length === 0 && <Card>No orders yet.</Card>}
      {orders && orders.length > 0 && (
        <>
          <ul className="space-y-2 md:hidden">
            {orders.map((o) => (
              <li key={o.id}>
                <Card>
                  <div className="flex justify-between gap-2">
                    <span className="truncate font-mono text-sm">{o.leadId}</span>
                    <span className={`shrink-0 rounded px-2 text-xs ${TONE[o.status] ?? "bg-gray-100"}`}>{o.status}</span>
                  </div>
                  <p className="text-sm text-gray-600">{inr(o.monthlyTotalInr)}/mo · {o.productIds.join(", ")}</p>
                  <p className="text-xs text-gray-500">ETA {o.expectedActivationDate}{o.statusReason ? ` · ${o.statusReason}` : ""}</p>
                </Card>
              </li>
            ))}
          </ul>
          <Card className="hidden overflow-x-auto p-0 sm:p-0 md:block">
            <table className="w-full text-left text-sm">
              <thead className="bg-gray-50 text-gray-600">
                <tr>{["Lead", "Status", "Reason", "Products", "Monthly", "ETA", "Updated"].map((h) => <th key={h} className="px-4 py-2 font-medium">{h}</th>)}</tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {orders.map((o) => (
                  <tr key={o.id}>
                    <td className="px-4 py-2 font-mono">{o.leadId}</td>
                    <td className="px-4 py-2"><span className={`rounded px-2 py-0.5 text-xs ${TONE[o.status] ?? "bg-gray-100"}`}>{o.status}</span></td>
                    <td className="px-4 py-2 text-gray-600">{o.statusReason}</td>
                    <td className="px-4 py-2">{o.productIds.join(", ")}</td>
                    <td className="px-4 py-2 tabular-nums">{inr(o.monthlyTotalInr)}</td>
                    <td className="px-4 py-2">{o.expectedActivationDate}</td>
                    <td className="px-4 py-2 text-gray-500">{new Date(o.updatedAt).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        </>
      )}
    </>
  );
}
