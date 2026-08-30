import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { FeaturesPage } from "./FeaturesPage";

const FROZEN = [
  { id: "filters", apply: "live", path: "spec.filters" },
  { id: "views", apply: "live", path: "spec.filters[].view" },
  { id: "restrict", apply: "live", path: "spec.ntp.restrict" },
  { id: "admission", apply: "live", path: "spec.ntp.admission" },
  { id: "allowClientCidrs", apply: "live", path: "spec.ntp.allowClientCidrs" },
  { id: "queryLog", apply: "live", path: "spec.ntp.queryLog" },
  { id: "management.http", apply: "live", path: "spec.management.bodyLimit|requestsPerSecond|burst|maxConcurrent" },
  { id: "listeners.ntp.address", apply: "reset-only", path: "spec.listeners.ntp.address" },
  { id: "listeners.management.address", apply: "reset-only", path: "spec.listeners.management.address" },
  { id: "ntp.nts", apply: "reset-only", path: "spec.ntp.nts" },
  { id: "ntp.symmetricKeys", apply: "reset-only", path: "spec.ntp.symmetricKeys" },
  { id: "auth", apply: "reset-only", path: "spec.auth" },
];

describe("FeaturesPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders live and reset-only chips for the frozen twelve ids", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/features")) {
          return json(200, { items: FROZEN });
        }
        return json(404, {
          status: 404,
          title: "not found",
          detail: "not found",
          code: "not_found",
          type: "urn:labntp:error:not-found",
        });
      }),
    );
    renderApp(<FeaturesPage />, { route: "/features" });
    expect((await screen.findAllByText("live")).length).toBeGreaterThan(0);
    expect(screen.getAllByText("reset-only").length).toBeGreaterThan(0);
    expect(FROZEN).toHaveLength(12);
    expect(FROZEN.map((f) => f.id)).not.toContain("ui");
    expect(screen.queryByText("ui")).toBeNull();
    expect(
      screen.getByText(/UI enablement is bootstrap YAML; reread with Reset. Not a features.list id./),
    ).toBeInTheDocument();
  });
});
