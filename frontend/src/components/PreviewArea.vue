<template>
    <section class="workspace__preview">
        <div class="pane-container pane-card--preview">
            <!-- 分屏布局树：单 pane 时退化为一个 pane 节点，与分屏前结构一致 -->
            <PreviewSplitNode :node="layout" />
        </div>
    </section>
</template>

<script setup>
import { computed } from "vue";
import PreviewSplitNode from "./PreviewSplitNode.vue";
import { providePreviewPaneContext } from "../composables/usePreviewPaneContext";

const props = defineProps({
    tabs: {
        type: Array,
        default: () => [],
    },
    layout: {
        type: Object,
        required: true,
    },
    focusedPaneId: {
        type: String,
        default: "",
    },
    paneCount: {
        type: Number,
        default: 1,
    },
    activeTabPath: {
        type: String,
        default: "",
    },
    autoSaveEnabled: {
        type: Boolean,
        default: true,
    },
    audioQueueMode: {
        type: String,
        default: "off",
    },
    audioHasPrev: {
        type: Boolean,
        default: false,
    },
    audioHasNext: {
        type: Boolean,
        default: false,
    },
});

const emit = defineEmits([
    "change-tab",
    "close-tab",
    "preview-error",
    "preview-rendered",
    "content-change",
    "encoding-change",
    "save-tab",
    "save-as-tab",
    "open-in-new-tab",
    "reorder-tab",
    "move-tab",
    "split-tab",
    "focus-pane",
    "resize-split",
    "new-tab",
    "auto-save-toggle",
    "media-error",
    "media-reload",
    "media-open-system",
    "request-prev",
    "request-next",
    "set-queue-mode",
]);

// paneId → PreviewTabs 实例：分屏后可能同时存在多个实例。
const paneTabsRefs = new Map();

function registerPaneTabs(paneId, component) {
    if (component) {
        paneTabsRefs.set(paneId, component);
    } else {
        paneTabsRefs.delete(paneId);
    }
}

const EMPTY_TABS = [];
// 缓存每个 pane 的 tab 数组：只要 openTabs 引用与 pane 内路径顺序都没变就复用同一数组。
// 分屏后父组件（PreviewArea）会因焦点 / 落点等状态频繁重渲染，若每次都返回新数组，
// 子组件的 tabs prop 就会不断变化，layui 的 tab 头跟着重新渲染 —— 拖拽期间的重渲染
// 会打断正在进行的拖动（源元素被重建、dragend 丢失）。
const paneTabsCache = new Map();

/** pane 的 tab 列表：顺序取自布局，内容取自全局 openTabs。 */
function tabsOfPane(pane) {
    if (!pane?.tabPaths?.length) {
        return EMPTY_TABS;
    }
    const pathsKey = pane.tabPaths.join("\u0000");
    const cached = paneTabsCache.get(pane.id);
    if (cached && cached.tabsRef === props.tabs && cached.pathsKey === pathsKey) {
        return cached.list;
    }
    const byPath = new Map(props.tabs.map((tab) => [tab.path, tab]));
    const list = pane.tabPaths.map((path) => byPath.get(path)).filter(Boolean);
    paneTabsCache.set(pane.id, { tabsRef: props.tabs, pathsKey, list });
    return list;
}

// 注入给递归的 PreviewSplitNode：pane 数据访问 + 事件上抛。
providePreviewPaneContext({
    focusedPaneId: computed(() => props.focusedPaneId),
    paneCount: computed(() => props.paneCount),
    autoSaveEnabled: computed(() => props.autoSaveEnabled),
    audioQueueMode: computed(() => props.audioQueueMode),
    audioHasPrev: computed(() => props.audioHasPrev),
    audioHasNext: computed(() => props.audioHasNext),
    tabsOfPane,
    registerPaneTabs,
    setRatio: (splitId, ratio) => emit("resize-split", { splitId, ratio }),
    handlers: {
        onChangeTab: (paneId, path) => emit("change-tab", paneId, path),
        onCloseTab: (path) => emit("close-tab", path),
        onNewTab: () => emit("new-tab"),
        onReorderTab: (payload) => emit("reorder-tab", payload),
        onMoveTab: (payload) => emit("move-tab", payload),
        onSplitTab: (payload) => emit("split-tab", payload),
        onFocusPane: (paneId) => emit("focus-pane", paneId),
        onPreviewError: (...args) => emit("preview-error", ...args),
        onPreviewRendered: (...args) => emit("preview-rendered", ...args),
        onContentChange: (...args) => emit("content-change", ...args),
        onEncodingChange: (...args) => emit("encoding-change", ...args),
        onSaveTab: (...args) => emit("save-tab", ...args),
        onSaveAsTab: (...args) => emit("save-as-tab", ...args),
        onOpenInNewTab: (...args) => emit("open-in-new-tab", ...args),
        onAutoSaveToggle: (...args) => emit("auto-save-toggle", ...args),
        onMediaError: (...args) => emit("media-error", ...args),
        onMediaReload: (...args) => emit("media-reload", ...args),
        onMediaOpenSystem: (...args) => emit("media-open-system", ...args),
        onRequestPrev: (...args) => emit("request-prev", ...args),
        onRequestNext: (...args) => emit("request-next", ...args),
        onSetQueueMode: (...args) => emit("set-queue-mode", ...args),
    },
});

// 透传各 pane 的 getCodeContent，供 App 侧「编辑即预览」/未命名 tab 命名读取编辑器内容。
function getCodeContent(path) {
    for (const component of paneTabsRefs.values()) {
        const content = component?.getCodeContent?.(path);
        if (content !== undefined) {
            return content;
        }
    }
    return undefined;
}

defineExpose({
    getCodeContent,
});
</script>

<style scoped>
/* 左右 4px 留空已上移到外部容器 .workspace__body,这里仅保留底部 4px,
   让预览大容器的边缘样式和左侧文件列表卡片保持一致。 */
.workspace__preview {
    padding: 0 0 4px;
    background: var(--bg-titlebar);
}

.pane-container {
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background-color: var(--bg-surface);
}
</style>
