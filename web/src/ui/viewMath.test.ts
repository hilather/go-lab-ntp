import { describe, expect, it } from "vitest";
import { computeSelectedViewMath, firstCidrHost, formatDuration, parseDuration } from "./viewMath";

describe("formatDuration", () => {
  it("matches config.FormatDuration", () => {
    expect(formatDuration(0)).toBe("0s");
    expect(formatDuration(-6 * 60 * 1_000_000_000)).toBe("-6m");
    expect(formatDuration(5 * 1_000_000_000)).toBe("5s");
    expect(formatDuration(-6 * 60 * 1_000_000_000)).not.toBe("-6m0s");
  });
});

describe("parseDuration", () => {
  it("parses Go duration strings", () => {
    expect(parseDuration("-6m")).toBe(-6 * 60 * 1_000_000_000);
    expect(parseDuration("0s")).toBe(0);
    expect(parseDuration("2h45m")).toBe((2 * 3600 + 45 * 60) * 1_000_000_000);
    expect(parseDuration("")).toBeNull();
    expect(parseDuration("nope")).toBeNull();
  });
});

describe("firstCidrHost", () => {
  it("uses the address before slash", () => {
    expect(firstCidrHost("10.99.42.20/32")).toBe("10.99.42.20");
    expect(firstCidrHost("0.0.0.0/0")).toBe("0.0.0.0");
    expect(firstCidrHost("::/0")).toBe("::");
  });
});

describe("computeSelectedViewMath", () => {
  const host = "2026-09-02T18:16:42.000Z";

  it("applies offset from host", () => {
    const got = computeSelectedViewMath(
      {
        name: "tester-a-kerberos",
        view: { mode: "offset", offset: "-6m", leap: "none", stratum: 1, refid: "LOCL" },
      },
      host,
    );
    if ("error" in got) {
      throw new Error(got.error);
    }
    expect(got.servedTime).toBe("2026-09-02T18:10:42Z");
    expect(got.offsetFromHost).toBe("-6m");
    expect(got.filter).toBe("tester-a-kerberos");
  });

  it("follow-real equals host", () => {
    const got = computeSelectedViewMath(
      { name: "default", view: { mode: "follow-real", offset: "0s", leap: "none", stratum: 2, refid: "GPS" } },
      host,
    );
    if ("error" in got) {
      throw new Error(got.error);
    }
    expect(got.servedTime).toBe("2026-09-02T18:16:42Z");
    expect(got.offsetFromHost).toBe("0s");
  });

  it("freeze uses freezeAt", () => {
    const got = computeSelectedViewMath(
      {
        name: "frozen",
        view: { mode: "freeze", freezeAt: "2020-01-01T00:00:00Z", leap: "none", stratum: 1, refid: "LOCL" },
      },
      host,
    );
    if ("error" in got) {
      throw new Error(got.error);
    }
    expect(got.servedTime).toBe("2020-01-01T00:00:00Z");
  });

  it("absolute uses absolute only (elapsed 0)", () => {
    const got = computeSelectedViewMath(
      {
        name: "tester-b-expired-cert",
        view: { mode: "absolute", absolute: "2035-01-01T00:00:00Z", leap: "none", stratum: 1, refid: "LOCL" },
      },
      host,
    );
    if ("error" in got) {
      throw new Error(got.error);
    }
    expect(got.servedTime).toBe("2035-01-01T00:00:00Z");
  });

  it("does not treat rate epoch as a wall subtractend", () => {
    const got = computeSelectedViewMath(
      {
        name: "racer",
        view: { mode: "rate", epoch: "2035-01-01T00:00:00Z", leap: "none", stratum: 1, refid: "LOCL" },
      },
      host,
    );
    if ("error" in got) {
      throw new Error(got.error);
    }
    expect(got.servedTime).toBe("2035-01-01T00:00:00Z");
    expect(got.offsetFromHost).not.toMatch(/^-9/);
  });
});
