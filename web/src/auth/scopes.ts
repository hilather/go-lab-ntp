export const SCOPE_READ = "ntp.read";
export const SCOPE_WRITE = "ntp.write";
export const SCOPE_ADMIN = "ntp.admin";
export const SCOPE_AUDIT = "ntp.audit.read";

export function hasScope(scopes: readonly string[], need: string): boolean {
  return scopes.includes(SCOPE_ADMIN) || scopes.includes(need);
}
