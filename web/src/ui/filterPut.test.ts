import { describe, expect, it } from "vitest";
import type { Filter } from "../api/types";
import { buildSaveFilter, buildToggleFilter, draftFromFilter } from "./filterPut";

const followReal: Filter = {
  name: "default",
  enabled: true,
  match: { cidrs: ["0.0.0.0/0", "::/0"] },
  view: {
    mode: "follow-real",
    offset: "0s",
    leap: "none",
    stratum: 2,
    refid: "GPS",
    precision: -20,
    rootDelay: "0s",
    rootDispersion: "0s",
    jitter: "0s",
  },
};

const offset: Filter = {
  name: "tester-a-kerberos",
  enabled: true,
  match: { cidrs: ["10.99.42.20/32"] },
  view: {
    mode: "offset",
    offset: "-6m",
    leap: "none",
    stratum: 1,
    refid: "LOCL",
    precision: -20,
    rootDelay: "0s",
    rootDispersion: "0s",
    jitter: "0s",
  },
};

const absolute: Filter = {
  name: "tester-b-expired-cert",
  enabled: true,
  match: { cidrs: ["10.99.42.30/32"] },
  view: {
    mode: "absolute",
    offset: "0s",
    absolute: "2035-01-01T00:00:00Z",
    leap: "none",
    stratum: 1,
    refid: "LOCL",
    precision: -20,
    rootDelay: "0s",
    rootDispersion: "0s",
    jitter: "0s",
  },
};

const rate: Filter = {
  name: "racer",
  enabled: true,
  match: { cidrs: ["10.99.42.40/32"] },
  view: {
    mode: "rate",
    offset: "0s",
    rate: 2,
    epoch: "2035-01-01T00:00:00Z",
    leap: "none",
    stratum: 1,
    refid: "LOCL",
    precision: -20,
    rootDelay: "0s",
    rootDispersion: "0s",
    jitter: "0s",
  },
};

describe("buildToggleFilter", () => {
  it("flips only enabled", () => {
    const next = buildToggleFilter(offset, false);
    expect(next.enabled).toBe(false);
    expect(next.view.offset).toBe("-6m");
    expect(next.view.mode).toBe("offset");
  });
});

describe("buildSaveFilter", () => {
  it("keeps offset string on unchanged save", () => {
    const got = buildSaveFilter(offset, draftFromFilter(offset));
    if (!got.ok) {
      throw new Error(got.error);
    }
    expect(got.filter.view.offset).toBe("-6m");
    expect(typeof got.filter.view.offset).toBe("string");
    expect(got.filter.view.rate).toBeUndefined();
  });

  it("drops leftover offset when switching to follow-real", () => {
    const draft = { ...draftFromFilter(offset), mode: "follow-real" };
    const got = buildSaveFilter(offset, draft);
    if (!got.ok) {
      throw new Error(got.error);
    }
    expect(got.filter.view.mode).toBe("follow-real");
    expect(got.filter.view.offset).toBe("0s");
    expect(got.filter.view.absolute).toBeUndefined();
    expect(got.filter.view.freezeAt).toBeUndefined();
    expect(got.filter.view.rate).toBeUndefined();
    expect(got.filter.view.epoch).toBeUndefined();
  });

  it("drops absolute when switching to follow-real", () => {
    const draft = { ...draftFromFilter(absolute), mode: "follow-real" };
    const got = buildSaveFilter(absolute, draft);
    if (!got.ok) {
      throw new Error(got.error);
    }
    expect(got.filter.view.absolute).toBeUndefined();
  });

  it("drops epoch and rate when leaving rate mode", () => {
    const draft = { ...draftFromFilter(rate), mode: "follow-real" };
    const got = buildSaveFilter(rate, draft);
    if (!got.ok) {
      throw new Error(got.error);
    }
    expect(got.filter.view.epoch).toBeUndefined();
    expect(got.filter.view.rate).toBeUndefined();
  });

  it("keeps rate epoch on leap-only save", () => {
    const draft = { ...draftFromFilter(rate), leap: "insert" };
    const got = buildSaveFilter(rate, draft);
    if (!got.ok) {
      throw new Error(got.error);
    }
    expect(got.filter.view.epoch).toBe("2035-01-01T00:00:00Z");
    expect(got.filter.view.rate).toBe(2);
    expect(got.filter.view.leap).toBe("insert");
  });

  it("rejects empty rate instead of sending 0", () => {
    const draft = { ...draftFromFilter(followReal), mode: "rate", rate: "" };
    const got = buildSaveFilter(followReal, draft);
    expect(got.ok).toBe(false);
  });
});
