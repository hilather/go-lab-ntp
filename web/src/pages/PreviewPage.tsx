import { FormEvent, useState } from "react";
import { APIError, previewView } from "../api/client";
import type { Preview } from "../api/types";

const TEST_IPS = ["10.99.42.20", "10.99.42.30", "10.99.42.1", "127.0.0.1", "8.8.8.8", "203.0.113.9"];

export function PreviewPage() {
  const [ip, setIP] = useState("");
  const [result, setResult] = useState<Preview | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function runPreview(value: string) {
    const next = value.trim();
    setError("");
    setResult(null);
    if (next === "") {
      setError("Enter an IP address.");
      return;
    }
    setBusy(true);
    try {
      setResult(await previewView(next));
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Preview failed.");
    } finally {
      setBusy(false);
    }
  }

  async function onSubmit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault();
    await runPreview(ip);
  }

  return (
    <main className="page">
      <h1>Preview</h1>
      <p>
        GET /v1/views/preview?ip= — compiled match walk plus allowClientCidrs. Preview does not send NTP. Host-publish
        NAT collision does not affect preview (D24). Compose-network sources and this page stay reliable without{" "}
        <code>userland-proxy: false</code>.
      </p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <form className="panel" onSubmit={(e) => void onSubmit(e)} noValidate>
        <p className="kicker">Client IP</p>
        <div className="row">
          <div className="field field--grow">
            <label htmlFor="preview-ip">IP address</label>
            <input
              id="preview-ip"
              name="ip"
              className="preview-input"
              value={ip}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => setIP(e.target.value)}
            />
          </div>
          <button type="submit" disabled={busy}>
            {busy ? "Previewing…" : "Preview"}
          </button>
        </div>
        <div className="chip-row">
          {TEST_IPS.map((addr) => (
            <button
              key={addr}
              type="button"
              className="chip-btn"
              disabled={busy}
              onClick={() => {
                setIP(addr);
                void runPreview(addr);
              }}
            >
              {addr}
            </button>
          ))}
        </div>
        <p className="muted">Test-data addresses. Each chip calls the same GET.</p>
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
      <div className="result-cards">
        <div className="result-card">
          <p className="kicker">Client IP in</p>
          <strong>{result.ip}</strong>
        </div>
        <div className="result-card">
          <p className="kicker">Virtual time out</p>
          <strong className="served">{result.servedTime ?? "—"}</strong>
        </div>
      </div>
      <dl>
        <div>
          <dt>Filter</dt>
          <dd>{result.filter || "—"}</dd>
        </div>
        <div>
          <dt>Served time</dt>
          <dd>
            <code className="served">{result.servedTime ?? "—"}</code>
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
            <span className="chip chip--mode">{result.mode || "—"}</span>
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
