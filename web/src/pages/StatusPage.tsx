import { useEffect, useState } from "react";
import { APIError, getState, getStatus } from "../api/client";
import type { StateView, Status } from "../api/types";

export function StatusPage() {
  const [status, setStatus] = useState<Status | null>(null);
  const [state, setState] = useState<StateView | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [st, sv] = await Promise.all([getStatus(), getState()]);
        if (!cancelled) {
          setStatus(st);
          setState(sv);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load status.");
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
  if (status === null || state === null) {
    return (
      <main className="page">
        <p role="status">Loading status…</p>
      </main>
    );
  }

  const ntp = status.listeners?.find((l) => l.name === "ntp");
  const ntpBound = Boolean(status.ready && ntp);

  return (
    <main className="page">
      <h1>Status</h1>
      <p className="banner-warn">LabNTP is not a production time source. The host clock never moves.</p>
      <dl>
        <div>
          <dt>Ready</dt>
          <dd>
            <strong>{status.ready ? "yes" : "no"}</strong>
          </dd>
        </div>
        <div>
          <dt>NTP bound</dt>
          <dd>
            <strong>{ntpBound ? "yes" : "no"}</strong>
            {ntp ? (
              <>
                {" "}
                (<code>{ntp.address}</code>)
              </>
            ) : null}
          </dd>
        </div>
        <div>
          <dt>Drifted</dt>
          <dd>
            <strong>{state.drifted ? "yes" : "no"}</strong>
          </dd>
        </div>
        <div>
          <dt>HostTime</dt>
          <dd>
            <code>{status.hostTime || "—"}</code>
          </dd>
        </div>
      </dl>
      <h2>Listeners</h2>
      <ul>
        {(status.listeners ?? []).map((l) => (
          <li key={l.name}>
            {l.name}: <code>{l.address}</code>
          </li>
        ))}
      </ul>
      <h2>Revisions</h2>
      <dl>
        <div>
          <dt>Bootstrap</dt>
          <dd>
            <code>{state.bootstrapRevision}</code>
          </dd>
        </div>
        <div>
          <dt>Runtime</dt>
          <dd>
            <code>{state.runtimeRevision}</code>
          </dd>
        </div>
        <div>
          <dt>Generation</dt>
          <dd>{state.generation}</dd>
        </div>
      </dl>
      <h2>Warnings</h2>
      {(status.warnings ?? []).length === 0 ? (
        <p className="muted">None.</p>
      ) : (
        <ul>
          {(status.warnings ?? []).map((w, i) => (
            <li key={`${w.Code ?? "w"}-${i}`}>
              <code>{w.Code || "warning"}</code> {w.Message || ""}
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
