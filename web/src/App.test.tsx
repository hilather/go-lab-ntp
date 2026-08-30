import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { json, resetClientState, sessionView } from "./test/render";

describe("App nav", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("hides the Reset link without ntp.admin", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView(["ntp.read"]));
        }
        if (url.endsWith("/v1/filters")) {
          return json(200, { items: [] });
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
    render(<App />);
    expect(await screen.findByRole("link", { name: "Filters" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Reset" })).toBeNull();
  });
});
