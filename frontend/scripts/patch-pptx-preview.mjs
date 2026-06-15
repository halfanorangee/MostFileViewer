import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const packageRoot = path.resolve(__dirname, "..");
const bundlePath = path.join(
    packageRoot,
    "node_modules",
    "pptx-preview",
    "dist",
    "pptx-preview.es.js",
);

const target =
    'r.prototype._parseBorder=function(){var o=c(this.source,["p:spPr","a:ln"]);var e=c(this.source,["p:style","a:lnRef"]);';
const replacement =
    'r.prototype._parseBorder=function(){var o=c(this.source,["p:spPr","a:ln"]);if(c(o,"a:noFill")){this.border={};return}var e=c(this.source,["p:style","a:lnRef"]);';

if (!fs.existsSync(bundlePath)) {
    console.warn("[patch-pptx-preview] pptx-preview bundle not found, skipping");
    process.exit(0);
}

const bundle = fs.readFileSync(bundlePath, "utf8");

if (bundle.includes(replacement)) {
    console.log("[patch-pptx-preview] already patched");
    process.exit(0);
}

if (!bundle.includes(target)) {
    console.warn("[patch-pptx-preview] expected code not found, skipping");
    process.exit(0);
}

fs.writeFileSync(bundlePath, bundle.replace(target, replacement));
console.log("[patch-pptx-preview] patched no-fill line handling");
