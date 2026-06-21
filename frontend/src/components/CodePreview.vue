<template>
    <div>
        <div ref="host" class="code-preview-wrapper"></div>
        <div class="code-preview-status">
            <label class="code-preview-encoding">
                <span>编码：</span>
                <select
                    :value="selectedEncoding"
                    class="code-preview-encoding-select"
                    @change="handleEncodingChange"
                >
                    <option
                        v-for="encoding in encodingOptions"
                        :key="encoding.value"
                        :value="encoding.value"
                    >
                        {{ encoding.label }}
                    </option>
                </select>
            </label>
        </div>
    </div>
</template>

<script setup>
import { onBeforeUnmount, ref, watch } from "vue";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { EditorState, Compartment } from "@codemirror/state";
import { StreamLanguage } from "@codemirror/language";
import {
    search,
    searchKeymap,
    highlightSelectionMatches,
} from "@codemirror/search";
import { EditorState as CMState } from "@codemirror/state";
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

const props = defineProps({
    content: {
        type: String,
        default: "",
    },
    extension: {
        type: String,
        default: "",
    },
    name: {
        type: String,
        default: "",
    },
    encoding: {
        type: String,
        default: "utf-8",
    },
});

const emit = defineEmits(["dirty", "save", "encoding-change"]);

const encodingOptions = [
    { label: "UTF-8", value: "utf-8" },
    { label: "GBK", value: "gbk" },
    { label: "GB2312", value: "gb2312" },
    { label: "Big5", value: "big5" },
    { label: "UTF-16 LE", value: "utf-16le" },
    { label: "UTF-16 BE", value: "utf-16be" },
    { label: "ISO-8859-1", value: "iso-8859-1" },
];
const selectedEncoding = ref(normalizeEncoding(props.encoding));

const host = ref(null);
const language = new Compartment();
const chineseSearchPhrases = CMState.phrases.of({
    "Control character": "控制字符",
    Search: "搜索",
    Replace: "替换",
    next: "下一个",
    previous: "上一个",
    all: "全部",
    "match case": "区分大小写",
    regexp: "正则表达式",
    "by word": "全词匹配",
    replace: "替换",
    "replace all": "全部替换",
    close: "关闭",
});
let editor = null;
let syncingFromProps = false;

watch(
    () => props.encoding,
    (encoding) => {
        selectedEncoding.value = normalizeEncoding(encoding);
    },
    { immediate: true },
);

watch(
    [() => props.content, () => props.extension, () => props.name, host],
    ([content, extension, name, container]) => {
        if (!container) {
            return;
        }

        if (!editor) {
            editor = new EditorView({
                state: EditorState.create({
                    doc: content,
                    extensions: [
                        basicSetup,
                        chineseSearchPhrases,
                        search({ top: false }),
                        highlightSelectionMatches(),
                        keymap.of([
                            ...searchKeymap,
                            {
                                key: "Mod-s",
                                preventDefault: true,
                                run: () => {
                                    emit("save");
                                    return true;
                                },
                            },
                        ]),
                        EditorView.lineWrapping,
                        EditorView.updateListener.of((update) => {
                            if (!update.docChanged || syncingFromProps) {
                                return;
                            }
                            emit("dirty");
                        }),
                        EditorView.theme({
                            "&": {
                                height: "100%",
                                fontSize: "13px",
                                backgroundColor: "#ffffff",
                                color: "#1f2937",
                            },
                            ".cm-scroller": {
                                fontFamily:
                                    '"JetBrains Mono", "Fira Code", Consolas, monospace',
                            },
                            ".cm-content": {
                                padding: "16px 0",
                            },
                            ".cm-gutters": {
                                backgroundColor: "#f8fafc",
                                color: "#94a3b8",
                                borderRight: "1px solid #e2e8f0",
                            },
                            ".cm-activeLine": {
                                backgroundColor: "#f8fbff",
                            },
                            ".cm-activeLineGutter": {
                                backgroundColor: "#f1f5f9",
                            },
                            ".cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection":
                                {
                                    backgroundColor: "rgba(37, 99, 235, 0.18)",
                                },
                            ".cm-cursor, .cm-dropCursor": {
                                borderLeftColor: "#2563eb",
                            },
                            ".cm-searchMatch": {
                                backgroundColor: "#fef3c7",
                                border: "1px solid #f59e0b",
                                borderRadius: "4px",
                            },
                            ".cm-searchMatch.cm-searchMatch-selected": {
                                backgroundColor: "#dbeafe",
                                borderColor: "#2563eb",
                            },
                            ".cm-selectionMatch": {
                                backgroundColor: "rgba(59, 130, 246, 0.12)",
                            },
                            ".cm-panels": {
                                backgroundColor: "#f9fbff",
                                color: "#334155",
                                borderBottom: "1px solid #e6edf7",
                            },
                            ".cm-panels-top": {
                                borderTopLeftRadius: "14px",
                                borderTopRightRadius: "14px",
                            },
                            ".cm-search": {
                                padding: "10px 12px",
                                gap: "8px",
                                alignItems: "center",
                                flexWrap: "wrap",
                                fontSize: "13px",
                            },
                            ".cm-search > *": {
                                display: "inline-flex",
                                alignItems: "center",
                                verticalAlign: "middle",
                            },
                            ".cm-search label": {
                                display: "inline-flex",
                                alignItems: "center",
                                gap: "6px",
                                color: "#64748b",
                                fontSize: "13px",
                                verticalAlign: "middle",
                            },
                            ".cm-search input": {
                                fontSize: "13px",
                                height: "27px",
                                padding: "0 8px",
                                borderRadius: "4px",
                                border: "1px solid #d7deea",
                                backgroundColor: "#ffffff",
                                color: "#1f2937",
                                outline: "none",
                            },
                            ".cm-search input[type='checkbox']": {
                                appearance: "none",
                                WebkitAppearance: "none",
                                width: "14px",
                                height: "14px",
                                margin: 0,
                                padding: 0,
                                borderRadius: "4px",
                                border: "1px solid #c5d2e6",
                                backgroundColor: "#ffffff",
                                position: "relative",
                                cursor: "pointer",
                                flexShrink: 0,
                            },
                            ".cm-search input[type='checkbox']:hover": {
                                borderColor: "#94a3b8",
                                backgroundColor: "#f8fbff",
                            },
                            ".cm-search input[type='checkbox']:focus": {
                                borderColor: "#2563eb",
                                boxShadow: "0 0 0 2px rgba(37, 99, 235, 0.12)",
                            },
                            ".cm-search input[type='checkbox']:checked": {
                                backgroundColor: "#2563eb",
                                borderColor: "#2563eb",
                            },
                            ".cm-search input[type='checkbox']:checked::after":
                                {
                                    content: '""',
                                    position: "absolute",
                                    left: "4px",
                                    top: "1px",
                                    width: "4px",
                                    height: "8px",
                                    border: "solid #ffffff",
                                    borderWidth: "0 2px 2px 0",
                                    transform: "rotate(45deg)",
                                },
                            ".cm-search input:focus": {
                                borderColor: "#2563eb",
                            },
                            ".cm-search button": {
                                fontSize: "13px",
                                height: "27px",
                                padding: "0 8px",
                                border: "1px solid #d7deea",
                                borderRadius: "4px",
                                background: "#ffffff",
                                color: "#334155",
                                cursor: "pointer",
                            },
                            ".cm-search button:hover": {
                                backgroundColor: "#f3f7ff",
                                borderColor: "#c5d2e6",
                            },
                            ".cm-search button:focus": {
                                outline: "none",
                                borderColor: "#2563eb",
                            },
                            ".cm-search button[name='close']": {
                                width: "20px",
                                borderRadius: "4px",
                            },
                            ".cm-search label:has(input[type='checkbox'])": {
                                display: "inline-flex",
                                alignItems: "center",
                                justifyContent: "center",
                                height: "27px",
                                padding: "0 8px",
                                color: "#334155",
                                cursor: "pointer",
                                lineHeight: 1,
                            },
                            ".cm-button": {
                                font: "inherit",
                            },
                        }),
                        language.of(resolveLanguage(extension, name)),
                    ],
                }),
                parent: container,
            });
            return;
        }

        if (editor.state.doc.toString() === content) {
            editor.dispatch({
                effects: language.reconfigure(resolveLanguage(extension, name)),
            });
            return;
        }

        syncingFromProps = true;
        editor.dispatch({
            changes: {
                from: 0,
                to: editor.state.doc.length,
                insert: content,
            },
            effects: language.reconfigure(resolveLanguage(extension, name)),
        });
        syncingFromProps = false;
    },
    { immediate: true, flush: "post" },
);

onBeforeUnmount(() => {
    editor?.destroy();
    editor = null;
});

function getContent() {
    return editor?.state.doc.toString() ?? props.content;
}

function handleEncodingChange(event) {
    const previousEncoding = selectedEncoding.value;
    const encoding = normalizeEncoding(event.target.value);
    emit("encoding-change", encoding);
    selectedEncoding.value = previousEncoding;
}

defineExpose({
    getContent,
});

function normalizeEncoding(encoding) {
    const normalized = (encoding || "utf-8").toLowerCase();
    return encodingOptions.some((option) => option.value === normalized)
        ? normalized
        : "utf-8";
}

function resolveLanguage(extension, name) {
    const normalizedName = (name || "").toLowerCase();

    if (normalizedName === "dockerfile") {
        return streamLanguage(dockerFile);
    }
    if (normalizedName === "cmakelists.txt") {
        return streamLanguage(cmake);
    }

    switch ((extension || "").toLowerCase()) {
        case ".js":
        case ".jsx":
        case ".mjs":
        case ".cjs":
            return javascript({ jsx: true });
        case ".ts":
        case ".tsx":
            return javascript({ jsx: true, typescript: true });
        case ".vue":
            return vue({ base: html() });
        case ".html":
        case ".htm":
            return html();
        case ".css":
        case ".scss":
        case ".sass":
        case ".less":
            return css();
        case ".json":
        case ".jsonc":
        case ".map":
            return json();
        case ".md":
        case ".markdown":
        case ".mdx":
            return markdown();
        case ".py":
        case ".pyw":
            return python();
        case ".java":
            return java();
        case ".c":
        case ".h":
        case ".cc":
        case ".cpp":
        case ".cxx":
        case ".hh":
        case ".hpp":
        case ".hxx":
            return cpp();
        case ".cs":
            return streamLanguage(csharp);
        case ".kt":
        case ".kts":
            return streamLanguage(kotlin);
        case ".scala":
        case ".sc":
            return streamLanguage(scala);
        case ".dart":
            return streamLanguage(dart);
        case ".go":
            return go();
        case ".rs":
            return rust();
        case ".php":
        case ".phtml":
            return php();
        case ".rb":
        case ".rake":
        case ".gemspec":
            return streamLanguage(ruby);
        case ".swift":
            return streamLanguage(swift);
        case ".lua":
            return streamLanguage(lua);
        case ".pl":
        case ".pm":
            return streamLanguage(perl);
        case ".r":
        case ".rmd":
            return streamLanguage(r);
        case ".clj":
        case ".cljs":
        case ".cljc":
        case ".edn":
            return streamLanguage(clojure);
        case ".sh":
        case ".bash":
        case ".zsh":
        case ".fish":
            return streamLanguage(shell);
        case ".ps1":
        case ".psm1":
        case ".psd1":
            return streamLanguage(powerShell);
        case ".sql":
            return sql();
        case ".xml":
        case ".svg":
        case ".xhtml":
            return xml();
        case ".yaml":
        case ".yml":
            return yaml();
        case ".toml":
            return streamLanguage(toml);
        case ".ini":
        case ".env":
        case ".properties":
        case ".conf":
            return streamLanguage(properties);
        case ".dockerfile":
            return streamLanguage(dockerFile);
        case ".cmake":
            return streamLanguage(cmake);
        case ".proto":
            return streamLanguage(protobuf);
        case ".diff":
        case ".patch":
            return streamLanguage(diff);
        case ".nginx":
            return streamLanguage(nginx);
        default:
            return [];
    }
}

function streamLanguage(mode) {
    return StreamLanguage.define(mode);
}
</script>

<style scoped>
.code-preview-wrapper {
    flex: 1;
    min-width: 0;
    min-height: 0;
    height: calc(100% - 28px);
    overflow: hidden;
    background: #fff;
}
.code-preview-status {
    background: #f1f5f9;
    height: 28px;
    line-height: 28px;
    padding: 0 12px;
    font-size: 13px;
    color: #64748b;
    display: flex;
    align-items: center;
    column-gap: 12px;
}

.code-preview-encoding {
    display: inline-flex;
    align-items: center;
    gap: 4px;
}

.code-preview-encoding-select {
    height: 20px;
    padding: 0 20px 0 6px;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    background: #ffffff;
    color: #334155;
    font-size: 12px;
    outline: none;
}

.code-preview-encoding-select:focus {
    border-color: #2563eb;
}
</style>
