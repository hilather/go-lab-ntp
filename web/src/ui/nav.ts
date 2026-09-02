export type NavGroup = "CLOCKS" | "LAB";

export type NavItem = { to: string; label: string; group: NavGroup };

export function navItems(canReset: boolean): NavItem[] {
  const items: NavItem[] = [
    { to: "/", label: "Filters", group: "CLOCKS" },
    { to: "/preview", label: "Preview", group: "CLOCKS" },
    { to: "/queries", label: "Queries", group: "CLOCKS" },
    { to: "/features", label: "Features", group: "LAB" },
    { to: "/status", label: "Status", group: "LAB" },
  ];
  if (canReset) {
    items.push({ to: "/reset", label: "Reset", group: "LAB" });
  }
  return items;
}

export function navGroups(canReset: boolean): { heading: NavGroup; items: NavItem[] }[] {
  const all = navItems(canReset);
  return [
    { heading: "CLOCKS", items: all.filter((i) => i.group === "CLOCKS") },
    { heading: "LAB", items: all.filter((i) => i.group === "LAB") },
  ];
}
