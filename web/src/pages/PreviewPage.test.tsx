import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { PreviewPage } from "./PreviewPage";

describe("PreviewPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("renders unmatched without throwing", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.includes("/v1/views/preview")) {
          return json(200, {
            ip: "203.0.113.9",
            filter: "",
            servedTime: null,
            hostTime: "2026-08-30T12:00:00Z",
            reason: "unmatched",
          });
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
    renderApp(<PreviewPage />, { route: "/preview" });
    await user.type(await screen.findByLabelText(/IP address/i), "203.0.113.9");
    await user.click(screen.getByRole("button", { name: /^Preview$/i }));
    expect(await screen.findByText(/unmatched/i)).toBeInTheDocument();
    expect(screen.getByText(/Served time is not available/i)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows allowlist reason on 200", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        if (url.includes("/v1/views/preview")) {
          return json(200, {
            ip: "8.8.8.8",
            filter: "",
            servedTime: null,
            hostTime: "2026-08-30T12:00:00Z",
            reason: "allowlist",
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labntp:error:not-found" });
      }),
    );
    renderApp(<PreviewPage />, { route: "/preview" });
    await user.type(await screen.findByLabelText(/IP address/i), "8.8.8.8");
    await user.click(screen.getByRole("button", { name: /^Preview$/i }));
    expect(await screen.findByText(/allowlist/i)).toBeInTheDocument();
    expect(screen.getByText(/Served time is not available/i)).toBeInTheDocument();
  });

  it("requires an IP on empty submit", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input).endsWith("/v1/session")) {
          return json(200, sessionView());
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labntp:error:not-found" });
      }),
    );
    renderApp(<PreviewPage />, { route: "/preview" });
    await user.click(await screen.findByRole("button", { name: /^Preview$/i }));
    expect(await screen.findByText("Enter an IP address.")).toBeInTheDocument();
  });

  it("chip clicks the same GET preview", async () => {
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
        if (url.includes("/v1/views/preview")) {
          return json(200, {
            ip: "10.99.42.20",
            filter: "tester-a-kerberos",
            servedTime: "2026-09-02T18:10:42Z",
            hostTime: "2026-09-02T18:16:42Z",
            mode: "offset",
            leap: "none",
            stratum: 1,
            refid: "LOCL",
            offsetFromHost: "-6m",
          });
        }
        return json(404, { status: 404, title: "not found", detail: "not found", code: "not_found", type: "urn:labntp:error:not-found" });
      }),
    );
    renderApp(<PreviewPage />, { route: "/preview" });
    await user.click(await screen.findByRole("button", { name: "10.99.42.20" }));
    expect(await screen.findByText("tester-a-kerberos")).toBeInTheDocument();
    expect(urls.some((u) => u.includes("/v1/views/preview?ip=10.99.42.20"))).toBe(true);
  });
});
