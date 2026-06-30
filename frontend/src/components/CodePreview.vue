<template>
    <div class="code-preview-root">
        <div
            ref="previewBody"
            class="code-preview-body"
            :class="{ 'code-preview-body--resizing': webPreviewResizing }"
        >
            <div ref="host" class="code-preview-wrapper"></div>
            <div
                v-if="webPreviewVisible && isStandaloneWebPreviewFile"
                class="code-preview-resizer"
                role="separator"
                aria-label="调整网页预览宽度"
                aria-orientation="vertical"
                :aria-valuenow="Math.round(webPreviewWidthPercent)"
                aria-valuemin="20"
                aria-valuemax="80"
                @pointerdown="handleWebPreviewResizeStart"
            ></div>
            <section
                v-if="webPreviewVisible && isStandaloneWebPreviewFile"
                class="code-preview-web-panel"
                :style="webPreviewPanelStyle"
                aria-label="网页预览"
            >
                <div class="code-preview-web-panel__header">
                    <span>网页预览</span>
                    <span class="code-preview-web-panel__name">{{ name }}</span>
                </div>
                <iframe
                    class="code-preview-web-panel__frame"
                    :srcdoc="webPreviewContent"
                    :title="name ? `${name} 预览` : '网页预览'"
                    sandbox="allow-scripts"
                    referrerpolicy="no-referrer"
                ></iframe>
            </section>
        </div>
        <div class="code-preview-status">
            <label class="code-preview-encoding">
                <select
                    :value="selectedSyntax"
                    class="code-preview-encoding-select"
                    :disabled="encodingLoading || syncingDocument"
                    @change="handleSyntaxChange"
                >
                    <option
                        v-for="syntax in syntaxOptions"
                        :key="syntax.value"
                        :value="syntax.value"
                    >
                        {{ syntax.label }}
                    </option>
                </select>
            </label>
            <label class="code-preview-encoding">
                <select
                    :value="selectedEncoding"
                    class="code-preview-encoding-select"
                    :disabled="encodingLoading || syncingDocument"
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
            <button
                v-if="showPreviewIcon"
                class="code-preview-action"
                type="button"
                :class="{ 'code-preview-action--active': webPreviewVisible }"
                :title="previewActionTitle"
                :aria-label="previewActionTitle"
                :disabled="!isStandaloneWebPreviewFile || syncingDocument"
                @click="handlePreviewClick"
            >
                <svg
                    class="code-preview-action__icon"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    focusable="false"
                >
                    <path
                        d="M12 5.5c4.36 0 7.63 3.1 9.17 5.46.41.63.41 1.45 0 2.08C19.63 15.4 16.36 18.5 12 18.5s-7.63-3.1-9.17-5.46a1.9 1.9 0 0 1 0-2.08C4.37 8.6 7.64 5.5 12 5.5Zm0 1.8c-3.45 0-6.2 2.45-7.67 4.45a.42.42 0 0 0 0 .5c1.47 2 4.22 4.45 7.67 4.45s6.2-2.45 7.67-4.45a.42.42 0 0 0 0-.5C18.2 9.75 15.45 7.3 12 7.3Zm0 1.7a3 3 0 1 1 0 6 3 3 0 0 1 0-6Zm0 1.75a1.25 1.25 0 1 0 0 2.5 1.25 1.25 0 0 0 0-2.5Z"
                        fill="currentColor"
                    />
                </svg>
            </button>
        </div>
    </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { EditorState, Compartment } from "@codemirror/state";
import {
    HighlightStyle,
    StreamLanguage,
    syntaxHighlighting,
} from "@codemirror/language";
import { tags } from "@lezer/highlight";
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
    contentVersion: {
        type: Number,
        default: 0,
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
    encodingLoading: {
        type: Boolean,
        default: false,
    },
});

const emit = defineEmits(["dirty", "save", "encoding-change"]);

const syntaxOptions = [
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
const selectedSyntax = ref(detectSyntaxKey(props.extension, props.name));

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
const syncingDocument = ref(false);
const showPreviewIcon = computed(() => isStandaloneWebFile(props.extension));
const isStandaloneWebPreviewFile = showPreviewIcon;
const webPreviewVisible = ref(false);
const webPreviewContent = ref("");
const webPreviewWidthPercent = ref(45);
const webPreviewResizing = ref(false);
const webPreviewPanelStyle = computed(() => ({
    flexBasis: `${webPreviewWidthPercent.value}%`,
}));
const previewActionTitle = computed(() => {
    if (!isStandaloneWebPreviewFile.value) {
        return "当前文件暂不支持网页预览";
    }
    return webPreviewVisible.value ? "关闭网页预览" : "预览网页样式";
});

const host = ref(null);
const previewBody = ref(null);
const language = new Compartment();
const editable = new Compartment();
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
let documentSyncToken = 0;
let lastContentVersion = null;
let lastExtension = "";
let lastName = "";

const codeHighlightStyle = HighlightStyle.define([
    {
        tag: tags.comment,
        color: "var(--cm-syntax-comment)",
        fontStyle: "italic",
    },
    {
        tag: [tags.keyword, tags.operatorKeyword, tags.modifier],
        color: "var(--cm-syntax-keyword)",
    },
    {
        tag: [tags.string, tags.character, tags.attributeValue],
        color: "var(--cm-syntax-string)",
    },
    {
        tag: [tags.number, tags.bool, tags.null, tags.atom],
        color: "var(--cm-syntax-constant)",
    },
    {
        tag: [tags.variableName, tags.propertyName, tags.attributeName],
        color: "var(--cm-syntax-variable)",
    },
    {
        tag: [
            tags.function(tags.variableName),
            tags.function(tags.propertyName),
            tags.labelName,
        ],
        color: "var(--cm-syntax-function)",
    },
    {
        tag: [tags.typeName, tags.className, tags.namespace, tags.tagName],
        color: "var(--cm-syntax-type)",
    },
    {
        tag: [tags.operator, tags.punctuation, tags.bracket],
        color: "var(--cm-syntax-operator)",
    },
    {
        tag: [tags.regexp, tags.escape, tags.special(tags.string)],
        color: "var(--cm-syntax-special)",
    },
    {
        tag: [tags.meta, tags.annotation, tags.processingInstruction],
        color: "var(--cm-syntax-meta)",
    },
    {
        tag: [tags.heading, tags.strong],
        color: "var(--cm-syntax-heading)",
        fontWeight: "600",
    },
    {
        tag: tags.link,
        color: "var(--cm-syntax-link)",
        textDecoration: "underline",
    },
    {
        tag: tags.invalid,
        color: "var(--cm-syntax-invalid)",
        textDecoration: "underline wavy var(--cm-syntax-invalid)",
    },
]);

const LARGE_CONTENT_CHARS = 512 * 1024;
const CONTENT_CHUNK_CHARS = 256 * 1024;
const MIN_WEB_PREVIEW_WIDTH_PERCENT = 20;
const MAX_WEB_PREVIEW_WIDTH_PERCENT = 80;

watch(
    () => props.encoding,
    (encoding) => {
        selectedEncoding.value = normalizeEncoding(encoding);
    },
    { immediate: true },
);

watch([() => props.extension, () => props.name], ([extension, name]) => {
    selectedSyntax.value = detectSyntaxKey(extension, name);
    webPreviewVisible.value = false;
    webPreviewContent.value = "";
});

watch(
    () => props.contentVersion,
    () => {
        if (webPreviewVisible.value) {
            webPreviewContent.value = getContent();
        }
    },
);

watch(
    () => props.encodingLoading,
    (loading) => {
        editor?.dispatch({
            effects: editable.reconfigure(EditorView.editable.of(!loading)),
        });
    },
);

watch(
    [() => props.contentVersion, () => props.extension, () => props.name, host],
    ([contentVersion, extension, name, container]) => {
        if (!container) {
            return;
        }

        if (!editor) {
            editor = new EditorView({
                state: EditorState.create({
                    doc: props.content,
                    extensions: [
                        basicSetup,
                        chineseSearchPhrases,
                        search({ top: false }),
                        highlightSelectionMatches(),
                        syntaxHighlighting(codeHighlightStyle),
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
                        editable.of(EditorView.editable.of(!props.encodingLoading)),
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
                                backgroundColor: "var(--cm-bg)",
                                color: "var(--cm-text)",
                            },
                            ".cm-scroller": {
                                fontFamily:
                                    '"JetBrains Mono", "Fira Code", Consolas, monospace',
                            },
                            ".cm-content": {
                                padding: "16px 0",
                                position: "relative",
                                zIndex: 2,
                                backgroundColor: "transparent",
                            },
                            ".cm-gutters": {
                                backgroundColor: "var(--cm-gutter-bg)",
                                color: "var(--cm-gutter-text)",
                                borderRight: "1px solid var(--cm-gutter-border)",
                            },
                            ".cm-activeLine": {
                                backgroundColor: "var(--cm-active-line-bg)",
                            },
                            ".cm-activeLineGutter": {
                                backgroundColor: "var(--cm-active-line-gutter-bg)",
                            },
                            ".cm-selectionLayer": {
                                zIndex: "3 !important",
                                pointerEvents: "none",
                            },
                            ".cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground":
                                {
                                    background: "var(--cm-selection-bg) !important",
                                },
                            ".cm-line::selection, .cm-line ::selection": {
                                backgroundColor: "var(--cm-selection-line-bg)",
                            },
                            ".cm-cursor, .cm-dropCursor": {
                                borderLeftColor: "var(--cm-cursor-color)",
                            },
                            ".cm-searchMatch": {
                                backgroundColor: "var(--cm-search-match-bg)",
                                border: "1px solid var(--cm-search-match-border)",
                                borderRadius: "4px",
                            },
                            ".cm-searchMatch.cm-searchMatch-selected": {
                                backgroundColor: "var(--cm-search-match-selected-bg)",
                                borderColor: "var(--cm-search-match-selected-border)",
                            },
                            ".cm-selectionMatch": {
                                backgroundColor: "var(--cm-selection-match-bg)",
                            },
                            ".cm-panels": {
                                backgroundColor: "var(--cm-panels-bg)",
                                color: "var(--cm-panels-text)",
                                borderBottom: "1px solid var(--cm-panels-border)",
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
                                color: "var(--cm-search-label-text)",
                                fontSize: "13px",
                                verticalAlign: "middle",
                            },
                            ".cm-search input": {
                                fontSize: "13px",
                                height: "27px",
                                padding: "0 8px",
                                borderRadius: "4px",
                                border: "1px solid var(--cm-search-input-border)",
                                backgroundColor: "var(--cm-search-input-bg)",
                                color: "var(--cm-search-input-text)",
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
                                border: "1px solid var(--cm-search-checkbox-border)",
                                backgroundColor: "var(--cm-search-checkbox-bg)",
                                position: "relative",
                                cursor: "pointer",
                                flexShrink: 0,
                            },
                            ".cm-search input[type='checkbox']:hover": {
                                borderColor: "var(--cm-search-checkbox-hover-border)",
                                backgroundColor: "var(--cm-search-checkbox-hover-bg)",
                            },
                            ".cm-search input[type='checkbox']:focus": {
                                borderColor: "var(--accent-primary)",
                                boxShadow: "0 0 0 2px var(--cm-search-checkbox-focus-ring)",
                            },
                            ".cm-search input[type='checkbox']:checked": {
                                backgroundColor: "var(--cm-search-checkbox-checked-bg)",
                                borderColor: "var(--cm-search-checkbox-checked-border)",
                            },
                            ".cm-search input[type='checkbox']:checked::after":
                                {
                                    content: '""',
                                    position: "absolute",
                                    left: "4px",
                                    top: "1px",
                                    width: "4px",
                                    height: "8px",
                                    border: "solid var(--cm-search-checkbox-check-color)",
                                    borderWidth: "0 2px 2px 0",
                                    transform: "rotate(45deg)",
                                },
                            ".cm-search input:focus": {
                                borderColor: "var(--accent-primary)",
                            },
                            ".cm-search button": {
                                fontSize: "13px",
                                height: "27px",
                                padding: "0 8px",
                                border: "1px solid var(--cm-search-button-border)",
                                borderRadius: "4px",
                                background: "var(--cm-search-button-bg)",
                                color: "var(--cm-search-button-text)",
                                cursor: "pointer",
                            },
                            ".cm-search button:hover": {
                                backgroundColor: "var(--cm-search-button-hover-bg)",
                                borderColor: "var(--cm-search-button-hover-border)",
                            },
                            ".cm-search button:focus": {
                                outline: "none",
                                borderColor: "var(--accent-primary)",
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
                                color: "var(--cm-search-label-checkbox-text)",
                                cursor: "pointer",
                                lineHeight: 1,
                            },
                            ".cm-button": {
                                font: "inherit",
                            },
                        }),
                        language.of(resolveSelectedLanguage(extension, name)),
                    ],
                }),
                parent: container,
            });
            lastContentVersion = contentVersion;
            lastExtension = extension;
            lastName = name;
            return;
        }

        const contentChanged = contentVersion !== lastContentVersion;
        const languageChanged = extension !== lastExtension || name !== lastName;
        lastContentVersion = contentVersion;
        lastExtension = extension;
        lastName = name;

        if (contentChanged) {
            void replaceEditorContent(props.content, extension, name);
            return;
        }

        if (languageChanged) {
            documentSyncToken += 1;
            editor.dispatch({
                effects: language.reconfigure(resolveSelectedLanguage(extension, name)),
            });
        }
    },
    { immediate: true, flush: "post" },
);

onBeforeUnmount(() => {
    documentSyncToken += 1;
    stopWebPreviewResize();
    editor?.destroy();
    editor = null;
});

async function replaceEditorContent(content, extension, name) {
    const token = ++documentSyncToken;
    const languageEffect = language.reconfigure(
        resolveSelectedLanguage(extension, name),
    );

    syncingFromProps = true;
    syncingDocument.value = content.length > LARGE_CONTENT_CHARS;
    try {
        if (!editor) {
            return;
        }

        editor.dispatch({
            effects: editable.reconfigure(EditorView.editable.of(false)),
        });

        if (content.length <= LARGE_CONTENT_CHARS) {
            editor.dispatch({
                changes: {
                    from: 0,
                    to: editor.state.doc.length,
                    insert: content,
                },
                effects: languageEffect,
            });
            return;
        }

        editor.dispatch({
            changes: {
                from: 0,
                to: editor.state.doc.length,
                insert: "",
            },
            effects: languageEffect,
        });

        for (let offset = 0; offset < content.length; ) {
            if (token !== documentSyncToken || !editor) {
                return;
            }

            await nextFrame();
            let nextOffset = getSafeChunkEnd(
                content,
                offset + CONTENT_CHUNK_CHARS,
            );
            if (nextOffset <= offset) {
                nextOffset = Math.min(content.length, offset + CONTENT_CHUNK_CHARS);
            }
            editor.dispatch({
                changes: {
                    from: editor.state.doc.length,
                    insert: content.slice(offset, nextOffset),
                },
            });
            offset = nextOffset;
        }
    } finally {
        if (token === documentSyncToken) {
            syncingFromProps = false;
            syncingDocument.value = false;
            editor?.dispatch({
                effects: editable.reconfigure(
                    EditorView.editable.of(!props.encodingLoading),
                ),
            });
        }
    }
}

function handleWebPreviewResizeStart(event) {
    if (!previewBody.value) {
        return;
    }

    event.preventDefault();
    webPreviewResizing.value = true;
    window.addEventListener("pointermove", handleWebPreviewResizeMove);
    window.addEventListener("pointerup", stopWebPreviewResize, { once: true });
    window.addEventListener("pointercancel", stopWebPreviewResize, { once: true });
    handleWebPreviewResizeMove(event);
}

function handleWebPreviewResizeMove(event) {
    if (!previewBody.value) {
        return;
    }

    const rect = previewBody.value.getBoundingClientRect();
    if (rect.width <= 0) {
        return;
    }

    const previewWidth = rect.right - event.clientX;
    const nextPercent = (previewWidth / rect.width) * 100;
    webPreviewWidthPercent.value = clamp(
        nextPercent,
        MIN_WEB_PREVIEW_WIDTH_PERCENT,
        MAX_WEB_PREVIEW_WIDTH_PERCENT,
    );
}

function stopWebPreviewResize() {
    if (!webPreviewResizing.value) {
        return;
    }

    webPreviewResizing.value = false;
    window.removeEventListener("pointermove", handleWebPreviewResizeMove);
    window.removeEventListener("pointerup", stopWebPreviewResize);
    window.removeEventListener("pointercancel", stopWebPreviewResize);
}

function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
}

function getSafeChunkEnd(content, end) {
    if (end >= content.length) {
        return content.length;
    }

    const previous = content.charCodeAt(end - 1);
    if (previous >= 0xd800 && previous <= 0xdbff) {
        return Math.max(end - 1, 0);
    }
    return end;
}

function nextFrame() {
    return new Promise((resolve) => requestAnimationFrame(resolve));
}

function getContent() {
    return editor?.state.doc.toString() ?? props.content;
}

function handleEncodingChange(event) {
    if (props.encodingLoading || syncingDocument.value) {
        return;
    }

    const previousEncoding = selectedEncoding.value;
    const encoding = normalizeEncoding(event.target.value);
    emit("encoding-change", encoding);
    selectedEncoding.value = previousEncoding;
}

function handleSyntaxChange(event) {
    if (props.encodingLoading || syncingDocument.value) {
        return;
    }

    selectedSyntax.value = event.target.value;
    documentSyncToken += 1;
    editor?.dispatch({
        effects: language.reconfigure(
            resolveSelectedLanguage(props.extension, props.name),
        ),
    });
}

function handlePreviewClick() {
    if (!isStandaloneWebPreviewFile.value || syncingDocument.value) {
        return;
    }

    if (webPreviewVisible.value) {
        webPreviewVisible.value = false;
        return;
    }

    webPreviewContent.value = getContent();
    webPreviewVisible.value = true;
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

function resolveSelectedLanguage() {
    return resolveLanguageBySyntaxKey(selectedSyntax.value);
}

function detectSyntaxKey(extension, name) {
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

function isStandaloneWebFile(extension) {
    return [".html", ".htm"].includes((extension || "").toLowerCase());
}

function resolveLanguageBySyntaxKey(syntax) {
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

function streamLanguage(mode) {
    return StreamLanguage.define(mode);
}
</script>

<style scoped>
.code-preview-root {
    position: relative;
    height: 100%;
    min-width: 0;
    min-height: 0;
}

.code-preview-body {
    display: flex;
    height: calc(100% - 30px);
    min-width: 0;
    min-height: 0;
}

.code-preview-body--resizing,
.code-preview-body--resizing * {
    cursor: col-resize !important;
    user-select: none;
}

.code-preview-body--resizing .code-preview-web-panel__frame {
    pointer-events: none;
}

.code-preview-wrapper {
    flex: 1 1 0;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
    background: var(--bg-surface);
}

.code-preview-resizer {
    flex: 0 0 8px;
    min-height: 0;
    cursor: col-resize;
    background: linear-gradient(
        to right,
        transparent 0,
        var(--border-cell) 50%,
        transparent 100%
    );
}

.code-preview-web-panel {
    flex: 0 0 45%;
    min-width: 240px;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg-surface);
}

.code-preview-web-panel__header {
    flex: 0 0 auto;
    height: 34px;
    padding: 0 12px;
    display: flex;
    align-items: center;
    gap: 8px;
    border-bottom: 1px solid var(--border-primary);
    color: var(--text-secondary);
    font-size: 12px;
    font-weight: 600;
}

.code-preview-web-panel__name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
    font-weight: 400;
}

.code-preview-web-panel__frame {
    flex: 1;
    width: 100%;
    min-height: 0;
    border: 0;
    background: #ffffff;
}
.code-preview-status {
    background: var(--bg-statusbar);
    height: 30px;
    line-height: 30px;
    padding: 0 12px;
    font-size: 13px;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    column-gap: 20px;
}

.code-preview-encoding {
    display: inline-flex;
    align-items: center;
    gap: 4px;
}

.code-preview-encoding-select {
    height: 20px;
    padding: 0 2px;
    border: 1px solid transparent;
    border-radius: 2px;
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    outline: none;
    appearance: none;
    cursor: pointer;
}

.code-preview-encoding-select:focus {
    padding: 0 4px;
    border-color: var(--accent-primary);
    background: var(--bg-surface);
    appearance: auto;
}

.code-preview-encoding-select:disabled {
    cursor: not-allowed;
    opacity: 0.6;
}

.code-preview-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    margin-left: auto;
    padding: 0;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
}

.code-preview-action:hover:not(:disabled),
.code-preview-action--active {
    background: var(--bg-surface);
    color: var(--accent-primary);
}

.code-preview-action:focus-visible {
    outline: 1px solid var(--accent-primary);
    outline-offset: 2px;
}

.code-preview-action:disabled {
    cursor: not-allowed;
    opacity: 0.5;
}

.code-preview-action__icon {
    width: 18px;
    height: 18px;
}
</style>
