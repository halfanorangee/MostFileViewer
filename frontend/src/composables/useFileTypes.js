// 文件类型相关的纯工具函数：语法选项、扩展名→语法键推断、文件类型判断。
// 不依赖任何 CodeMirror 包，可被任意组件安全导入而不会引入语言包体积。

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

// Central media definition used by preview routing and MIME capability checks.
export const MEDIA_TYPES = Object.freeze({
    audio: Object.freeze({
        extensions: Object.freeze([
            ".mp3", ".wav", ".ogg", ".oga", ".flac", ".m4a", ".aac", ".wma",
        ]),
        mime: Object.freeze({
            ".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg",
            ".oga": "audio/ogg", ".flac": "audio/flac", ".m4a": "audio/mp4",
            ".aac": "audio/aac", ".wma": "audio/x-ms-wma",
        }),
    }),
    video: Object.freeze({
        extensions: Object.freeze([
            ".mp4", ".webm", ".mov", ".avi", ".mkv", ".flv", ".wmv",
        ]),
        mime: Object.freeze({
            ".mp4": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
            ".avi": "video/x-msvideo", ".mkv": "video/x-matroska",
            ".flv": "video/x-flv", ".wmv": "video/x-ms-wmv",
        }),
    }),
});

export const MEDIA_MIME_BY_EXTENSION = Object.freeze({
    ...MEDIA_TYPES.audio.mime,
    ...MEDIA_TYPES.video.mime,
});

export function getMediaType(extension) {
    const normalized = (extension || "").toLowerCase();
    if (MEDIA_TYPES.audio.extensions.includes(normalized)) return "audio";
    if (MEDIA_TYPES.video.extensions.includes(normalized)) return "video";
    return "";
}

export function isAudioExtension(extension) {
    return getMediaType(extension) === "audio";
}

export function isVideoExtension(extension) {
    return getMediaType(extension) === "video";
}

// 电子书扩展名集合，EBookPreview 负责渲染。
// .epub / .fb2 / .cbz / .fbz 由 foliate-js 按扩展名或魔数识别，
// .mobi / .prc / .azw / .azw3 走魔数（BOOKMOBI）识别。
export const EBOOK_EXTENSIONS = Object.freeze([
    ".epub",
    ".mobi",
    ".prc",
    ".azw",
    ".azw3",
    ".fb2",
    ".fbz",
    ".cbz",
]);

export function isEbookExtension(extension) {
    return EBOOK_EXTENSIONS.includes((extension || "").toLowerCase());
}

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

// 由 syntax 键反推默认文件扩展名（虚拟 tab 落地时若 syntax 已切，按语法给后缀）
export function inferExtensionFromSyntax(syntaxKey) {
    const map = {
        javascript: ".js",
        typescript: ".ts",
        vue: ".vue",
        html: ".html",
        css: ".css",
        json: ".json",
        markdown: ".md",
        xml: ".xml",
        yaml: ".yaml",
        toml: ".toml",
        python: ".py",
        shell: ".sh",
        go: ".go",
        java: ".java",
        sql: ".sql",
        cpp: ".cpp",
        csharp: ".cs",
        rust: ".rs",
        ruby: ".rb",
        php: ".php",
        swift: ".swift",
        kotlin: ".kt",
        scala: ".scala",
        dart: ".dart",
        lua: ".lua",
        perl: ".pl",
        r: ".r",
        clojure: ".clj",
        powershell: ".ps1",
        protobuf: ".proto",
        nginx: ".conf",
        cmake: ".cmake",
        diff: ".patch",
        properties: ".ini",
    };
    return map[syntaxKey] || ".txt";
}
