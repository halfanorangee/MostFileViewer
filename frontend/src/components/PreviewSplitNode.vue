<template>
    <!-- 分割节点：两个子节点 + 可拖拽分隔条 -->
    <div
        v-if="node.type === 'split'"
        ref="splitRoot"
        class="preview-split"
        :class="[
            node.direction === 'column'
                ? 'preview-split--column'
                : 'preview-split--row',
            { 'preview-split--resizing': resizing },
        ]"
    >
        <div class="preview-split__child" :style="firstStyle">
            <PreviewSplitNode :node="node.children[0]" :depth="depth + 1" />
        </div>
        <div
            class="preview-split__divider"
            :class="{ 'preview-split__divider--active': resizing }"
            role="separator"
            :aria-orientation="node.direction === 'column' ? 'horizontal' : 'vertical'"
            @pointerdown.stop.prevent="startResize"
            @dblclick.stop.prevent="resetRatio"
        ></div>
        <div class="preview-split__child preview-split__child--fill">
            <PreviewSplitNode :node="node.children[1]" :depth="depth + 1" />
        </div>
    </div>

    <!-- 叶子节点：一个 pane（tab 条 + 预览内容） -->
    <div
        v-else
        class="preview-split__pane"
        :class="{ 'preview-split__pane--unfocused': isMultiPane && node.id !== focusedPaneId }"
    >
        <PreviewTabs
            :ref="(component) => registerPaneTabs(node.id, component)"
            :tabs="tabsOfPane(node)"
            :active-tab-path="node.activePath"
            :pane-id="node.id"
            :auto-save-enabled="context.autoSaveEnabled.value"
            :audio-queue-mode="context.audioQueueMode.value"
            :audio-has-prev="context.audioHasPrev.value"
            :audio-has-next="context.audioHasNext.value"
            @change-tab="(path) => context.handlers.onChangeTab(node.id, path)"
            @close-tab="(path) => context.handlers.onCloseTab(path)"
            @new-tab="() => context.handlers.onNewTab()"
            @reorder-tab="(payload) => context.handlers.onReorderTab(payload)"
            @move-tab="(payload) => context.handlers.onMoveTab(payload)"
            @split-tab="(payload) => context.handlers.onSplitTab(payload)"
            @focus-pane="() => context.handlers.onFocusPane(node.id)"
            @preview-error="(...args) => context.handlers.onPreviewError(...args)"
            @preview-rendered="(...args) => context.handlers.onPreviewRendered(...args)"
            @content-change="(...args) => context.handlers.onContentChange(...args)"
            @encoding-change="(...args) => context.handlers.onEncodingChange(...args)"
            @save-tab="(...args) => context.handlers.onSaveTab(...args)"
            @save-as-tab="(...args) => context.handlers.onSaveAsTab(...args)"
            @open-in-new-tab="(...args) => context.handlers.onOpenInNewTab(...args)"
            @auto-save-toggle="(enabled) => context.handlers.onAutoSaveToggle(enabled)"
            @media-error="(...args) => context.handlers.onMediaError(...args)"
            @media-reload="(...args) => context.handlers.onMediaReload(...args)"
            @media-open-system="(...args) => context.handlers.onMediaOpenSystem(...args)"
            @request-prev="(...args) => context.handlers.onRequestPrev(...args)"
            @request-next="(...args) => context.handlers.onRequestNext(...args)"
            @set-queue-mode="(mode) => context.handlers.onSetQueueMode(mode)"
        />
    </div>
</template>

<script setup>
import { computed, inject, onBeforeUnmount, ref } from "vue";
import PreviewTabs from "./PreviewTabs.vue";
import { PREVIEW_PANE_CONTEXT } from "../composables/usePreviewPaneContext";

defineOptions({ name: "PreviewSplitNode" });

const props = defineProps({
    node: {
        type: Object,
        required: true,
    },
    depth: {
        type: Number,
        default: 0,
    },
});

// 递归组件依赖 PreviewArea 注入的共享上下文（tab 数据、事件、焦点等）。
const context = inject(PREVIEW_PANE_CONTEXT);

const splitRoot = ref(null);
const resizing = ref(false);
let dragState = null;

const focusedPaneId = computed(() => context.focusedPaneId.value);
const isMultiPane = computed(() => context.paneCount.value > 1);

const firstStyle = computed(() => ({
    flexBasis: `${Math.round((props.node.ratio || 0.5) * 10000) / 100}%`,
}));

/** pane 的 tab 列表（顺序取自布局，内容取自全局 openTabs）。 */
function tabsOfPane(pane) {
    return context.tabsOfPane(pane);
}

function registerPaneTabs(paneId, component) {
    context.registerPaneTabs(paneId, component);
}

// ---- 分隔条拖拽调整比例 ----
// 用 pointer 事件 + window 监听：指针移出分隔条后仍可继续拖动。
function startResize() {
    const root = splitRoot.value;
    if (!root) {
        return;
    }
    dragState = {
        horizontal: props.node.direction !== "column",
        rect: root.getBoundingClientRect(),
    };
    resizing.value = true;
    window.addEventListener("pointermove", onResizeMove);
    window.addEventListener("pointerup", stopResize);
    window.addEventListener("pointercancel", stopResize);
}

function onResizeMove(event) {
    if (!dragState) return;
    const { horizontal, rect } = dragState;
    const total = horizontal ? rect.width : rect.height;
    if (!total) return;
    const offset = horizontal ? event.clientX - rect.left : event.clientY - rect.top;
    context.setRatio(props.node.id, offset / total);
}

function resetRatio() {
    context.setRatio(props.node.id, 0.5);
}

function stopResize() {
    resizing.value = false;
    dragState = null;
    window.removeEventListener("pointermove", onResizeMove);
    window.removeEventListener("pointerup", stopResize);
    window.removeEventListener("pointercancel", stopResize);
}

onBeforeUnmount(stopResize);
</script>
