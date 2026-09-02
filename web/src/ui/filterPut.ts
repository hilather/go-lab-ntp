import type { Filter, View } from "../api/types";
import { parseDuration } from "./viewMath";

export const MODES = ["follow-real", "offset", "absolute", "freeze", "rate"] as const;

export type FilterDraft = {
  enabled: boolean;
  mode: string;
  offset: string;
  absolute: string;
  freezeAt: string;
  rate: string;
  leap: string;
  stratum: string;
  refid: string;
};

export function draftFromFilter(f: Filter): FilterDraft {
  return {
    enabled: f.enabled,
    mode: f.view.mode,
    offset: f.view.offset ?? "0s",
    absolute: f.view.absolute ?? "",
    freezeAt: f.view.freezeAt ?? "",
    rate: f.view.rate === undefined ? "" : String(f.view.rate),
    leap: f.view.leap || "none",
    stratum: String(f.view.stratum),
    refid: f.view.refid ?? "",
  };
}

export function cloneFilter(f: Filter): Filter {
  return {
    name: f.name,
    enabled: f.enabled,
    match: { cidrs: [...(f.match.cidrs ?? [])] },
    view: { ...f.view },
  };
}

function stripModeKeys(view: View): View {
  const next: View = { ...view };
  delete next.absolute;
  delete next.freezeAt;
  delete next.rate;
  delete next.epoch;
  next.offset = "0s";
  return next;
}

export function buildToggleFilter(last: Filter, enabled: boolean): Filter {
  const out = cloneFilter(last);
  out.enabled = enabled;
  return out;
}

export function buildSaveFilter(last: Filter, draft: FilterDraft): { ok: true; filter: Filter } | { ok: false; error: string } {
  const mode = draft.mode;
  if (!MODES.includes(mode as (typeof MODES)[number])) {
    return { ok: false, error: "mode must be follow-real, offset, absolute, freeze, or rate." };
  }
  const stratum = Number(draft.stratum);
  if (!Number.isInteger(stratum) || stratum < 1 || stratum > 16) {
    return { ok: false, error: "stratum must be 1–16." };
  }
  const view = stripModeKeys(last.view);
  view.mode = mode;
  view.leap = draft.leap || "none";
  view.stratum = stratum;
  view.refid = draft.refid;

  switch (mode) {
    case "follow-real":
      view.offset = "0s";
      break;
    case "offset": {
      const o = draft.offset.trim();
      if (o === "") {
        return { ok: false, error: "offset is required." };
      }
      if (parseDuration(o) === null) {
        return { ok: false, error: "offset must use Go duration syntax (for example -6m)." };
      }
      view.offset = o;
      break;
    }
    case "absolute": {
      const a = draft.absolute.trim();
      if (a === "") {
        return { ok: false, error: "absolute RFC3339 is required." };
      }
      view.absolute = a;
      break;
    }
    case "freeze": {
      const at = draft.freezeAt.trim();
      if (at === "") {
        return { ok: false, error: "freezeAt RFC3339 is required." };
      }
      view.freezeAt = at;
      break;
    }
    case "rate": {
      const raw = draft.rate.trim();
      if (raw === "") {
        return { ok: false, error: "rate is required." };
      }
      const n = Number(raw);
      if (!Number.isFinite(n)) {
        return { ok: false, error: "rate must be a finite number." };
      }
      view.rate = n;
      if (typeof last.view.epoch === "string" && last.view.epoch.trim() !== "") {
        view.epoch = last.view.epoch;
      }
      break;
    }
    default:
      break;
  }

  return {
    ok: true,
    filter: {
      ...cloneFilter(last),
      enabled: draft.enabled,
      view,
    },
  };
}

export function modeField(mode: string): "offset" | "absolute" | "freezeAt" | "rate" | null {
  switch (mode) {
    case "offset":
      return "offset";
    case "absolute":
      return "absolute";
    case "freeze":
      return "freezeAt";
    case "rate":
      return "rate";
    default:
      return null;
  }
}
