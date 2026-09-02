import { type ReactNode, useEffect, useState } from "react";
import { BrowserRouter, NavLink, Navigate, Outlet, Route, Routes } from "react-router-dom";
import { getStatus } from "./api/client";
import { AuthProvider, useAuth } from "./auth/AuthProvider";
import { SCOPE_ADMIN } from "./auth/scopes";
import { FeaturesPage } from "./pages/FeaturesPage";
import { FiltersPage } from "./pages/FiltersPage";
import { LoginPage } from "./pages/LoginPage";
import { PreviewPage } from "./pages/PreviewPage";
import { QueriesPage } from "./pages/QueriesPage";
import { ResetPage } from "./pages/ResetPage";
import { StatusPage } from "./pages/StatusPage";
import { navGroups } from "./ui/nav";

function SkipLink() {
  return (
    <a className="skip-link" href="#app-main">
      Skip to main content
    </a>
  );
}

function NavItem({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink to={to} className={({ isActive }) => (isActive ? "nav-active" : undefined)} end={to === "/"}>
      {children}
    </NavLink>
  );
}

function Shell() {
  const { state, hasScope, logout } = useAuth();
  const signedIn = state.status === "signed_in";
  const scopes = signedIn ? (state.session.scopes ?? []) : [];
  const groups = signedIn ? navGroups(hasScope(SCOPE_ADMIN)) : [];
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!signedIn) {
      setReady(false);
      return;
    }
    let cancelled = false;
    void getStatus()
      .then((st) => {
        if (!cancelled) {
          setReady(Boolean(st.ready));
        }
      })
      .catch(() => {
        if (!cancelled) {
          setReady(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [signedIn]);

  return (
    <div className="app">
      <SkipLink />
      <header className="masthead">
        <NavLink className="brand" to="/">
          LabNTP
        </NavLink>
        <div className="masthead-end">
          {signedIn ? (
            <>
              <span className="chip chip--ok">live</span>
              <span className={ready ? "chip chip--ok" : "chip"}>ready</span>
              {scopes.map((scope) => (
                <span key={scope} className="chip chip--scope">
                  {scope}
                </span>
              ))}
              <button type="button" className="sign-out" onClick={() => void logout()}>
                Sign out
              </button>
            </>
          ) : null}
        </div>
      </header>
      <div className={signedIn ? "app-body" : "app-body app-body--anon"}>
        {signedIn ? (
          <nav className="rail" aria-label="Primary">
            {groups.map((group) => (
              <div key={group.heading}>
                <p className="rail-group">{group.heading}</p>
                {group.items.map((item) => (
                  <NavItem key={item.to} to={item.to}>
                    {item.label}
                  </NavItem>
                ))}
              </div>
            ))}
          </nav>
        ) : null}
        <div id="app-main">
          <Outlet />
        </div>
      </div>
    </div>
  );
}

function RequireSession() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status !== "signed_in") {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}

function RedirectIfSignedIn() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status === "signed_in") {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route element={<Shell />}>
            <Route element={<RedirectIfSignedIn />}>
              <Route path="/login" element={<LoginPage />} />
            </Route>
            <Route element={<RequireSession />}>
              <Route path="/" element={<FiltersPage />} />
              <Route path="/preview" element={<PreviewPage />} />
              <Route path="/queries" element={<QueriesPage />} />
              <Route path="/features" element={<FeaturesPage />} />
              <Route path="/status" element={<StatusPage />} />
              <Route path="/reset" element={<ResetPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
