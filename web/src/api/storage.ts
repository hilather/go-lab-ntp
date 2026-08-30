const TOKEN_KEY = /^(labntp[_-]?)?(token|bearer|authorization|api[_-]?key|csrf)$/i;
const BEARER_VALUE = /bearer\s+\S+/i;

function scan(store: Storage, label: string): void {
  for (let i = 0; i < store.length; i += 1) {
    const key = store.key(i);
    if (key === null) {
      continue;
    }
    if (TOKEN_KEY.test(key)) {
      throw new Error(`${label} must not hold a token key (${key})`);
    }
    const value = store.getItem(key);
    if (value !== null && BEARER_VALUE.test(value)) {
      throw new Error(`${label} must not hold a bearer token`);
    }
  }
}

/** Fail closed if a bearer leaked into web storage. Session uses HttpOnly cookies. */
export function assertNoTokenStorage(): void {
  if (typeof window === "undefined") {
    return;
  }
  scan(window.localStorage, "localStorage");
  scan(window.sessionStorage, "sessionStorage");
}
