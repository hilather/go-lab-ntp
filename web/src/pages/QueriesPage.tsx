import { useEffect, useState } from "react";
import { APIError, QUERY_LIMIT, listQueries } from "../api/client";
import type { QueryEntry } from "../api/types";

const POLL_MS = 5000;

export function QueriesPage() {
  const [items, setItems] = useState<QueryEntry[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const list = await listQueries(QUERY_LIMIT);
        if (!cancelled) {
          setItems(list.items ?? []);
          setError("");
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load queries.");
        }
      }
    }
    void load();
    const id = window.setInterval(() => {
      void load();
    }, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, []);

  if (items === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading queries…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Queries</h1>
      <p className="muted">Newest first. Polled every 5s. The table is the in-memory ring (limit 256).</p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <table className="data">
        <thead>
          <tr>
            <th>whenHost</th>
            <th>clientIP</th>
            <th>filter</th>
            <th>servedTime</th>
            <th>leap</th>
            <th>mode</th>
            <th>vn</th>
          </tr>
        </thead>
        <tbody>
          {(items ?? []).map((q, i) => (
            <tr key={`${q.whenHost ?? ""}-${q.clientIP}-${i}`}>
              <td>
                <code>{q.whenHost ?? "—"}</code>
              </td>
              <td>
                <code>{q.clientIP}</code>
              </td>
              <td>{q.filter || "—"}</td>
              <td>
                <code>{q.servedTime ?? "—"}</code>
              </td>
              <td>
                <span className="chip">{q.leap || "—"}</span>
              </td>
              <td>
                <span className="chip">{q.mode || "—"}</span>
              </td>
              <td>{q.vn}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
