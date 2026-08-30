import { render, type RenderOptions } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type { ReactElement, ReactNode } from "react";
import { clearMemoryCSRF, setMemoryCSRF } from "../api/client";
import type { SessionView } from "../api/types";
import { AuthProvider } from "../auth/AuthProvider";

export const ALL_SCOPES = ["ntp.read", "ntp.write", "ntp.admin", "ntp.audit.read"];

export function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": status >= 400 ? "application/problem+json" : "application/json" },
  });
}

export function sessionView(scopes: string[] = ALL_SCOPES): SessionView {
  return {
    id: "admin",
    role: "administrator",
    scopes,
    csrf: "csrf-test",
    expiresAt: "2099-01-01T00:00:00Z",
  };
}

export function seedCSRF(): void {
  setMemoryCSRF("csrf-test");
}

export function renderApp(ui: ReactElement, options?: Omit<RenderOptions, "wrapper"> & { route?: string }) {
  const route = options?.route ?? "/";
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter initialEntries={[route]}>
        <AuthProvider>{children}</AuthProvider>
      </MemoryRouter>
    );
  }
  return render(ui, { wrapper: Wrapper, ...options });
}

export function resetClientState(): void {
  clearMemoryCSRF();
  localStorage.clear();
  sessionStorage.clear();
}
