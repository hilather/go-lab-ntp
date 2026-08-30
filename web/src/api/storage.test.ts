import { describe, expect, it } from "vitest";
import { assertNoTokenStorage } from "./storage";

describe("assertNoTokenStorage", () => {
  it("rejects a token key in localStorage", () => {
    localStorage.setItem("token", "abc");
    expect(() => assertNoTokenStorage()).toThrow(/token key/i);
    localStorage.clear();
  });

  it("rejects a bearer stored in localStorage", () => {
    localStorage.setItem("note", "Authorization: Bearer abc");
    expect(() => assertNoTokenStorage()).toThrow(/bearer/i);
    localStorage.clear();
  });

  it("allows empty storage", () => {
    localStorage.clear();
    sessionStorage.clear();
    expect(() => assertNoTokenStorage()).not.toThrow();
  });
});
