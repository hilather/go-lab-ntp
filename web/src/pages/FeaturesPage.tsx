import { useEffect, useState } from "react";
import { APIError, listFeatures } from "../api/client";
import type { Feature } from "../api/types";

export function FeaturesPage() {
  const [items, setItems] = useState<Feature[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listFeatures();
        if (!cancelled) {
          setItems(list.items ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load features.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  if (error !== "") {
    return (
      <main className="page">
        <p className="banner-error" role="alert">
          {error}
        </p>
      </main>
    );
  }
  if (items === null) {
    return (
      <main className="page">
        <p role="status">Loading features…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Features</h1>
      <table className="data">
        <thead>
          <tr>
            <th>ID</th>
            <th>Apply</th>
            <th>Path</th>
          </tr>
        </thead>
        <tbody>
          {items.map((f) => (
            <tr key={f.id}>
              <td>
                <code>{f.id}</code>
              </td>
              <td>
                <span className={f.apply === "reset-only" ? "chip chip--reset" : "chip chip--live"}>{f.apply}</span>
              </td>
              <td>
                <code>{f.path}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="muted">UI enablement is bootstrap YAML; reread with Reset. Not a features.list id.</p>
    </main>
  );
}
