import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CSRF_HEADER } from "../api/client";
import type { Filter } from "../api/types";
import { json, renderApp, resetClientState, seedCSRF, sessionView } from "../test/render";
import { FiltersPage } from "./FiltersPage";

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

function problem(status: number, code: string, detail: string) {
  return json(status, {
    status,
    title: code,
    detail,
    code,
    type: `urn:labntp:error:${code.replaceAll("_", "-")}`,
  });
}

describe("FiltersPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("toggles enabled with runtimeRevision, CSRF, and duration strings", async () => {
    const user = userEvent.setup();
    seedCSRF();
    const puts: Array<{ url: string; init: RequestInit | undefined }> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session") && method === "GET") {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/filters") && method === "GET") {
          return json(200, { items: [followReal] });
        }
        if (url.endsWith("/v1/state") && method === "GET") {
          return json(200, {
            bootstrapRevision: "sha256:boot",
            runtimeRevision: "sha256:runtime",
            generation: 1,
            drifted: false,
          });
        }
        if (url.includes("/v1/filters/") && method === "PUT") {
          puts.push({ url, init });
          return json(200, { applied: true, runtimeRevision: "sha256:next" });
        }
        return problem(404, "not_found", "not found");
      }),
    );

    renderApp(<FiltersPage />, { route: "/" });
    const box = await screen.findByRole("checkbox", { name: /Enable default/i });
    expect(box).toBeEnabled();
    await user.click(box);

    await waitFor(() => {
      expect(puts).toHaveLength(1);
    });
    const init = puts[0]?.init;
    if (!init) {
      throw new Error("expected PUT");
    }
    expect(new Headers(init.headers).get(CSRF_HEADER)).toBe("csrf-test");
    const body = JSON.parse(String(init.body)) as {
      expectedRevision: string;
      filter: Filter;
    };
    expect(body.expectedRevision).toBe("sha256:runtime");
    expect(body.filter.enabled).toBe(false);
    expect(body.filter.view.offset).toBe("0s");
    expect(body.filter.view.rootDelay).toBe("0s");
    expect(typeof body.filter.view.offset).toBe("string");
    expect("rate" in body.filter.view).toBe(false);
  });

  it("disables the checkbox for ntp.read viewers", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session") && method === "GET") {
          return json(200, sessionView(["ntp.read"]));
        }
        if (url.endsWith("/v1/filters") && method === "GET") {
          return json(200, { items: [followReal] });
        }
        return problem(404, "not_found", "not found");
      }),
    );
    renderApp(<FiltersPage />, { route: "/" });
    const box = await screen.findByRole("checkbox", { name: /Enable default/i });
    expect(box).toBeDisabled();
  });
});
