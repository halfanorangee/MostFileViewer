<template>
    <div>
        <div ref="host" class="code-preview-wrapper"></div>
        <div class="code-preview-status"></div>
    </div>
</template>

<script setup>
import { onBeforeUnmount, ref, watch } from "vue";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { EditorState, Compartment } from "@codemirror/state";
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
import { sql } from "@codemirror/lang-sql";
import { xml } from "@codemirror/lang-xml";

const props = defineProps({
    content: {
        type: String,
        default: "",
    },
    extension: {
        type: String,
        default: "",
    },
});

const emit = defineEmits(["dirty", "save"]);

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
    [() => props.content, () => props.extension, host],
    ([content, extension, container]) => {
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
                        language.of(resolveLanguage(extension)),
                    ],
                }),
                parent: container,
            });
            return;
        }

        if (editor.state.doc.toString() === content) {
            editor.dispatch({
                effects: language.reconfigure(resolveLanguage(extension)),
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
            effects: language.reconfigure(resolveLanguage(extension)),
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

defineExpose({
    getContent,
});

function resolveLanguage(extension) {
    switch ((extension || "").toLowerCase()) {
        case ".js":
        case ".jsx":
        case ".mjs":
        case ".cjs":
        case ".ts":
        case ".tsx":
        case ".vue":
            return javascript({ jsx: true, typescript: true });
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
            return json();
        case ".md":
        case ".markdown":
            return markdown();
        case ".py":
            return python();
        case ".java":
            return java();
        case ".c":
        case ".h":
        case ".cc":
        case ".cpp":
        case ".cxx":
        case ".hpp":
            return cpp();
        case ".sql":
            return sql();
        case ".xml":
        case ".svg":
        case ".xhtml":
            return xml();
        default:
            return [];
    }
}
</script>

<style scoped>
.code-preview-wrapper {
    flex: 1;
    min-width: 0;
    min-height: 0;
    height: calc(100% - 24px);
    overflow: hidden;
    background: #fff;
}
.code-preview-status {
    background: #f1f5f9;
    height: 24px;
    line-height: 24px;
    padding: 0 12px;
    font-size: 13px;
    color: #64748b;
}
</style>
