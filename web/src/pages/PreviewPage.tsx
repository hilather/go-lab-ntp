import { FormEvent, useState } from "react";
import { APIError, previewView } from "../api/client";
import type { Preview } from "../api/types";

export function PreviewPage() {
  const [result, setResult] = useState<Preview | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault();
    const fd = new FormData(ev.currentTarget);
    const ip = String(fd.get("ip") ?? "").trim();
    setError("");
    setResult(null);
    if (ip === "") {
      setError("Enter an IP address.");
      return;
    }
    setBusy(true);
    try {
      setResult(await previewView(ip));
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Preview failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page">
      <h1>Preview</h1>
      <p>
        Preview does not send NTP. Host-publish NAT collision does not affect preview (D24). Compose-network
        sources and this page stay reliable without <code>userland-proxy: false</code>.
      </p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <form className="row" onSubmit={(e) => void onSubmit(e)}>
        <div className="field">
          <label htmlFor="preview-ip">IP address</label>
          <input id="preview-ip" name="ip" autoComplete="off" spellCheck={false} required />
        </div>
        <button type="submit" disabled={busy}>
          {busy ? "Previewing…" : "Preview"}
        </button>
      </form>
      {result ? <PreviewResult result={result} /> : null}
    </main>
  );
}

function PreviewResult({ result }: { result: Preview }) {
  if (result.reason) {
    return (
      <section>
        <h2>Result</h2>
        <p>
          IP <code>{result.ip}</code> is <strong>{result.reason}</strong>. Served time is not available.
        </p>
      </section>
    );
  }
  return (
    <section>
      <h2>Result</h2>
      <dl>
        <div>
          <dt>Filter</dt>
          <dd>{result.filter || "—"}</dd>
        </div>
        <div>
          <dt>Served time</dt>
          <dd>
            <code>{result.servedTime ?? "—"}</code>
          </dd>
        </div>
        <div>
          <dt>Host time</dt>
          <dd>
            <code>{result.hostTime}</code>
          </dd>
        </div>
        <div>
          <dt>Mode</dt>
          <dd>
            <span className="chip">{result.mode || "—"}</span>
          </dd>
        </div>
        <div>
          <dt>Leap</dt>
          <dd>
            <span className="chip">{result.leap || "—"}</span>
          </dd>
        </div>
        <div>
          <dt>Stratum</dt>
          <dd>
            <span className={result.stratum === 16 ? "chip chip--unsync" : "chip"}>{result.stratum ?? "—"}</span>
          </dd>
        </div>
        <div>
          <dt>refid</dt>
          <dd>
            <code>{result.refid || "—"}</code>
          </dd>
        </div>
        <div>
          <dt>Offset from host</dt>
          <dd>{result.offsetFromHost || "—"}</dd>
        </div>
      </dl>
    </section>
  );
}
