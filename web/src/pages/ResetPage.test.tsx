import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { json, renderApp, resetClientState, sessionView } from "../test/render";
import { ResetPage } from "./ResetPage";

describe("ResetPage", () => {
  afterEach(() => {
    resetClientState();
    vi.unstubAllGlobals();
  });

  it("keeps reset disabled until RESET is typed and confirmed", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input).endsWith("/v1/session")) {
          return json(200, sessionView());
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
    renderApp(<ResetPage />, { route: "/reset" });
    const submit = await screen.findByRole("button", { name: /Reset LabNTP/i });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/Confirmation phrase/i), "RESET");
    expect(submit).toBeDisabled();
    await user.click(screen.getByLabelText(/Wipe the query ring/i));
    expect(submit).toBeEnabled();
  });

  it("keeps reset disabled without ntp.admin", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        if (String(input).endsWith("/v1/session")) {
          return json(200, sessionView(["ntp.read", "ntp.write"]));
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
    renderApp(<ResetPage />, { route: "/reset" });
    const submit = await screen.findByRole("button", { name: /Reset LabNTP/i });
    expect(submit).toBeDisabled();
  });
});
