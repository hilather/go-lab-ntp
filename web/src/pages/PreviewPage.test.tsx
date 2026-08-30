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
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
