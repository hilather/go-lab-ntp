import { useCallback, useEffect, useState } from "react";
import { APIError, getState, listFilters, putFilter } from "../api/client";
import type { Filter } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE } from "../auth/scopes";

export function FiltersPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [items, setItems] = useState<Filter[] | null>(null);
  const [error, setError] = useState("");
  const [busyName, setBusyName] = useState("");

  const reload = useCallback(async () => {
    const list = await listFilters();
    setItems(list.items ?? []);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        await reload();
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load filters.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reload]);

  async function onToggle(filter: Filter, enabled: boolean) {
    if (!canWrite) {
      return;
    }
    setBusyName(filter.name);
    setError("");
    try {
      const state = await getState();
      const next: Filter = { ...filter, enabled, match: { ...filter.match }, view: { ...filter.view } };
      await putFilter(filter.name, {
        expectedRevision: state.runtimeRevision,
        reason: enabled ? "ui: enable filter" : "ui: disable filter",
        filter: next,
      });
      await reload();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Could not update filter.");
    } finally {
      setBusyName("");
    }
  }

  if (items === null && error === "") {
    return (
      <main className="page">
        <p role="status">Loading filters…</p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1>Filters</h1>
      <p>First enabled match wins. Longest-prefix does not.</p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <table className="data">
        <caption>First enabled match wins. Longest-prefix does not.</caption>
        <thead>
          <tr>
            <th>Name</th>
            <th>Enabled</th>
            <th>CIDRs</th>
            <th>Mode</th>
            <th>Leap</th>
            <th>Stratum</th>
            <th>refid</th>
          </tr>
        </thead>
        <tbody>
          {(items ?? []).map((f) => (
            <tr key={f.name}>
              <td>{f.name}</td>
              <td>
                <input
                  type="checkbox"
                  aria-label={`Enable ${f.name}`}
                  checked={f.enabled}
                  disabled={!canWrite || busyName === f.name}
                  onChange={(e) => void onToggle(f, e.target.checked)}
                />
              </td>
              <td>{(f.match.cidrs ?? []).join(", ")}</td>
              <td>
                <span className="chip">{f.view.mode}</span>
              </td>
              <td>
                <span className="chip">{f.view.leap}</span>
              </td>
              <td>
                <span className={f.view.stratum === 16 ? "chip chip--unsync" : "chip"}>{f.view.stratum}</span>
              </td>
              <td>
                <code>{f.view.refid}</code>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  );
}
