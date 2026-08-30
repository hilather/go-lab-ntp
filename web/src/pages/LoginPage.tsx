import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { APIError, bearerAuthorization } from "../api/client";
import { useAuth } from "../auth/AuthProvider";

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault();
    const fd = new FormData(ev.currentTarget);
    const token = String(fd.get("token") ?? "").trim();
    if (token === "") {
      setError("Enter an API bearer token.");
      return;
    }
    setError("");
    setBusy(true);
    const form = ev.currentTarget;
    try {
      await login(bearerAuthorization(token));
      form.reset();
      void navigate("/", { replace: true });
    } catch (err) {
      const detail =
        err instanceof APIError
          ? err.problem.detail || "Sign-in failed."
          : err instanceof Error
            ? err.message
            : "Sign-in failed.";
      setError(detail);
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page page--narrow">
      <h1>Sign in to LabNTP</h1>
      <p>
        Exchange a scoped API bearer token for an HttpOnly session cookie. Credentials are not written to web
        storage. LabNTP does not accept HTTP Basic.
      </p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <form className="stack" onSubmit={(e) => void onSubmit(e)} noValidate>
        <div className="field">
          <label htmlFor="login-token">API bearer token</label>
          <input
            id="login-token"
            name="token"
            type="password"
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
            required
          />
        </div>
        <button type="submit" disabled={busy}>
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </main>
  );
}
