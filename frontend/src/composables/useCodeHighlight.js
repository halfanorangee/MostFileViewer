import { highlightCode, tagHighlighter } from "@lezer/highlight";
import { tags as t } from "@lezer/highlight";
import { StreamLanguage } from "@codemirror/language";
import { javascript } from "@codemirror/lang-javascript";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { json } from "@codemirror/lang-json";
import { python } from "@codemirror/lang-python";
import { java } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { go } from "@codemirror/lang-go";
import { rust } from "@codemirror/lang-rust";
import { php } from "@codemirror/lang-php";
import { sql } from "@codemirror/lang-sql";
import { xml } from "@codemirror/lang-xml";
import { yaml } from "@codemirror/lang-yaml";
import { clojure } from "@codemirror/legacy-modes/mode/clojure";
import { cmake } from "@codemirror/legacy-modes/mode/cmake";
import { csharp, dart, kotlin, scala } from "@codemirror/legacy-modes/mode/clike";
import { diff } from "@codemirror/legacy-modes/mode/diff";
import { dockerFile } from "@codemirror/legacy-modes/mode/dockerfile";
import { lua } from "@codemirror/legacy-modes/mode/lua";
import { nginx } from "@codemirror/legacy-modes/mode/nginx";
import { perl } from "@codemirror/legacy-modes/mode/perl";
import { powerShell } from "@codemirror/legacy-modes/mode/powershell";
import { properties } from "@codemirror/legacy-modes/mode/properties";
import { protobuf } from "@codemirror/legacy-modes/mode/protobuf";
import { r } from "@codemirror/legacy-modes/mode/r";
import { ruby } from "@codemirror/legacy-modes/mode/ruby";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { swift } from "@codemirror/legacy-modes/mode/swift";
import { toml } from "@codemirror/legacy-modes/mode/toml";

// 将高亮标签映射为 class 名，class 的具体配色在 theme.css 中通过
// --cm-syntax-* 变量定义，从而与 CodeMirror 编辑器保持一致并自动联动明暗主题。
const highlighter = tagHighlighter([
    { tag: t.comment, class: "tok-comment" },
    { tag: [t.keyword, t.operatorKeyword, t.modifier], class: "tok-keyword" },
    { tag: [t.string, t.character, t.attributeValue], class: "tok-string" },
    { tag: [t.number, t.bool, t.null, t.atom], class: "tok-constant" },
    {
        tag: [t.variableName, t.propertyName, t.attributeName],
        class: "tok-variable",
    },
    {
        tag: [
            t.function(t.variableName),
            t.function(t.propertyName),
            t.labelName,
        ],
        class: "tok-function",
    },
    {
        tag: [t.typeName, t.className, t.namespace, t.tagName],
        class: "tok-type",
    },
    { tag: [t.operator, t.punctuation, t.bracket], class: "tok-operator" },
    { tag: [t.regexp, t.escape, t.special(t.string)], class: "tok-special" },
    {
        tag: [t.meta, t.annotation, t.processingInstruction],
        class: "tok-meta",
    },
    { tag: [t.heading, t.strong], class: "tok-heading" },
    { tag: t.link, class: "tok-link" },
    { tag: t.invalid, class: "tok-invalid" },
]);

function streamLanguage(mode) {
    return StreamLanguage.define(mode);
}

// 代码块围栏语言名（含常见别名）→ CodeMirror 语言支持。
// 返回值可能是 LanguageSupport 或 Language（StreamLanguage），统一在
// getParser 中取 .parser。
function resolveLanguage(lang) {
    switch (lang) {
        case "javascript":
        case "js":
        case "jsx":
        case "mjs":
        case "cjs":
            return javascript({ jsx: true });
        case "typescript":
        case "ts":
        case "tsx":
            return javascript({ jsx: true, typescript: true });
        case "html":
        case "htm":
            return html();
        case "css":
        case "scss":
        case "less":
            return css();
        case "json":
        case "jsonc":
            return json();
        case "python":
        case "py":
            return python();
        case "java":
            return java();
        case "c":
        case "cpp":
        case "c++":
        case "cc":
        case "h":
        case "hpp":
            return cpp();
        case "csharp":
        case "cs":
        case "c#":
            return streamLanguage(csharp);
        case "kotlin":
        case "kt":
            return streamLanguage(kotlin);
        case "scala":
            return streamLanguage(scala);
        case "dart":
            return streamLanguage(dart);
        case "go":
        case "golang":
            return go();
        case "rust":
        case "rs":
            return rust();
        case "php":
            return php();
        case "ruby":
        case "rb":
            return streamLanguage(ruby);
        case "swift":
            return streamLanguage(swift);
        case "lua":
            return streamLanguage(lua);
        case "perl":
        case "pl":
            return streamLanguage(perl);
        case "r":
            return streamLanguage(r);
        case "clojure":
        case "clj":
            return streamLanguage(clojure);
        case "shell":
        case "sh":
        case "bash":
        case "zsh":
            return streamLanguage(shell);
        case "powershell":
        case "ps1":
            return streamLanguage(powerShell);
        case "sql":
            return sql();
        case "xml":
        case "svg":
            return xml();
        case "yaml":
        case "yml":
            return yaml();
        case "toml":
            return streamLanguage(toml);
        case "properties":
        case "ini":
        case "env":
            return streamLanguage(properties);
        case "dockerfile":
        case "docker":
            return streamLanguage(dockerFile);
        case "cmake":
            return streamLanguage(cmake);
        case "protobuf":
        case "proto":
            return streamLanguage(protobuf);
        case "diff":
        case "patch":
            return streamLanguage(diff);
        case "nginx":
            return streamLanguage(nginx);
        default:
            return null;
    }
}

function getParser(lang) {
    const support = resolveLanguage((lang || "").toLowerCase());
    if (!support) {
        return null;
    }
    // LanguageSupport 有 .language.parser；StreamLanguage（Language）直接有 .parser。
    if (support.language && support.language.parser) {
        return support.language.parser;
    }
    if (support.parser) {
        return support.parser;
    }
    return null;
}

function escapeHtml(text) {
    return text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;");
}

/**
 * 将代码字符串按指定语言生成带高亮 class 的 HTML。
 * 无法识别语言或解析失败时返回 null，交由调用方回退为纯文本。
 * @param {string} code 代码内容
 * @param {string} lang 围栏语言名
 * @returns {string|null} 带 <span class="tok-*"> 的 HTML，或 null
 */
export function highlightCodeToHtml(code, lang) {
    const parser = getParser(lang);
    if (!parser) {
        return null;
    }
    try {
        const tree = parser.parse(code);
        let result = "";
        highlightCode(
            code,
            tree,
            highlighter,
            (text, classes) => {
                result += classes
                    ? `<span class="${classes}">${escapeHtml(text)}</span>`
                    : escapeHtml(text);
            },
            () => {
                result += "\n";
            },
        );
        return result;
    } catch {
        return null;
    }
}
