import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { APIError, clearMemoryCSRF, createSession, deleteSession, getSession } from "../api/client";
import type { SessionView } from "../api/types";
import { hasScope } from "./scopes";

type AuthState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "signed_in"; session: SessionView };

type AuthContextValue = {
  state: AuthState;
  hasScope: (scope: string) => boolean;
  login: (authorization: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: "loading" });
  const gen = useRef(0);

  useEffect(() => {
    const mine = gen.current;
    let cancelled = false;
    void (async () => {
      try {
        const session = await getSession();
        if (cancelled || gen.current !== mine) {
          return;
        }
        setState({ status: "signed_in", session });
      } catch {
        if (cancelled || gen.current !== mine) {
          return;
        }
        setState({ status: "anonymous" });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (authorization: string) => {
    gen.current += 1;
    await createSession(authorization);
    const session = await getSession();
    setState({ status: "signed_in", session });
  }, []);

  const logout = useCallback(async () => {
    gen.current += 1;
    try {
      await deleteSession();
    } catch (err) {
      if (err instanceof APIError && err.problem.status === 403) {
        return;
      }
    }
    clearMemoryCSRF();
    setState({ status: "anonymous" });
  }, []);

  const checkScope = useCallback(
    (scope: string) => {
      if (state.status !== "signed_in") {
        return false;
      }
      return hasScope(state.session.scopes ?? [], scope);
    },
    [state],
  );

  const value = useMemo(
    () => ({ state, hasScope: checkScope, login, logout }),
    [state, checkScope, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth requires AuthProvider");
  }
  return ctx;
}
