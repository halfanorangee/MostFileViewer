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
            <div
                class="code-preview-dropdown"
                @mousedown.stop
            >
                <button
                    type="button"
                    class="code-preview-dropdown__btn"
                    :disabled="encodingLoading || syncingDocument"
                    :title="currentSyntaxLabel"
                    :aria-label="`文件格式：${currentSyntaxLabel}`"
                    @click="toggleSyntaxMenu"
                >
                    <span class="code-preview-dropdown__label"
                        >{{ currentSyntaxLabel }}</span
                    >
                </button>
                <div
                    v-if="syntaxMenuOpen"
                    class="menu-panel code-preview-dropdown__panel"
                    role="menu"
                >
                    <button
                        v-for="syntax in syntaxOptions"
                        :key="syntax.value"
                        type="button"
                        class="menu-item code-preview-dropdown__item"
                        :class="{
                            'code-preview-dropdown__item--active':
                                syntax.value === selectedSyntax,
                        }"
                        role="menuitemradio"
                        :aria-checked="syntax.value === selectedSyntax"
                        @click="handleSyntaxSelect(syntax.value)"
                    >
                        <span class="code-preview-dropdown__item-label">{{
                            syntax.label
                        }}</span>
                        <span
                            v-if="syntax.value === selectedSyntax"
                            class="code-preview-dropdown__item-check"
                            aria-hidden="true"
                            >✓</span
                        >
                    </button>
                </div>
            </div>
            <div
                class="code-preview-dropdown"
                @mousedown.stop
            >
                <button
                    type="button"
                    class="code-preview-dropdown__btn"
                    :disabled="encodingLoading || syncingDocument"
                    :title="currentEncodingLabel"
                    :aria-label="`编码：${currentEncodingLabel}`"
                    @click="toggleEncodingMenu"
                >
                    <span class="code-preview-dropdown__label"
                        >{{ currentEncodingLabel }}</span
                    >
                </button>
                <div
                    v-if="encodingMenuOpen"
                    class="menu-panel code-preview-dropdown__panel"
                    role="menu"
                >
                    <button
                        v-for="encoding in encodingOptions"
                        :key="encoding.value"
                        type="button"
                        class="menu-item code-preview-dropdown__item"
                        :class="{
                            'code-preview-dropdown__item--active':
                                encoding.value === selectedEncoding,
                        }"
                        role="menuitemradio"
                        :aria-checked="encoding.value === selectedEncoding"
                        @click="handleEncodingSelect(encoding.value)"
                    >
                        <span class="code-preview-dropdown__item-label">{{
                            encoding.label
                        }}</span>
                        <span
                            v-if="encoding.value === selectedEncoding"
                            class="code-preview-dropdown__item-check"
                            aria-hidden="true"
                            >✓</span
                        >
                    </button>
                </div>
            </div>
            <button
                type="button"
                class="code-preview-fontsize"
                :class="{
                    'code-preview-fontsize--default':
                        fontSize === defaultFontSize,
                }"
                :title="
                    fontSize === defaultFontSize
                        ? `当前字号 ${fontSize}px（默认；Ctrl+滚轮调节）`
                        : `当前 ${fontSize}px，点击恢复默认 ${defaultFontSize}px（Ctrl+滚轮调节）`
                "
                :aria-label="`字号 ${fontSize}px${
                    fontSize === defaultFontSize ? '' : '，点击重置'
                }`"
                @click="handleFontSizeResetClick"
            >
                {{ fontSize }}px
            </button>
            <label class="code-preview-autosave">
                <span>自动保存</span>
                <span
                    class="code-preview-switch"
                    :class="{ 'code-preview-switch--on': autoSaveEnabled }"
                >
                    <input
                        type="checkbox"
                        class="code-preview-switch__input"
                        role="switch"
                        :checked="autoSaveEnabled"
                        :disabled="encodingLoading || syncingDocument"
                        @change="handleAutoSaveChange"
                        @keydown.space.prevent
                    />
                    <span class="code-preview-switch__track">
                        <span class="code-preview-switch__thumb"></span>
                    </span>
                </span>
            </label>
            <span class="code-preview-spacer"></span>
            <template v-if="selectedSyntax === 'json'">
                <span
                    v-if="jsonError"
                    class="code-preview-json-error"
                    :title="jsonError"
                    >{{ jsonError }}</span
                >
                <button
                    class="code-preview-action code-preview-action--json"
                    type="button"
                    title="格式化 JSON"
                    :disabled="syncingDocument"
                    @click="handleFormatJson"
                >
                    <svg
                        class="code-preview-action__icon"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                        focusable="false"
                    >
                        <path
                            d="M9.4 16.6 4.8 12l4.6-4.6L8 6l-6 6 6 6 1.4-1.4Zm5.2 0 4.6-4.6-4.6-4.6L16 6l6 6-6 6-1.4-1.4Z"
                            fill="currentColor"
                        />
                    </svg>
                </button>
                <button
                    class="code-preview-action code-preview-action--json"
                    type="button"
                    title="压缩 JSON"
                    :disabled="syncingDocument"
                    @click="handleCompressJson"
                >
                    <svg
                        class="code-preview-action__icon"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                        focusable="false"
                    >
                        <path
                            d="M8 19h3v3h2v-3h3l-4-4-4 4Zm8-14h-3V2h-2v3H8l4 4 4-4ZM4 11v2h16v-2H4Z"
                            fill="currentColor"
                        />
                    </svg>
                </button>
            </template>
            <!-- 「另存为」按钮已移除，仅保留 Ctrl+Shift+S 快捷键入口 -->
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
            <button
                v-if="showPreviewIcon"
                class="code-preview-action code-preview-action--new-tab"
                type="button"
                title="在新标签页中预览"
                :aria-label="
                    '在新标签页中预览' +
                    (isMarkdownPreviewFile ? 'Markdown 预览' : '网页预览')
                "
                :disabled="!showPreviewIcon || syncingDocument"
                @click="handleOpenInNewTab"
            >
                <svg
                    class="code-preview-action__icon"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    focusable="false"
                >
                    <path
                        d="M19 19H5V5h7V3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7h-2v7ZM14 3v2h3.59l-9.83 9.83 1.41 1.41L19 6.41V10h2V3h-7Z"
                        fill="currentColor"
                    />
                </svg>
            </button>
        </div>
    </div>
</template>

<script setup>
import {
    computed,
    nextTick,
    onBeforeUnmount,
    onMounted,
    ref,
    watch,
} from "vue";
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
import { useFontSize } from "../composables/useFontSize.js";
import {
    syntaxOptions,
    detectSyntaxKey,
} from "../composables/useFileTypes.js";
import { resolveLanguageBySyntaxKey } from "../composables/useLanguageLoader.js";
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
    isVirtual: {
        type: Boolean,
        default: false,
    },
    isDirty: {
        type: Boolean,
        default: false,
    },
    isSaving: {
        type: Boolean,
        default: false,
    },
    autoSaveEnabled: {
        type: Boolean,
        default: true,
    },
});

const emit = defineEmits([
    "dirty",
    "save",
    "save-as",
    "encoding-change",
    "open-in-new-tab",
    "auto-save-toggle",
]);

const selectedSyntax = ref(detectSyntaxKey(props.extension, props.name));

// 文件格式 / 编码下拉菜单的展开状态。两者互斥，开启其中一个时关闭另一个。
const syntaxMenuOpen = ref(false);
const encodingMenuOpen = ref(false);

const currentSyntaxLabel = computed(() => {
    const match = syntaxOptions.find(
        (option) => option.value === selectedSyntax.value,
    );
    return match ? match.label : selectedSyntax.value;
});

const currentEncodingLabel = computed(() => {
    const match = encodingOptions.find(
        (option) => option.value === selectedEncoding.value,
    );
    return match ? match.label : selectedEncoding.value;
});

function toggleSyntaxMenu() {
    if (props.encodingLoading || syncingDocument.value) {
        return;
    }
    syntaxMenuOpen.value = !syntaxMenuOpen.value;
    if (syntaxMenuOpen.value) {
        encodingMenuOpen.value = false;
    }
}

function toggleEncodingMenu() {
    if (props.encodingLoading || syncingDocument.value) {
        return;
    }
    encodingMenuOpen.value = !encodingMenuOpen.value;
    if (encodingMenuOpen.value) {
        syntaxMenuOpen.value = false;
    }
}

function closeAllMenus() {
    syntaxMenuOpen.value = false;
    encodingMenuOpen.value = false;
}

function handleSyntaxSelect(value) {
    syntaxMenuOpen.value = false;
    if (value === selectedSyntax.value) {
        return;
    }
    handleSyntaxChange({ target: { value } });
}

function handleEncodingSelect(value) {
    encodingMenuOpen.value = false;
    if (value === selectedEncoding.value) {
        return;
    }
    handleEncodingChange({ target: { value } });
}

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
// JSON 格式化/压缩解析失败时的错误提示，仅在 JSON 语法下显示。
const jsonError = ref("");
const isStandaloneWebPreviewFile = computed(
    () => selectedSyntax.value === "html",
);
const isMarkdownPreviewFile = computed(
    () => selectedSyntax.value === "markdown",
);
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

// 状态栏另存为按钮的可见性：实文件 + virtual 都允许（用于另存为副本 / 首次落地）。
// 原地保存按钮已移除，保留 Ctrl+S 快捷键入口。
const canSaveAs = computed(() => !props.isSaving);
function handleSaveAsClick() {
    emit("save-as");
}

const host = ref(null);
const previewBody = ref(null);
const markdownPreviewBody = ref(null);
const language = new Compartment();
const editable = new Compartment();
let editor = null;
let syncingFromProps = false;
let documentSyncToken = 0;
// 语言加载令牌：每次发起异步语言加载时递增，加载完成后校验是否仍为最新，避免竞态覆盖。
let languageLoadToken = 0;
// 编辑时实时刷新 Markdown 预览的防抖定时器，避免逐字符渲染带来的性能开销。
let markdownLivePreviewTimer = null;
const MARKDOWN_LIVE_PREVIEW_DELAY = 120;
let markdownRenderToken = 0;
let lastContentVersion = null;
let lastExtension = "";
let lastName = "";
// 跟踪「当前 syntax 是否由用户在下拉里手动选择过」：true 时 watch 只清理预览状态，
// 不再用 props.extension 推断的 syntax 覆盖用户的选择。
// 主要场景：空白 tab 的 extension 固定为 .txt，但用户会切换到 markdown/json 等其他格式；
// 一旦开始输入，App.vue 会把 tab.name 改为输入内容的前 10 个字符，
// 此时不能再因为「name 变了」就把 syntax 重置回 text。
let userOverrodeSyntax = false;
// 独立跟踪最近一次进入 watch 的 extension，用于判断「extension 真的变了（新文件）」
// 与「name 变了（virtual tab 编辑）」两种情况。初始就同步为当前 extension，
// 避免首次 watch 触发时被误判为「extension 变化」并清掉用户刚选过的 syntax。
let lastSeenExtension = props.extension;
let releaseResizeScrollSync = null;

// 预览面板拖拽调宽控制器；拖拽结束后重建 Markdown 滚动同步锚点。
const {
    widthPercent: webPreviewWidthPercent,
    resizing: webPreviewResizing,
    panelStyle: webPreviewPanelStyle,
    handleResizeStart: handleWebPreviewResizeStart,
    stop: stopWebPreviewResize,
} = createWebPreviewResizer(() => previewBody.value, {
    onResizeStart: () => {
        releaseResizeScrollSync?.();
        releaseResizeScrollSync = scrollSync.suspend();
    },
    onResizeEnd: () => {
        releaseResizeScrollSync?.();
        releaseResizeScrollSync = null;
        if (isMarkdownPreviewFile.value && webPreviewVisible.value) {
            scrollSync.syncEditorToPreview();
            refreshMarkdownScrollSync();
        }
    },
});

// Markdown 预览与源文的双向同步滚动控制器。
const scrollSync = createScrollSync(
    () => editor,
    () => markdownPreviewBody.value,
);

const { currentTheme } = useTheme();
const {
    fontSize,
    default: defaultFontSize,
    set: setFontSize,
    reset: resetFontSize,
} = useFontSize();

// 缩放一档的步长倍数。Ctrl+滚轮一次事件通常 deltaY 约 100，
// 但浏览器/系统差异较大；用方向归一化后再乘 step 保证体验稳定。
const FONT_SIZE_WHEEL_STEP = 1;
function handleFontSizeWheel(event) {
    if (!event.ctrlKey && !event.metaKey) {
        return;
    }
    // 拦截浏览器原生缩放行为。
    event.preventDefault();
    event.stopPropagation();
    const direction = event.deltaY < 0 ? 1 : -1;
    setFontSize(fontSize.value + direction * FONT_SIZE_WHEEL_STEP);
}

function bindFontSizeWheel(element) {
    if (!element) {
        return;
    }
    // passive 必须为 false 才能 preventDefault。
    element.addEventListener("wheel", handleFontSizeWheel, { passive: false });
}

function unbindFontSizeWheel(element) {
    if (!element) {
        return;
    }
    element.removeEventListener("wheel", handleFontSizeWheel);
}

function handleFontSizeResetClick() {
    if (fontSize.value === defaultFontSize) {
        return;
    }
    resetFontSize();
}

// 渲染预览区内的 mermaid 图表；需等 v-html 完成 DOM 更新后执行。
// 图表渲染会改变元素高度，期间不能让预览滚动事件反向修改编辑器位置。
async function renderMarkdownMermaid(force = false) {
    if (!isMarkdownPreviewFile.value || !webPreviewVisible.value) {
        return;
    }
    const releaseScrollSync = scrollSync.suspend();
    try {
        await nextTick();
        await renderMermaidDiagrams(markdownPreviewBody.value, force);
        await nextTick();
        scrollSync.syncEditorToPreview();
        refreshMarkdownScrollSync();
    } finally {
        releaseScrollSync();
    }
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

// 只允许最新一次渲染结果写入 DOM。渲染期间保留编辑器滚动位置，
// 完成后从编辑器单向校准预览，避免预览重排触发反向跳动。
async function renderMarkdownPreview() {
    if (!isMarkdownPreviewFile.value || !webPreviewVisible.value) {
        return;
    }

    const token = ++markdownRenderToken;
    const releaseScrollSync = scrollSync.suspend();
    try {
        const html = await renderMarkdown(getContent());
        if (token !== markdownRenderToken || !webPreviewVisible.value) {
            return;
        }

        markdownPreviewHtml.value = html;
        await nextTick();
        await renderMermaidDiagrams(markdownPreviewBody.value);
        await nextTick();

        if (token !== markdownRenderToken || !webPreviewVisible.value) {
            return;
        }
        scrollSync.syncEditorToPreview();
        refreshMarkdownScrollSync();
    } finally {
        releaseScrollSync();
    }
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
        void renderMarkdownPreview();
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
    // 仅当 extension 真的变化（切换到新文件）时，才用 extension/name 重新推断 syntax
    // 并重置「用户主动选择」标记，同时清空分屏预览状态；仅 name 变化时
    // （virtual tab 编辑内容触发）保留用户在下拉里选过的 syntax 与已打开的预览。
    const extensionChanged = extension !== lastSeenExtension;
    lastSeenExtension = extension;
    if (extensionChanged) {
        userOverrodeSyntax = false;
        selectedSyntax.value = detectSyntaxKey(extension, name);
        if (markdownLivePreviewTimer !== null) {
            clearTimeout(markdownLivePreviewTimer);
            markdownLivePreviewTimer = null;
        }
        markdownRenderToken += 1;
        webPreviewVisible.value = false;
        webPreviewContent.value = "";
        markdownPreviewHtml.value = "";
    }
});

watch(
    () => props.contentVersion,
    () => {
        if (!webPreviewVisible.value) {
            return;
        }
        if (isMarkdownPreviewFile.value) {
            void renderMarkdownPreview();
        } else {
            webPreviewContent.value = getContent();
        }
    },
);

watch(
    () => props.encodingLoading,
    () => {
        editor?.dispatch({
            effects: editable.reconfigure(
                EditorView.editable.of(isEditorEditable()),
            ),
        });
    },
);

// 切换自动保存开关：将最新状态抛给父组件持久化与控制保存行为。
function handleAutoSaveChange(event) {
    if (props.encodingLoading || syncingDocument.value) {
        return;
    }
    emit("auto-save-toggle", event.target.checked);
}

const JSON_ERROR_DISPLAY_MS = 4000;

/**
 * 格式化 JSON：将编辑器内容解析后重新缩进，覆盖编辑器文档。
 * 解析失败时在状态栏短暂显示错误提示。
 */
function handleFormatJson() {
    if (syncingDocument.value || !editor) {
        return;
    }
    try {
        const content = getContent();
        const parsed = JSON.parse(content);
        const formatted = JSON.stringify(parsed, null, 2);
        if (formatted === content) {
            return;
        }
        syncingFromProps = true;
        editor.dispatch({
            changes: {
                from: 0,
                to: editor.state.doc.length,
                insert: formatted,
            },
        });
        syncingFromProps = false;
        jsonError.value = "";
        emit("dirty");
    } catch (e) {
        jsonError.value = `JSON 格式错误: ${e.message}`;
        setTimeout(() => {
            jsonError.value = "";
        }, JSON_ERROR_DISPLAY_MS);
    }
}

/**
 * 压缩 JSON：将编辑器内容解析后移除多余空白，覆盖编辑器文档。
 * 解析失败时在状态栏短暂显示错误提示。
 */
function handleCompressJson() {
    if (syncingDocument.value || !editor) {
        return;
    }
    try {
        const content = getContent();
        const parsed = JSON.parse(content);
        const compressed = JSON.stringify(parsed);
        if (compressed === content) {
            return;
        }
        syncingFromProps = true;
        editor.dispatch({
            changes: {
                from: 0,
                to: editor.state.doc.length,
                insert: compressed,
            },
        });
        syncingFromProps = false;
        jsonError.value = "";
        emit("dirty");
    } catch (e) {
        jsonError.value = `JSON 格式错误: ${e.message}`;
        setTimeout(() => {
            jsonError.value = "";
        }, JSON_ERROR_DISPLAY_MS);
    }
}

// 编辑器最终可编辑状态：仅在加载编码过程中临时禁用编辑。
function isEditorEditable() {
    return !props.encodingLoading;
}

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
                            {
                                key: "Mod-Shift-s",
                                preventDefault: true,
                                run: () => {
                                    emit("save-as");
                                    return true;
                                },
                            },
                        ]),
                        EditorView.lineWrapping,
                        editable.of(EditorView.editable.of(isEditorEditable())),
                        EditorView.updateListener.of((update) => {
                            if (!update.docChanged || syncingFromProps) {
                                return;
                            }
                            emit("dirty");
                            scheduleMarkdownLivePreview();
                        }),
                        editorTheme,
                        language.of([]),
                    ],
                }),
                parent: container,
            });
            lastContentVersion = contentVersion;
            lastExtension = extension;
            lastName = name;
            // 编辑器挂载后绑定 Ctrl+滚轮缩放；编辑器 dom 替换会重建此节点。
            bindFontSizeWheel(editor?.dom);
            void configureLanguage();
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
            void configureLanguage();
        }
    },
    { immediate: true, flush: "post" },
);

onBeforeUnmount(() => {
    document.removeEventListener("click", handleDocumentClick);
    document.removeEventListener("keydown", handleDocumentKeydown);
    documentSyncToken += 1;
    markdownRenderToken += 1;
    if (markdownLivePreviewTimer !== null) {
        clearTimeout(markdownLivePreviewTimer);
        markdownLivePreviewTimer = null;
    }
    stopWebPreviewResize();
    scrollSync.dispose();
    unbindFontSizeWheel(editor?.dom);
    editor?.destroy();
    editor = null;
});

// ---- 文件格式 / 编码下拉菜单外部关闭逻辑 ----
function handleDocumentClick(event) {
    if (!syntaxMenuOpen.value && !encodingMenuOpen.value) {
        return;
    }
    // 模板中的根节点上加了 @mousedown.stop，使按钮和菜单自身的点击不会冒泡到这里。
    if (event.target?.closest?.(".code-preview-dropdown")) {
        return;
    }
    closeAllMenus();
}

function handleDocumentKeydown(event) {
    if (event.key === "Escape") {
        closeAllMenus();
    }
}

onMounted(() => {
    document.addEventListener("click", handleDocumentClick);
    document.addEventListener("keydown", handleDocumentKeydown);
});

async function replaceEditorContent(content) {
    const token = ++documentSyncToken;

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
            });
        } else {
            editor.dispatch({
                changes: {
                    from: 0,
                    to: editor.state.doc.length,
                    insert: "",
                },
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
        }

        // 内容就位后异步加载语言并重新配置高亮
        void configureLanguage();
    } finally {
        if (token === documentSyncToken) {
            syncingFromProps = false;
            syncingDocument.value = false;
            editor?.dispatch({
                effects: editable.reconfigure(
                    EditorView.editable.of(isEditorEditable()),
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
    // 标记用户主动通过下拉修改过 syntax，避免后续 name 变化时 watch 用
    // props.extension 推断的默认值把它覆盖回去。
    userOverrodeSyntax = true;
    webPreviewVisible.value = false;
    webPreviewContent.value = "";
    markdownPreviewHtml.value = "";
    markdownRenderToken += 1;
    if (markdownLivePreviewTimer !== null) {
        clearTimeout(markdownLivePreviewTimer);
        markdownLivePreviewTimer = null;
    }
    documentSyncToken += 1;
    void configureLanguage();
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

    webPreviewVisible.value = true;
    void renderMarkdownPreview();
}

// 请求在新标签页中打开当前文件的只读预览，携带编辑器中的最新内容。
function handleOpenInNewTab() {
    if (!showPreviewIcon.value || syncingDocument.value) {
        return;
    }

    emit("open-in-new-tab", {
        content: getContent(),
        extension: props.extension,
        name: props.name,
        syntax: selectedSyntax.value,
    });
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

// 异步加载当前语法键对应的 CodeMirror 语言支持并重新配置编辑器。
// 使用令牌防竞态：仅最后一次调用的结果会生效。
async function configureLanguage() {
    const token = ++languageLoadToken;
    const lang = await resolveLanguageBySyntaxKey(selectedSyntax.value);
    if (token !== languageLoadToken || !editor) {
        return;
    }
    editor.dispatch({
        effects: language.reconfigure(lang),
    });
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
    padding: 4px 4px;
    background: var(--bg-surface);
    color: var(--text-primary);
    font-size: var(--mfv-font-size, 14px);
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
    column-gap: 4px;
}

.code-preview-encoding {
    display: inline-flex;
    align-items: center;
    gap: 4px;
}

/* ---- 文件格式 / 编码 下拉按钮 + 浮动菜单 ---- */
.code-preview-dropdown {
    position: relative;
    display: inline-flex;
    align-items: center;
}

.code-preview-dropdown__btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 22px;
    padding: 0 6px;
    border: 1px solid transparent;
    border-radius: 3px;
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    line-height: 1;
    cursor: pointer;
    outline: none;
    user-select: none;
    max-width: 200px;
}

.code-preview-dropdown__btn:hover:not(:disabled) {
    background: var(--bg-surface);
    color: var(--accent-active);
}

.code-preview-dropdown__btn:focus-visible {
    border-color: var(--accent-primary);
}

.code-preview-dropdown__btn:disabled {
    cursor: not-allowed;
    opacity: 0.6;
}

.code-preview-dropdown__label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.code-preview-dropdown__panel {
    position: absolute;
    bottom: calc(100% + 6px);
    left: 0;
    z-index: 50;
    min-width: 180px;
    max-height: 320px;
    overflow-y: auto;
    line-height: 1.4;
}

.code-preview-dropdown__item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 0;
}

.code-preview-dropdown__item-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.code-preview-dropdown__item-check {
    flex-shrink: 0;
    color: var(--accent-primary);
    font-size: 12px;
}

.code-preview-dropdown__item--active {
    color: var(--accent-active);
}

.code-preview-autosave {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-secondary);
    font-size: 12px;
    cursor: pointer;
    user-select: none;
}

/* 字号显示/重置按钮：默认样式提示当前为默认字号；非默认时变为可点击的强调样式。 */
.code-preview-fontsize {
    display: inline-flex;
    align-items: center;
    height: 22px;
    padding: 0 6px;
    border: 1px solid transparent;
    border-radius: 3px;
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    line-height: 1;
    font-variant-numeric: tabular-nums;
    cursor: pointer;
    outline: none;
    user-select: none;
    transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.code-preview-fontsize:hover {
    background: var(--bg-surface);
    color: var(--accent-active);
}

.code-preview-fontsize:focus-visible {
    border-color: var(--accent-primary);
}

.code-preview-fontsize--default {
    color: var(--text-muted);
    cursor: default;
}

.code-preview-fontsize--default:hover {
    background: transparent;
    color: var(--text-muted);
}

.code-preview-json-error {
    max-width: 320px;
    overflow: hidden;
    color: var(--status-error-text);
    font-size: 12px;
    white-space: nowrap;
    text-overflow: ellipsis;
}

.code-preview-switch {
    position: relative;
    display: inline-flex;
    width: 28px;
    height: 16px;
}

.code-preview-switch__input {
    position: absolute;
    inset: 0;
    margin: 0;
    opacity: 0;
    cursor: pointer;
}

.code-preview-switch__track {
    flex: 1;
    border-radius: 999px;
    background: var(--border-cell);
    transition: background 0.15s ease;
}

.code-preview-switch__thumb {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--bg-surface);
    transition: transform 0.15s ease;
}

.code-preview-switch--on .code-preview-switch__track {
    background: var(--accent-primary);
}

.code-preview-switch--on .code-preview-switch__thumb {
    transform: translateX(12px);
}

.code-preview-switch__input:disabled {
    cursor: not-allowed;
}

.code-preview-switch__input:disabled ~ .code-preview-switch__track {
    opacity: 0.6;
}

.code-preview-switch__input:focus-visible ~ .code-preview-switch__track {
    outline: 1px solid var(--accent-primary);
    outline-offset: 2px;
}

.code-preview-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    padding: 0;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
}

/* 占位弹簧：把其后的按钮组整体推到状态栏右侧 */
.code-preview-spacer {
    flex: 1 1 auto;
}

/* 预览/新标签按钮紧随其后排布，不再各自 auto margin */
.code-preview-action--new-tab {
    margin-left: 2px;
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
