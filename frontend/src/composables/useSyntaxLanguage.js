// 文件扩展名 / 文件名 → 语法键 → CodeMirror 语言支持的映射集中在此。
// 将 CodePreview 组件中大量的 @codemirror/lang-* 导入与分支逻辑抽离出来，
// 组件只需消费 syntaxOptions、detectSyntaxKey、resolveLanguageBySyntaxKey 等接口。
import { StreamLanguage } from "@codemirror/language";
import { javascript } from "@codemirror/lang-javascript";
import { html } from "@codemirror/lang-html";
import { css } from "@codemirror/lang-css";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { python } from "@codemirror/lang-python";
import { java } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { go } from "@codemirror/lang-go";
import { rust } from "@codemirror/lang-rust";
import { php } from "@codemirror/lang-php";
import { sql } from "@codemirror/lang-sql";
import { vue } from "@codemirror/lang-vue";
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

// 状态栏语法下拉的可选项，value 即语法键。
export const syntaxOptions = [
    { label: "C / C++", value: "cpp" },
    { label: "C#", value: "csharp" },
    { label: "Clojure", value: "clojure" },
    { label: "CMake", value: "cmake" },
    { label: "CSS / SCSS / Less", value: "css" },
    { label: "Dart", value: "dart" },
    { label: "Diff / Patch", value: "diff" },
    { label: "Dockerfile", value: "dockerfile" },
    { label: "Go", value: "go" },
    { label: "HTML", value: "html" },
    { label: "Java", value: "java" },
    { label: "JavaScript / JSX", value: "javascript" },
    { label: "JSON", value: "json" },
    { label: "Kotlin", value: "kotlin" },
    { label: "Lua", value: "lua" },
    { label: "Markdown", value: "markdown" },
    { label: "Nginx", value: "nginx" },
    { label: "Perl", value: "perl" },
    { label: "PHP", value: "php" },
    { label: "PowerShell", value: "powershell" },
    { label: "Properties / INI / ENV", value: "properties" },
    { label: "Protocol Buffers", value: "protobuf" },
    { label: "Python", value: "python" },
    { label: "R", value: "r" },
    { label: "Ruby", value: "ruby" },
    { label: "Rust", value: "rust" },
    { label: "Scala", value: "scala" },
    { label: "Shell / Bash", value: "shell" },
    { label: "SQL", value: "sql" },
    { label: "Swift", value: "swift" },
    { label: "TOML", value: "toml" },
    { label: "TXT", value: "text" },
    { label: "TypeScript / TSX", value: "typescript" },
    { label: "Vue", value: "vue" },
    { label: "XML / SVG", value: "xml" },
    { label: "YAML", value: "yaml" },
];

// 根据文件扩展名与文件名推断默认语法键。
export function detectSyntaxKey(extension, name) {
    const normalizedName = (name || "").toLowerCase();

    if (normalizedName === "dockerfile") {
        return "dockerfile";
    }
    if (normalizedName === "cmakelists.txt") {
        return "cmake";
    }

    switch ((extension || "").toLowerCase()) {
        case ".js":
        case ".jsx":
        case ".mjs":
        case ".cjs":
            return "javascript";
        case ".ts":
        case ".tsx":
            return "typescript";
        case ".vue":
            return "vue";
        case ".html":
        case ".htm":
            return "html";
        case ".css":
        case ".scss":
        case ".sass":
        case ".less":
            return "css";
        case ".json":
        case ".jsonc":
        case ".map":
            return "json";
        case ".md":
        case ".markdown":
        case ".mdx":
            return "markdown";
        case ".py":
        case ".pyw":
            return "python";
        case ".java":
            return "java";
        case ".c":
        case ".h":
        case ".cc":
        case ".cpp":
        case ".cxx":
        case ".hh":
        case ".hpp":
        case ".hxx":
            return "cpp";
        case ".cs":
            return "csharp";
        case ".kt":
        case ".kts":
            return "kotlin";
        case ".scala":
        case ".sc":
            return "scala";
        case ".dart":
            return "dart";
        case ".go":
            return "go";
        case ".rs":
            return "rust";
        case ".php":
        case ".phtml":
            return "php";
        case ".rb":
        case ".rake":
        case ".gemspec":
            return "ruby";
        case ".swift":
            return "swift";
        case ".lua":
            return "lua";
        case ".pl":
        case ".pm":
            return "perl";
        case ".r":
        case ".rmd":
            return "r";
        case ".clj":
        case ".cljs":
        case ".cljc":
        case ".edn":
            return "clojure";
        case ".sh":
        case ".bash":
        case ".zsh":
        case ".fish":
            return "shell";
        case ".ps1":
        case ".psm1":
        case ".psd1":
            return "powershell";
        case ".sql":
            return "sql";
        case ".xml":
        case ".svg":
        case ".xhtml":
            return "xml";
        case ".yaml":
        case ".yml":
            return "yaml";
        case ".toml":
            return "toml";
        case ".ini":
        case ".env":
        case ".properties":
        case ".conf":
            return "properties";
        case ".dockerfile":
            return "dockerfile";
        case ".cmake":
            return "cmake";
        case ".proto":
            return "protobuf";
        case ".diff":
        case ".patch":
            return "diff";
        case ".nginx":
            return "nginx";
        default:
            return "text";
    }
}

export function isStandaloneWebFile(extension) {
    return [".html", ".htm"].includes((extension || "").toLowerCase());
}

export function isMarkdownFile(extension) {
    return [".md", ".markdown", ".mdx"].includes(
        (extension || "").toLowerCase(),
    );
}

function streamLanguage(mode) {
    return StreamLanguage.define(mode);
}

// 语法键 → CodeMirror 语言支持。返回值可为 LanguageSupport、Language 或空数组。
export function resolveLanguageBySyntaxKey(syntax) {
    switch (syntax) {
        case "javascript":
            return javascript({ jsx: true });
        case "typescript":
            return javascript({ jsx: true, typescript: true });
        case "vue":
            return vue({ base: html() });
        case "html":
            return html();
        case "css":
            return css();
        case "json":
            return json();
        case "markdown":
            return markdown();
        case "python":
            return python();
        case "java":
            return java();
        case "cpp":
            return cpp();
        case "csharp":
            return streamLanguage(csharp);
        case "kotlin":
            return streamLanguage(kotlin);
        case "scala":
            return streamLanguage(scala);
        case "dart":
            return streamLanguage(dart);
        case "go":
            return go();
        case "rust":
            return rust();
        case "php":
            return php();
        case "ruby":
            return streamLanguage(ruby);
        case "swift":
            return streamLanguage(swift);
        case "lua":
            return streamLanguage(lua);
        case "perl":
            return streamLanguage(perl);
        case "r":
            return streamLanguage(r);
        case "clojure":
            return streamLanguage(clojure);
        case "shell":
            return streamLanguage(shell);
        case "powershell":
            return streamLanguage(powerShell);
        case "sql":
            return sql();
        case "xml":
            return xml();
        case "yaml":
            return yaml();
        case "toml":
            return streamLanguage(toml);
        case "properties":
            return streamLanguage(properties);
        case "dockerfile":
            return streamLanguage(dockerFile);
        case "cmake":
            return streamLanguage(cmake);
        case "protobuf":
            return streamLanguage(protobuf);
        case "diff":
            return streamLanguage(diff);
        case "nginx":
            return streamLanguage(nginx);
        default:
            return [];
    }
}
