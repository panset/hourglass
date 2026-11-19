// SPDX-License-Identifier: MIT
// Generates TypeScript types from the OpenAPI contract.
// Usage: node scripts/gen-api.mjs <openapi.yaml> <out.ts>
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import openapiTS, { astToString } from "openapi-typescript";

// Under Bazel (rules_js) the process starts in the runfiles tree; argument paths are execroot-relative.
const base = process.env.JS_BINARY__EXECROOT ?? process.cwd();
const [spec, out] = process.argv.slice(2).map((p) => (p ? resolve(base, p) : p));
if (!spec || !out) {
  console.error("usage: gen-api.mjs <openapi.yaml> <out.ts>");
  process.exit(2);
}
const ast = await openapiTS(readFileSync(spec, "utf8"));
writeFileSync(out, "// SPDX-License-Identifier: MIT\n" + astToString(ast));
