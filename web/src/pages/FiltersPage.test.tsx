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

const fullYamlOrder: Filter[] = [
  {
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
  },
  {
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
  },
  followReal,
];

function problem(status: number, code: string, detail: string) {
  return json(status, {
    status,
    title: code,
    detail,
    code,
    type: `urn:labntp:error:${code.replaceAll("_", "-")}`,
  });
}

function statusOK() {
  return json(200, { ready: true, hostTime: "2026-09-02T18:16:42Z", listeners: [] });
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
      reason: string;
      filter: Filter;
    };
    expect(body.expectedRevision).toBe("sha256:runtime");
    expect(body.reason).toBe("ui: disable filter");
    expect(body.filter.enabled).toBe(false);
    expect(body.filter.view.offset).toBe("0s");
    expect(body.filter.view.rootDelay).toBe("0s");
    expect(typeof body.filter.view.offset).toBe("string");
    expect("rate" in body.filter.view).toBe(false);
  });

  it("disables write controls for ntp.read viewers", async () => {
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
    expect(screen.getByRole("button", { name: /Save view/i })).toBeDisabled();
    expect(screen.getByLabelText(/^Mode$/i)).toBeDisabled();
  });

  it("renders list order with ordinals and catch-all last", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/filters")) {
          return json(200, { items: fullYamlOrder });
        }
        return problem(404, "not_found", "not found");
      }),
    );
    renderApp(<FiltersPage />, { route: "/" });
    expect(await screen.findByText("tester-a-kerberos")).toBeInTheDocument();
    expect(screen.getByText("01")).toBeInTheDocument();
    expect(screen.getByText("02")).toBeInTheDocument();
    expect(screen.getByText("03")).toBeInTheDocument();
    const names = screen.getAllByText(/tester-a-kerberos|tester-b-expired-cert|^default$/).map((n) => n.textContent);
    expect(names.indexOf("tester-a-kerberos")).toBeLessThan(names.indexOf("tester-b-expired-cert"));
    expect(names.indexOf("tester-b-expired-cert")).toBeLessThan(names.indexOf("default"));
    expect(screen.getByText(/First/i).textContent).toMatch(/enabled/i);
    expect(screen.getByText(/Longest prefix does not/i)).toBeInTheDocument();
  });

  it("shows only the mode-conditional field for the selected mode", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/filters")) {
          return json(200, { items: fullYamlOrder });
        }
        return problem(404, "not_found", "not found");
      }),
    );
    renderApp(<FiltersPage />, { route: "/" });
    expect(await screen.findByLabelText(/^Offset$/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/^Absolute$/i)).toBeNull();
    expect(screen.queryByLabelText(/^freezeAt$/i)).toBeNull();
    expect(screen.queryByLabelText(/^Rate$/i)).toBeNull();

    await user.click(screen.getByRole("button", { name: /tester-b-expired-cert/i }));
    expect(await screen.findByLabelText(/^Absolute$/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/^Offset$/i)).toBeNull();

    await user.selectOptions(screen.getByLabelText(/^Mode$/i), "freeze");
    expect(screen.getByLabelText(/^freezeAt$/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/^Absolute$/i)).toBeNull();

    await user.selectOptions(screen.getByLabelText(/^Mode$/i), "rate");
    expect(screen.getByLabelText(/^Rate$/i)).toBeInTheDocument();
    expect(screen.queryByLabelText(/^freezeAt$/i)).toBeNull();

    await user.selectOptions(screen.getByLabelText(/^Mode$/i), "follow-real");
    expect(screen.queryByLabelText(/^Offset$/i)).toBeNull();
    expect(screen.queryByLabelText(/^Absolute$/i)).toBeNull();
    expect(screen.queryByLabelText(/^Rate$/i)).toBeNull();
  });

  it("saves a view with expectedRevision and ui: save view", async () => {
    const user = userEvent.setup();
    seedCSRF();
    const puts: Array<{ reason: string; filter: Filter; expectedRevision: string }> = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = (init?.method ?? "GET").toUpperCase();
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/filters") && method === "GET") {
          return json(200, { items: [fullYamlOrder[0]] });
        }
        if (url.endsWith("/v1/state")) {
          return json(200, {
            bootstrapRevision: "sha256:boot",
            runtimeRevision: "sha256:runtime",
            generation: 1,
            drifted: false,
          });
        }
        if (url.includes("/v1/filters/") && method === "PUT") {
          puts.push(JSON.parse(String(init?.body)) as { reason: string; filter: Filter; expectedRevision: string });
          return json(200, { applied: true });
        }
        return problem(404, "not_found", "not found");
      }),
    );
    renderApp(<FiltersPage />, { route: "/" });
    await screen.findByLabelText(/Enable tester-a-kerberos/i);
    await user.click(screen.getByRole("button", { name: /Save view/i }));
    await waitFor(() => {
      expect(puts).toHaveLength(1);
    });
    expect(puts[0]?.reason).toBe("ui: save view");
    expect(puts[0]?.expectedRevision).toBe("sha256:runtime");
    expect(puts[0]?.filter.view.offset).toBe("-6m");
    expect(typeof puts[0]?.filter.view.offset).toBe("string");
  });

  it("in-pane approximate does not call views/preview", async () => {
    const user = userEvent.setup();
    const urls: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        urls.push(url);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.endsWith("/v1/filters")) {
          return json(200, { items: [fullYamlOrder[0]] });
        }
        if (url.endsWith("/v1/status")) {
          return statusOK();
        }
        return problem(404, "not_found", "not found");
      }),
    );
    renderApp(<FiltersPage />, { route: "/" });
    await screen.findByLabelText(/Enable tester-a-kerberos/i);
    await user.click(screen.getByRole("button", { name: /^Approximate$/i }));
    expect(await screen.findByText("2026-09-02T18:10:42Z")).toBeInTheDocument();
    expect(urls.some((u) => u.includes("/v1/views/preview"))).toBe(false);
    expect(urls.some((u) => u.includes("/v1/status"))).toBe(true);
  });
});
