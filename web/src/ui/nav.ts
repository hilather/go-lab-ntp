export type NavItem = { to: string; label: string };

export function navItems(canReset: boolean): NavItem[] {
  const items: NavItem[] = [
    { to: "/", label: "Filters" },
    { to: "/preview", label: "Preview" },
    { to: "/queries", label: "Queries" },
    { to: "/features", label: "Features" },
    { to: "/status", label: "Status" },
  ];
  if (canReset) {
    items.push({ to: "/reset", label: "Reset" });
  }
  return items;
}
