import { useCallback, useEffect, useState } from "react";
import { APIError, getState, getStatus, listFilters, putFilter } from "../api/client";
import type { Filter } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE } from "../auth/scopes";
import { buildSaveFilter, buildToggleFilter, draftFromFilter, MODES, modeField, type FilterDraft } from "../ui/filterPut";
import { computeSelectedViewMath, sampleIPFromFilter, type ViewMathResult } from "../ui/viewMath";

function padOrdinal(i: number): string {
  return String(i).padStart(2, "0");
}

export function FiltersPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [items, setItems] = useState<Filter[] | null>(null);
  const [selectedName, setSelectedName] = useState("");
  const [draft, setDraft] = useState<FilterDraft | null>(null);
  const [error, setError] = useState("");
  const [busyName, setBusyName] = useState("");
  const [sampleIP, setSampleIP] = useState("");
  const [math, setMath] = useState<ViewMathResult | null>(null);
  const [mathError, setMathError] = useState("");

  const selected = (items ?? []).find((f) => f.name === selectedName) ?? null;

  const reload = useCallback(async () => {
    const list = await listFilters();
    const next = list.items ?? [];
    setItems(next);
    setSelectedName((cur) => {
      if (cur !== "" && next.some((f) => f.name === cur)) {
        return cur;
      }
      return next[0]?.name ?? "";
    });
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

  useEffect(() => {
    if (!selected) {
      setDraft(null);
      return;
    }
    setDraft(draftFromFilter(selected));
    setSampleIP(sampleIPFromFilter(selected));
    setMath(null);
    setMathError("");
  }, [selected]);

  async function onToggle(enabled: boolean) {
    if (!canWrite || !selected) {
      return;
    }
    setBusyName(selected.name);
    setError("");
    try {
      const state = await getState();
      await putFilter(selected.name, {
        expectedRevision: state.runtimeRevision,
        reason: enabled ? "ui: enable filter" : "ui: disable filter",
        filter: buildToggleFilter(selected, enabled),
      });
      await reload();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Could not update filter.");
    } finally {
      setBusyName("");
    }
  }

  async function onSave() {
    if (!canWrite || !selected || !draft) {
      return;
    }
    const built = buildSaveFilter(selected, draft);
    if (!built.ok) {
      setError(built.error);
      return;
    }
    setBusyName(selected.name);
    setError("");
    try {
      const state = await getState();
      await putFilter(selected.name, {
        expectedRevision: state.runtimeRevision,
        reason: "ui: save view",
        filter: built.filter,
      });
      await reload();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Could not update filter.");
    } finally {
      setBusyName("");
    }
  }

  async function onInPaneMath() {
    if (!selected || !draft) {
      return;
    }
    setMathError("");
    setMath(null);
    try {
      const st = await getStatus();
      const result = computeSelectedViewMath(
        {
          name: selected.name,
          view: {
            mode: draft.mode,
            offset: draft.offset,
            absolute: draft.absolute,
            freezeAt: draft.freezeAt,
            epoch: selected.view.epoch ?? "",
            leap: draft.leap,
            stratum: Number(draft.stratum),
            refid: draft.refid,
          },
        },
        st.hostTime,
      );
      if ("error" in result) {
        setMathError(result.error);
        return;
      }
      setMath(result);
    } catch (err) {
      setMathError(err instanceof APIError ? err.message : "Could not read host time.");
    }
  }

  if (items === null && error === "") {
    return (
      <main className="page page--workspace">
        <p role="status">Loading filters…</p>
      </main>
    );
  }

  const field = draft ? modeField(draft.mode) : null;
  const busy = selected !== null && busyName === selected.name;

  return (
    <main className="page page--workspace">
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="filters-workspace">
        <section className="inventory" aria-labelledby="filters-heading">
          <div className="inventory-head">
            <p className="kicker">Filters</p>
            <h1 id="filters-heading">{(items ?? []).length} · list order</h1>
            <p className="muted">
              First <strong>enabled</strong> CIDR hit wins. Longest prefix does not. Catch-all stays last.
            </p>
          </div>
          <div>
            {(items ?? []).map((f, i) => (
              <button
                key={f.name}
                type="button"
                className={f.name === selectedName ? "filter-row is-selected" : "filter-row"}
                onClick={() => setSelectedName(f.name)}
              >
                <span className="ordinal">{padOrdinal(i + 1)}</span>
                <span>
                  <span className="filter-name">{f.name}</span>
                  <br />
                  <code>{(f.match.cidrs ?? []).join(" ")}</code>
                  <br />
                  <span className="chip chip--mode">{f.view.mode}</span>
                  <span className={f.enabled ? "chip chip--ok" : "chip"}>{f.enabled ? "ENABLED" : "disabled"}</span>
                </span>
              </button>
            ))}
          </div>
        </section>
        <section className="inspector" aria-labelledby="inspector-heading">
          {selected && draft ? (
            <>
              <div className="inspector-head">
                <div>
                  <h1 id="inspector-heading">{selected.name}</h1>
                  <p className="muted">
                    PUT /v1/filters/{selected.name} · expectedRevision from GET /v1/state
                  </p>
                </div>
                <div className="inspector-actions">
                  <button
                    type="button"
                    className="ghost"
                    disabled={!canWrite || busy}
                    onClick={() => void onToggle(!selected.enabled)}
                  >
                    {selected.enabled ? "Disable" : "Enable"}
                  </button>
                  <button type="button" className="primary" disabled={!canWrite || busy} onClick={() => void onSave()}>
                    Save view
                  </button>
                </div>
              </div>
              <h2 className="kicker">Clock</h2>
              <label>
                <input
                  type="checkbox"
                  aria-label={`Enable ${selected.name}`}
                  checked={draft.enabled}
                  disabled={!canWrite || busy}
                  onChange={(e) => void onToggle(e.target.checked)}
                />{" "}
                SERVE THIS VIEW
              </label>
              <div className="clock-grid">
                <div className="field">
                  <label htmlFor="filter-mode">Mode</label>
                  <select
                    id="filter-mode"
                    value={draft.mode}
                    disabled={!canWrite}
                    onChange={(e) => setDraft({ ...draft, mode: e.target.value })}
                  >
                    {MODES.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                </div>
                {field === "offset" ? (
                  <div className="field">
                    <label htmlFor="filter-offset">Offset</label>
                    <input
                      id="filter-offset"
                      value={draft.offset}
                      disabled={!canWrite}
                      autoComplete="off"
                      spellCheck={false}
                      onChange={(e) => setDraft({ ...draft, offset: e.target.value })}
                    />
                  </div>
                ) : null}
                {field === "absolute" ? (
                  <div className="field">
                    <label htmlFor="filter-absolute">Absolute</label>
                    <input
                      id="filter-absolute"
                      value={draft.absolute}
                      disabled={!canWrite}
                      autoComplete="off"
                      spellCheck={false}
                      onChange={(e) => setDraft({ ...draft, absolute: e.target.value })}
                    />
                  </div>
                ) : null}
                {field === "freezeAt" ? (
                  <div className="field">
                    <label htmlFor="filter-freezeAt">freezeAt</label>
                    <input
                      id="filter-freezeAt"
                      value={draft.freezeAt}
                      disabled={!canWrite}
                      autoComplete="off"
                      spellCheck={false}
                      onChange={(e) => setDraft({ ...draft, freezeAt: e.target.value })}
                    />
                  </div>
                ) : null}
                {field === "rate" ? (
                  <div className="field">
                    <label htmlFor="filter-rate">Rate</label>
                    <input
                      id="filter-rate"
                      value={draft.rate}
                      disabled={!canWrite}
                      autoComplete="off"
                      spellCheck={false}
                      onChange={(e) => setDraft({ ...draft, rate: e.target.value })}
                    />
                  </div>
                ) : null}
                <div className="field">
                  <label htmlFor="filter-leap">Leap</label>
                  <select
                    id="filter-leap"
                    value={draft.leap}
                    disabled={!canWrite}
                    onChange={(e) => setDraft({ ...draft, leap: e.target.value })}
                  >
                    <option value="none">none</option>
                    <option value="insert">insert</option>
                    <option value="delete">delete</option>
                    <option value="unsync">unsync</option>
                  </select>
                </div>
                <div className="field">
                  <label htmlFor="filter-stratum">Stratum</label>
                  <input
                    id="filter-stratum"
                    value={draft.stratum}
                    disabled={!canWrite}
                    autoComplete="off"
                    onChange={(e) => setDraft({ ...draft, stratum: e.target.value })}
                  />
                </div>
                <div className="field">
                  <label htmlFor="filter-refid">refid</label>
                  <input
                    id="filter-refid"
                    value={draft.refid}
                    disabled={!canWrite}
                    autoComplete="off"
                    spellCheck={false}
                    onChange={(e) => setDraft({ ...draft, refid: e.target.value })}
                  />
                </div>
              </div>
              <p className="kicker">Match CIDRs</p>
              <p>
                {(selected.match.cidrs ?? []).map((c) => (
                  <span key={c} className="chip">
                    {c}
                  </span>
                ))}
              </p>
              <h2 className="kicker">What this client would see</h2>
              <p className="muted">
                Selected-filter math only. Does not send an NTP packet. Compiled match walk is Preview. Absolute and
                rate elapsed use compile epochMono, which is not on the Filter DTO.
              </p>
              <div className="row">
                <div className="field field--grow">
                  <label htmlFor="filter-sample-ip">Sample IP</label>
                  <input
                    id="filter-sample-ip"
                    className="preview-input"
                    value={sampleIP}
                    autoComplete="off"
                    spellCheck={false}
                    onChange={(e) => setSampleIP(e.target.value)}
                  />
                </div>
                <button type="button" className="primary" onClick={() => void onInPaneMath()}>
                  Approximate
                </button>
              </div>
              {mathError !== "" ? (
                <p className="banner-error" role="alert">
                  {mathError}
                </p>
              ) : null}
              {math ? (
                <dl>
                  <div>
                    <dt>Filter</dt>
                    <dd>{math.filter}</dd>
                  </div>
                  <div>
                    <dt>Served time</dt>
                    <dd className="served">{math.servedTime}</dd>
                  </div>
                  <div>
                    <dt>Host time</dt>
                    <dd>
                      <code>{math.hostTime}</code>
                    </dd>
                  </div>
                  <div>
                    <dt>Mode</dt>
                    <dd>
                      <span className="chip chip--mode">{math.mode}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>Leap</dt>
                    <dd>
                      <span className="chip">{math.leap}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>Stratum</dt>
                    <dd>
                      <span className="chip">{math.stratum ?? "—"}</span>
                    </dd>
                  </div>
                  <div>
                    <dt>refid</dt>
                    <dd>
                      <code>{math.refid}</code>
                    </dd>
                  </div>
                  <div>
                    <dt>Offset from host</dt>
                    <dd>{math.offsetFromHost}</dd>
                  </div>
                </dl>
              ) : null}
            </>
          ) : (
            <p className="muted">Select a filter. The list is empty.</p>
          )}
        </section>
      </div>
    </main>
  );
}
