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

// 当前 pptx-preview 的 _parseBorder 实现会先读主题里的 a:lnRef（accent1 默认为橙色），
// 然后再读 p:spPr/a:ln，因此当形状显式声明 <a:ln><a:noFill/></a:ln> 时，
// 主题色已经先被合并进 border，后续 Object.assign(空) 无法清掉，导致 PPT 里"无线条"的
// 元素在预览里被画成橙色边线。这里在函数入口前置判断，遇到 a:noFill 就直接清空 border
// 并返回，绕开 a:lnRef 的默认色。
const target =
    'r.prototype._parseBorder=function(){var e=c(this.source,["p:style","a:lnRef"]);';
const replacement =
    'r.prototype._parseBorder=function(){var lnEl=c(this.source,["p:spPr","a:ln"]);if(c(lnEl,"a:noFill")){this.border={};return}var e=c(this.source,["p:style","a:lnRef"]);';

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
