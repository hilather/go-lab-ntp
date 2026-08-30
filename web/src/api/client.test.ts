import { afterEach, describe, expect, it, vi } from "vitest";
import { CSRF_HEADER, apiFetch, bearerAuthorization, clearMemoryCSRF, createSession, setMemoryCSRF } from "./client";
import { resetClientState } from "../test/render";

describe("API client", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("exchanges a bearer for a session without writing web storage", async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(async () => {
      return new Response(JSON.stringify({ csrf: "csrf-1", expiresAt: "2099-01-01T00:00:00Z" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    const created = await createSession(bearerAuthorization("lab-bootstrap-token-32-bytes!!!"));
    expect(created.csrf).toBe("csrf-1");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const init = fetchMock.mock.calls[0]?.[1];
    if (!init) {
      throw new Error("expected fetch init");
    }
    expect(init.credentials).toBe("same-origin");
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer lab-bootstrap-token-32-bytes!!!");
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("copies the in-memory CSRF secret onto mutating requests", async () => {
    setMemoryCSRF("csrf-test");
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(
      async () => new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await apiFetch("/v1/session", { method: "DELETE" });
    const init = fetchMock.mock.calls[0]?.[1];
    if (!init) {
      throw new Error("expected fetch init");
    }
    expect(new Headers(init.headers).get(CSRF_HEADER)).toBe("csrf-test");
  });

  it("does not send CSRF on GET", async () => {
    setMemoryCSRF("csrf-test");
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>(
      async () => new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    await apiFetch("/v1/filters");
    const init = fetchMock.mock.calls[0]?.[1];
    if (!init) {
      throw new Error("expected fetch init");
    }
    expect(new Headers(init.headers).has(CSRF_HEADER)).toBe(false);
    clearMemoryCSRF();
  });
});
