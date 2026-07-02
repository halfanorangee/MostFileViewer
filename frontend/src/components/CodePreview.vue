<template>
    <div class="code-preview-root">
        <div
            ref="previewBody"
            class="code-preview-body"
            :class="{ 'code-preview-body--resizing': webPreviewResizing }"
        >
            <div ref="host" class="code-preview-wrapper"></div>
            <div
                v-if="webPreviewVisible && showPreviewIcon"
                class="code-preview-resizer"
                role="separator"
                :aria-label="isMarkdownPreviewFile ? '调整 Markdown 预览宽度' : '调整网页预览宽度'"
                aria-orientation="vertical"
                :aria-valuenow="Math.round(webPreviewWidthPercent)"
                aria-valuemin="20"
                aria-valuemax="80"
                @pointerdown="handleWebPreviewResizeStart"
            ></div>
            <section
                v-if="webPreviewVisible && showPreviewIcon"
                class="code-preview-web-panel"
                :style="webPreviewPanelStyle"
                :aria-label="isMarkdownPreviewFile ? 'Markdown 预览' : '网页预览'"
            >
                <iframe
                    v-if="isStandaloneWebPreviewFile"
                    class="code-preview-web-panel__frame"
                    :srcdoc="webPreviewContent"
                    :title="name ? `${name} 预览` : '网页预览'"
                    sandbox="allow-scripts"
                    referrerpolicy="no-referrer"
                ></iframe>
                <div
                    v-else-if="isMarkdownPreviewFile"
                    ref="markdownPreviewBody"
                    class="code-preview-md-body markdown-body"
                    v-html="markdownPreviewHtml"
                ></div>
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
                :disabled="!showPreviewIcon || syncingDocument"
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
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { keymap } from "@codemirror/view";
import { basicSetup, EditorView } from "codemirror";
import { EditorState, Compartment } from "@codemirror/state";
import { syntaxHighlighting } from "@codemirror/language";
import {
    search,
    searchKeymap,
    highlightSelectionMatches,
} from "@codemirror/search";
import { renderMarkdown } from "../composables/useMarkdown.js";
import { renderMermaidDiagrams } from "../composables/useMermaid.js";
import { createScrollSync } from "../composables/useScrollSync.js";
import { useTheme } from "../composables/useTheme.js";
import {
    syntaxOptions,
    detectSyntaxKey,
    resolveLanguageBySyntaxKey,
    isStandaloneWebFile,
    isMarkdownFile,
} from "../composables/useSyntaxLanguage.js";
import {
    codeHighlightStyle,
    chineseSearchPhrases,
    editorTheme,
} from "../composables/useEditorTheme.js";
import { createWebPreviewResizer } from "../composables/useWebPreviewResizer.js";
import "./markdown-preview.css";

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
const isStandaloneWebPreviewFile = computed(() =>
    isStandaloneWebFile(props.extension),
);
const isMarkdownPreviewFile = computed(() => isMarkdownFile(props.extension));
const showPreviewIcon = computed(
    () => isStandaloneWebPreviewFile.value || isMarkdownPreviewFile.value,
);
const webPreviewVisible = ref(false);
const webPreviewContent = ref("");
const markdownPreviewHtml = ref("");
const previewActionTitle = computed(() => {
    if (isMarkdownPreviewFile.value) {
        return webPreviewVisible.value ? "关闭预览" : "预览";
    }
    if (!isStandaloneWebPreviewFile.value) {
        return "当前文件暂不支持网页预览";
    }
    return webPreviewVisible.value ? "关闭网页预览" : "预览网页样式";
});

const host = ref(null);
const previewBody = ref(null);
const markdownPreviewBody = ref(null);
const language = new Compartment();
const editable = new Compartment();
let editor = null;
let syncingFromProps = false;
let documentSyncToken = 0;
// 编辑时实时刷新 Markdown 预览的防抖定时器，避免逐字符渲染带来的性能开销。
let markdownLivePreviewTimer = null;
const MARKDOWN_LIVE_PREVIEW_DELAY = 120;
let lastContentVersion = null;
let lastExtension = "";
let lastName = "";

// 预览面板拖拽调宽控制器；拖拽结束后重建 Markdown 滚动同步锚点。
const {
    widthPercent: webPreviewWidthPercent,
    resizing: webPreviewResizing,
    panelStyle: webPreviewPanelStyle,
    handleResizeStart: handleWebPreviewResizeStart,
    stop: stopWebPreviewResize,
} = createWebPreviewResizer(() => previewBody.value, {
    onResizeEnd: refreshMarkdownScrollSync,
});

// Markdown 预览与源文的双向同步滚动控制器。
const scrollSync = createScrollSync(
    () => editor,
    () => markdownPreviewBody.value,
);

const { currentTheme } = useTheme();

// 渲染预览区内的 mermaid 图表；需等 v-html 完成 DOM 更新后执行。
// 图表渲染会改变元素高度，完成后重建滚动同步锚点。
function renderMarkdownMermaid(force = false) {
    if (!isMarkdownPreviewFile.value || !webPreviewVisible.value) {
        return;
    }
    nextTick(() => {
        renderMermaidDiagrams(markdownPreviewBody.value, force).then(() => {
            refreshMarkdownScrollSync();
        });
    });
}

// 明暗主题切换时重渲已完成的 mermaid 图表，使配色跟随主题。
watch(currentTheme, () => {
    renderMarkdownMermaid(true);
});

// 重建同步锚点：需等预览 DOM 渲染并完成布局后再采集。
function refreshMarkdownScrollSync() {
    if (!isMarkdownPreviewFile.value || !webPreviewVisible.value) {
        return;
    }
    nextTick(() => {
        requestAnimationFrame(() => scrollSync.rebuild());
    });
}

// 编辑 Markdown 时防抖刷新右侧预览，实现「编辑即预览」。
function scheduleMarkdownLivePreview() {
    if (!isMarkdownPreviewFile.value || !webPreviewVisible.value) {
        return;
    }
    if (markdownLivePreviewTimer !== null) {
        clearTimeout(markdownLivePreviewTimer);
    }
    markdownLivePreviewTimer = setTimeout(() => {
        markdownLivePreviewTimer = null;
        markdownPreviewHtml.value = renderMarkdown(getContent());
        renderMarkdownMermaid();
        refreshMarkdownScrollSync();
    }, MARKDOWN_LIVE_PREVIEW_DELAY);
}

const LARGE_CONTENT_CHARS = 512 * 1024;
const CONTENT_CHUNK_CHARS = 256 * 1024;

watch(
    () => props.encoding,
    (encoding) => {
        selectedEncoding.value = normalizeEncoding(encoding);
    },
    { immediate: true },
);

watch([() => props.extension, () => props.name], ([extension, name]) => {
    selectedSyntax.value = detectSyntaxKey(extension, name);
    if (markdownLivePreviewTimer !== null) {
        clearTimeout(markdownLivePreviewTimer);
        markdownLivePreviewTimer = null;
    }
    webPreviewVisible.value = false;
    webPreviewContent.value = "";
    markdownPreviewHtml.value = "";
});

watch(
    () => props.contentVersion,
    () => {
        if (!webPreviewVisible.value) {
            return;
        }
        if (isMarkdownPreviewFile.value) {
            markdownPreviewHtml.value = renderMarkdown(getContent());
            renderMarkdownMermaid();
            refreshMarkdownScrollSync();
        } else {
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
                            scheduleMarkdownLivePreview();
                        }),
                        editorTheme,
                        language.of(resolveSelectedLanguage()),
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
                effects: language.reconfigure(resolveSelectedLanguage()),
            });
        }
    },
    { immediate: true, flush: "post" },
);

onBeforeUnmount(() => {
    documentSyncToken += 1;
    if (markdownLivePreviewTimer !== null) {
        clearTimeout(markdownLivePreviewTimer);
        markdownLivePreviewTimer = null;
    }
    stopWebPreviewResize();
    scrollSync.dispose();
    editor?.destroy();
    editor = null;
});

async function replaceEditorContent(content) {
    const token = ++documentSyncToken;
    const languageEffect = language.reconfigure(resolveSelectedLanguage());

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
        effects: language.reconfigure(resolveSelectedLanguage()),
    });
}

function handlePreviewClick() {
    if (!showPreviewIcon.value || syncingDocument.value) {
        return;
    }

    if (isMarkdownPreviewFile.value) {
        handleMarkdownPreviewClick();
        return;
    }

    if (webPreviewVisible.value) {
        webPreviewVisible.value = false;
        return;
    }

    webPreviewContent.value = getContent();
    webPreviewVisible.value = true;
}

function handleMarkdownPreviewClick() {
    if (webPreviewVisible.value) {
        webPreviewVisible.value = false;
        return;
    }

    markdownPreviewHtml.value = renderMarkdown(getContent());
    webPreviewVisible.value = true;
    renderMarkdownMermaid();
    refreshMarkdownScrollSync();
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

.code-preview-web-panel__frame {
    flex: 1;
    width: 100%;
    min-height: 0;
    border: 0;
    background: #ffffff;
}

.code-preview-md-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 24px 28px;
    background: var(--bg-surface);
    color: var(--text-primary);
    font-size: 14px;
    line-height: 1.7;
    word-wrap: break-word;
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
