import { describe, expect, it } from "vitest";
import { navItems } from "./nav";
import { canSubmitReset } from "./reset";

describe("operator nav", () => {
  it("omits Reset without ntp.admin", () => {
    expect(navItems(false).map((i) => i.label)).toEqual([
      "Filters",
      "Preview",
      "Queries",
      "Features",
      "Status",
    ]);
    expect(navItems(true).map((i) => i.label)).toContain("Reset");
    expect(navItems(true).some((i) => i.label === "Audit")).toBe(false);
    expect(navItems(false).filter((i) => i.group === "CLOCKS").map((i) => i.label)).toEqual([
      "Filters",
      "Preview",
      "Queries",
    ]);
    expect(navItems(false).filter((i) => i.group === "LAB").map((i) => i.label)).toEqual(["Features", "Status"]);
  });

  it("gates reset on the exact phrase and confirmation", () => {
    expect(canSubmitReset("RESET", true, true)).toBe(true);
    expect(canSubmitReset("reset", true, true)).toBe(false);
    expect(canSubmitReset("RESET", false, true)).toBe(false);
    expect(canSubmitReset("RESET", true, false)).toBe(false);
  });
});
