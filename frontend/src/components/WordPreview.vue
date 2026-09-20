<template>
    <div
        ref="host"
        class="word-preview"
        @wheel.capture="handleWheel"
    ></div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useTheme } from "../composables/useTheme";
import {
    OFFICE_WORKER_TIMEOUT_MS,
    cloneOfficeSource,
    getOfficeWasmUrl,
    normalizeOfficePreviewError,
    waitForRenderableHost,
} from "./officePreviewUtils";

const props = defineProps({
    src: {
        type: ArrayBuffer,
        default: null,
    },
});

const emit = defineEmits(["error"]);

const MIN_ZOOM = 0.1;
const MAX_ZOOM = 4;
const WHEEL_ZOOM_DISTANCE = 1000;

const host = ref(null);
let viewer = null;
let renderSequence = 0;
let zoomFrame = 0;
let pendingZoomDelta = 0;
let zoomAnchor = null;

const { currentTheme } = useTheme();

function getThemeBackground() {
    const value = getComputedStyle(document.documentElement)
        .getPropertyValue("--bg-preview")
        .trim();
    return value || "#f0f0f0";
}

function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
}

function getScrollHost() {
    return host.value?.firstElementChild?.firstElementChild ?? null;
}

function getPageWrapper(element, scrollHost) {
    let current = element;

    while (current && current.parentElement !== scrollHost) {
        current = current.parentElement;
    }

    if (!current || current === scrollHost?.firstElementChild) {
        return null;
    }

    return current;
}

function captureZoomAnchor(event) {
    const scrollHost = getScrollHost();

    if (!scrollHost) {
        return null;
    }

    const rect = scrollHost.getBoundingClientRect();
    const viewportX = clamp(event.clientX - rect.left, 0, rect.width);
    const viewportY = clamp(event.clientY - rect.top, 0, rect.height);
    const page = getPageWrapper(
        document.elementFromPoint(event.clientX, event.clientY),
        scrollHost,
    );
    const pageRect = page?.getBoundingClientRect();

    return {
        scrollHost,
        viewportX,
        viewportY,
        page,
        pageX: pageRect?.width
            ? clamp((event.clientX - pageRect.left) / pageRect.width, 0, 1)
            : 0,
        pageY: pageRect?.height
            ? clamp((event.clientY - pageRect.top) / pageRect.height, 0, 1)
            : 0,
        contentX: scrollHost.scrollLeft + viewportX,
        contentY: scrollHost.scrollTop + viewportY,
    };
}

function restoreZoomAnchor(anchor, previousScale, nextScale) {
    const scrollHost = anchor?.scrollHost;

    if (!scrollHost?.isConnected) {
        return;
    }

    const page = anchor.page;
    const pageIsMounted = page?.isConnected && page.parentElement === scrollHost;
    const scaleRatio = previousScale > 0 ? nextScale / previousScale : 1;
    const contentX = pageIsMounted
        ? page.offsetLeft + page.offsetWidth * anchor.pageX
        : anchor.contentX * scaleRatio;
    const contentY = pageIsMounted
        ? page.offsetTop + page.offsetHeight * anchor.pageY
        : anchor.contentY * scaleRatio;
    const maxScrollLeft = Math.max(
        0,
        scrollHost.scrollWidth - scrollHost.clientWidth,
    );
    const maxScrollTop = Math.max(
        0,
        scrollHost.scrollHeight - scrollHost.clientHeight,
    );

    scrollHost.scrollLeft = clamp(
        contentX - anchor.viewportX,
        0,
        maxScrollLeft,
    );
    scrollHost.scrollTop = clamp(
        contentY - anchor.viewportY,
        0,
        maxScrollTop,
    );
}

function cancelZoomAnimation() {
    if (zoomFrame) {
        cancelAnimationFrame(zoomFrame);
        zoomFrame = 0;
    }

    pendingZoomDelta = 0;
    zoomAnchor = null;
}

function scheduleZoomFrame() {
    if (!zoomFrame) {
        zoomFrame = requestAnimationFrame(applyPendingZoom);
    }
}

function applyPendingZoom() {
    zoomFrame = 0;

    if (!viewer || pendingZoomDelta === 0) {
        return;
    }

    const currentScale = viewer.getScale();
    const delta = pendingZoomDelta;
    const anchor = zoomAnchor;

    pendingZoomDelta = 0;
    zoomAnchor = null;

    const nextScale = clamp(
        currentScale - delta / WHEEL_ZOOM_DISTANCE,
        MIN_ZOOM,
        MAX_ZOOM,
    );

    viewer.setScale(nextScale);
    restoreZoomAnchor(anchor, currentScale, nextScale);
}

function getWheelDelta(event) {
    if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
        return event.deltaY * 16;
    }

    if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
        return event.deltaY * (getScrollHost()?.clientHeight || 800);
    }

    return event.deltaY;
}

function handleWheel(event) {
    if (!(event.ctrlKey || event.metaKey) || event.deltaY === 0 || !viewer) {
        return;
    }

    event.preventDefault();
    event.stopPropagation();

    pendingZoomDelta += getWheelDelta(event);
    zoomAnchor = captureZoomAnchor(event) ?? zoomAnchor;
    scheduleZoomFrame();
}

async function render(source) {
    const currentRender = ++renderSequence;

    cancelZoomAnimation();
    viewer?.destroy();
    viewer = null;

    if (!source || !host.value) return;

    // 兜底：byteLength 为 0 意味着缓冲区已被 detach（历史版本 load() 会消费
    // 传入的 buffer），此时解析只会得到无意义的报错，直接给出可读提示。
    if (source.byteLength === 0) {
        emit("error", "文档数据已失效，请关闭后重新打开该标签");
        return;
    }

    try {
        if (!(await waitForRenderableHost(host.value))) {
            if (currentRender === renderSequence) {
                emit("error", "预览区域尺寸异常，无法渲染 Word 文档");
            }
            return;
        }

        const { DocxScrollViewer } = await import("@silurus/ooxml/docx");

        if (currentRender !== renderSequence) return;

        viewer = new DocxScrollViewer(host.value, {
            background: getThemeBackground(),
            gap: 24,
            paddingTop: 28,
            enableTextSelection: true,
            enableZoom: false,
            zoomMin: MIN_ZOOM,
            zoomMax: MAX_ZOOM,
            wasmUrl: getOfficeWasmUrl("word"),
            workerTimeoutMs: OFFICE_WORKER_TIMEOUT_MS,
            onError: (err) => emit("error", normalizeOfficePreviewError(err)),
        });

        // load() 会把 buffer 作为 transferable 交给解析 worker（随后原缓冲区被
        // detach）。tab.source 是共享数据源（分屏/移动 tab 的重挂载、主题变化、
        // 会话恢复后的重渲染都会复用它），这里每次复制副本交给库，保证
        // tab.source 永远不被消费、可重复渲染。
        await viewer.load(cloneOfficeSource(source));

        if (currentRender !== renderSequence) {
            viewer?.destroy();
            viewer = null;
        }
    } catch (error) {
        if (currentRender !== renderSequence) return;
        viewer?.destroy();
        viewer = null;
        emit("error", normalizeOfficePreviewError(error));
    }
}

// 首次渲染在 onMounted 中触发，确保 host 模板 ref 已绑定到 DOM。
// watch 仅负责后续 src 变化时重新渲染（跳过初始值，避免与 onMounted 重复）。
watch(
    () => props.src,
    (source) => {
        if (source) render(source);
    },
);

onMounted(() => {
    if (props.src) render(props.src);
});

watch(currentTheme, () => {
    if (props.src) render(props.src);
});

onBeforeUnmount(() => {
    renderSequence += 1;
    cancelZoomAnimation();
    viewer?.destroy();
    viewer = null;
});
</script>

<style scoped>
.word-preview {
    flex: 1;
    min-width: 0;
    min-height: 0;
    height: 100%;
    overflow: hidden;
}
</style>
