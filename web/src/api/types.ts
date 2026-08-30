export type Problem = {
  type: string;
  title: string;
  status: number;
  detail: string;
  code: string;
};

export type SessionCreated = {
  csrf: string;
  expiresAt: string;
};

export type SessionView = {
  id: string;
  role: string;
  scopes: string[];
  csrf?: string;
  expiresAt?: string;
};

export type Match = {
  cidrs: string[];
};

export type View = {
  mode: string;
  offset: string;
  leap: string;
  stratum: number;
  refid: string;
  precision: number;
  rootDelay: string;
  rootDispersion: string;
  jitter: string;
  absolute?: string;
  freezeAt?: string;
  epoch?: string;
  rate?: number;
  minpoll?: number;
  maxpoll?: number;
};

export type Filter = {
  name: string;
  enabled: boolean;
  match: Match;
  view: View;
};

export type FilterList = {
  items: Filter[];
};

export type PutFilterBody = {
  expectedRevision: string;
  reason?: string;
  idempotencyKey?: string;
  filter: Filter;
};

export type Preview = {
  ip: string;
  filter: string;
  servedTime: string | null;
  hostTime: string;
  mode?: string;
  leap?: string;
  stratum?: number;
  refid?: string;
  offsetFromHost?: string;
  reason?: string;
};

export type QueryEntry = {
  clientIP: string;
  filter: string;
  servedTime?: string;
  leap?: string;
  mode?: string;
  vn: number;
  whenHost?: string;
};

export type QueryList = {
  items: QueryEntry[];
  nextCursor?: string;
};

export type Feature = {
  id: string;
  apply: "live" | "reset-only" | string;
  path: string;
};

export type FeatureList = {
  items: Feature[];
};

export type Listener = {
  name: string;
  address: string;
};

/** Nested status revisions/warnings use Go field names (no json tags). */
export type StatusWarning = {
  Code?: string;
  Message?: string;
};

export type Status = {
  ready: boolean;
  hostTime: string;
  listeners: Listener[];
  revisions?: Record<string, unknown>;
  warnings?: StatusWarning[];
};

export type StateView = {
  bootstrapRevision: string;
  runtimeRevision: string;
  generation: number;
  drifted: boolean;
  loadedAt?: string;
  canonical?: unknown;
};
