#!/usr/bin/env node
// Fail if the production JS entry exceeds the operator-SPA budget.
import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const budgetBytes = 450 * 1024;
const assets = join("dist", "assets");
const files = readdirSync(assets).filter((f) => f.endsWith(".js"));
if (files.length === 0) {
  console.error("bundle budget: no JS assets in web/dist/assets");
  process.exit(1);
}
let worst = { name: "", size: 0 };
for (const name of files) {
  const size = statSync(join(assets, name)).size;
  console.log(`bundle ${name} ${size} bytes`);
  if (size > worst.size) {
    worst = { name, size };
  }
}
if (worst.size > budgetBytes) {
  console.error(`bundle budget: ${worst.name} is ${worst.size} bytes (limit ${budgetBytes})`);
  process.exit(1);
}
console.log(`bundle budget: ok (largest ${worst.name} ${worst.size} <= ${budgetBytes})`);
