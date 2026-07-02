import { marked } from "marked";
import DOMPurify from "dompurify";
import { highlightCodeToHtml } from "./useCodeHighlight.js";

marked.setOptions({
    gfm: true, // 表格、删除线、任务列表
    breaks: false, // 遵循 CommonMark 换行规则
});

function escapeHtml(text) {
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;");
}

// 将含 Unicode 的字符串编码为 base64，供 data 属性安全承载 mermaid 源码，
// 避免 --> 等语法字符被 DOMPurify 转义或语法高亮破坏。
function encodeMermaidSource(text) {
    return btoa(unescape(encodeURIComponent(text)));
}

// 自定义代码块渲染：用 CodeMirror 的 Lezer 解析器生成带 tok-* class 的高亮 HTML，
// class 配色在 theme.css 中映射到 --cm-syntax-* 变量，与编辑器一致并联动明暗主题。
// 无法识别语言或解析失败时回退为转义后的纯文本。
marked.use({
    renderer: {
        code({ text, lang }) {
            const language = (lang || "").trim().split(/\s+/)[0];
            // mermaid 代码块输出占位容器，源码经 base64 存入 data 属性，
            // 由 useMermaid 在 DOM 挂载后异步渲染为 SVG。
            if (language === "mermaid") {
                return `<div class="mermaid-diagram" data-mermaid-source="${encodeMermaidSource(text)}"></div>\n`;
            }
            const highlighted = highlightCodeToHtml(text, language);
            const body = highlighted ?? escapeHtml(text);
            const langClass = language ? ` class="language-${language}"` : "";
            return `<pre><code${langClass}>${body}</code></pre>\n`;
        },
    },
});

// 净化后为所有链接补全在新窗口打开的属性，避免在应用内跳转导致预览上下文丢失。
DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node.tagName === "A" && node.hasAttribute("href")) {
        node.setAttribute("target", "_blank");
        node.setAttribute("rel", "noopener noreferrer");
    }
});

/**
 * 将 Markdown 源码解析为经过净化的安全 HTML 字符串。
 * @param {string} source Markdown 源文本
 * @returns {string} 可直接用于 v-html 的净化后 HTML
 */
export function renderMarkdown(source) {
    const rawHtml = marked.parse(source ?? "");
    return DOMPurify.sanitize(rawHtml, {
        ADD_ATTR: ["target", "rel", "class", "data-mermaid-source"],
    });
}
