import type { Filter } from "../api/types";

const NS = 1;
const US = 1_000;
const MS = 1_000_000;
const SEC = 1_000_000_000;
const MIN = 60 * SEC;
const HOUR = 60 * MIN;

const UNIT_NS: Record<string, number> = {
  ns: NS,
  us: US,
  µs: US,
  μs: US,
  ms: MS,
  s: SEC,
  m: MIN,
  h: HOUR,
};

/** Port of config.FormatDuration — not Go Duration.String(). */
export function formatDuration(ns: number): string {
  if (ns === 0) {
    return "0s";
  }
  if (ns < 0) {
    return `-${formatDuration(-ns)}`;
  }
  if (ns % HOUR === 0) {
    return `${ns / HOUR}h`;
  }
  if (ns % MIN === 0) {
    return `${ns / MIN}m`;
  }
  if (ns % SEC === 0) {
    return `${ns / SEC}s`;
  }
  if (ns % MS === 0) {
    return `${ns / MS}ms`;
  }
  if (ns % US === 0) {
    return `${ns / US}us`;
  }
  return `${ns}ns`;
}

/** Go time.ParseDuration subset used on the Filter wire. */
export function parseDuration(input: string): number | null {
  const s = input.trim();
  if (s === "") {
    return null;
  }
  const re = /^([+-]?)(?:(\d+(?:\.\d+)?)(ns|us|µs|μs|ms|s|m|h))+$/;
  if (!re.test(s)) {
    return null;
  }
  const sign = s.startsWith("-") ? -1 : 1;
  const body = s.replace(/^[+-]/, "");
  const part = /(\d+(?:\.\d+)?)(ns|us|µs|μs|ms|s|m|h)/g;
  let ns = 0;
  let m: RegExpExecArray | null;
  while ((m = part.exec(body)) !== null) {
    const n = Number(m[1]);
    const unit = m[2];
    if (!Number.isFinite(n) || unit === undefined) {
      return null;
    }
    const factor = UNIT_NS[unit];
    if (factor === undefined) {
      return null;
    }
    ns += n * factor;
  }
  return sign * ns;
}

export function firstCidrHost(cidr: string): string {
  const slash = cidr.indexOf("/");
  if (slash <= 0) {
    return cidr.trim();
  }
  return cidr.slice(0, slash).trim();
}

export function sampleIPFromFilter(filter: Filter): string {
  const cidr = filter.match.cidrs?.[0] ?? "";
  return firstCidrHost(cidr);
}

export type ViewMathInput = {
  name: string;
  view: {
    mode: string;
    offset?: string;
    absolute?: string;
    freezeAt?: string;
    epoch?: string;
    leap?: string;
    stratum?: number;
    refid?: string;
  };
};

export type ViewMathResult = {
  filter: string;
  servedTime: string;
  hostTime: string;
  mode: string;
  leap: string;
  stratum: number | undefined;
  refid: string;
  offsetFromHost: string;
};

function parseRFC3339(value: string): Date | null {
  const t = Date.parse(value);
  if (Number.isNaN(t)) {
    return null;
  }
  return new Date(t);
}

function toRFC3339Z(d: Date): string {
  return d.toISOString().replace(/\.000Z$/, "Z");
}

/**
 * Selected-filter inspector math. Compiled epochMono is not on the Filter DTO.
 * Do not subtract view.epoch from host (epoch is virtual and rate-only).
 */
export function computeSelectedViewMath(input: ViewMathInput, hostTime: string): ViewMathResult | { error: string } {
  const host = parseRFC3339(hostTime);
  if (!host) {
    return { error: "hostTime is not RFC3339." };
  }
  const mode = input.view.mode;
  let served: Date;
  switch (mode) {
    case "follow-real":
      served = host;
      break;
    case "offset": {
      const off = parseDuration(input.view.offset ?? "");
      if (off === null) {
        return { error: "offset must use Go duration syntax (for example -6m)." };
      }
      served = new Date(host.getTime() + off / 1_000_000);
      break;
    }
    case "freeze": {
      const at = parseRFC3339(input.view.freezeAt ?? "");
      if (!at) {
        return { error: "freezeAt must be RFC3339." };
      }
      served = at;
      break;
    }
    case "absolute": {
      const at = parseRFC3339(input.view.absolute ?? "");
      if (!at) {
        return { error: "absolute must be RFC3339." };
      }
      served = at;
      break;
    }
    case "rate": {
      const epoch = (input.view.epoch ?? "").trim();
      if (epoch !== "") {
        const ev = parseRFC3339(epoch);
        if (!ev) {
          return { error: "epoch must be RFC3339." };
        }
        served = ev;
      } else {
        served = host;
      }
      break;
    }
    default:
      return { error: "mode must be follow-real, offset, absolute, freeze, or rate." };
  }
  const offsetNs = (served.getTime() - host.getTime()) * 1_000_000;
  return {
    filter: input.name,
    servedTime: toRFC3339Z(served),
    hostTime: toRFC3339Z(host),
    mode,
    leap: input.view.leap || "—",
    stratum: input.view.stratum,
    refid: input.view.refid || "—",
    offsetFromHost: formatDuration(offsetNs),
  };
}
