<template>
    <div class="ppt-preview">
        <div class="ppt-preview__toolbar">
            <span>{{ statusText }}</span>
            <span class="ppt-preview__hint">
                Ctrl + 滚轮缩放
            </span>
        </div>
        <div ref="stage" class="ppt-preview__stage" @wheel="handleWheel">
            <div class="ppt-preview__zoom-frame" :style="zoomFrameStyle">
                <div
                    ref="host"
                    class="ppt-preview__render-host"
                    :style="renderHostStyle"
                ></div>
            </div>
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

const props = defineProps({
    src: {
        type: ArrayBuffer,
        default: null,
    },
});

const emit = defineEmits(["error", "rendered"]);

const BASE_WIDTH = 960;
const ZOOM_STEP = 0.1;
const MIN_ZOOM = 0.25;
const MAX_ZOOM = 2.5;
const STAGE_HORIZONTAL_PADDING = 48;

const stage = ref(null);
const host = ref(null);
const loading = ref(false);
const slideCount = ref(0);
const fitScale = ref(1);
const userScale = ref(1);
const contentSize = ref({ width: 0, height: 0 });

let previewer = null;
let renderSequence = 0;
let resizeObserver = null;
let fitFrame = 0;

const zoom = computed(() =>
    clamp(
        Number((fitScale.value * userScale.value).toFixed(4)),
        MIN_ZOOM,
        MAX_ZOOM,
    ),
);

const statusText = computed(() => {
    if (loading.value) {
        return "\u6b63\u5728\u89e3\u6790 PPT...";
    }
    if (slideCount.value > 0) {
        return `\u5171 ${slideCount.value} \u9875 \u00b7 ${Math.round(
            zoom.value * 100,
        )}%`;
    }
    return "PPT \u9884\u89c8";
});

const zoomFrameStyle = computed(() => {
    if (!contentSize.value.width || !contentSize.value.height) {
        return {};
    }

    return {
        width: `${Math.ceil(contentSize.value.width * zoom.value)}px`,
        height: `${Math.ceil(contentSize.value.height * zoom.value)}px`,
    };
});

const renderHostStyle = computed(() => ({
    transform: `scale(${zoom.value})`,
    width: contentSize.value.width ? `${contentSize.value.width}px` : undefined,
}));

function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
}

function getViewportCenterSnapshot() {
    const container = stage.value;
    if (!container) {
        return null;
    }

    return {
        centerX: container.scrollLeft + container.clientWidth / 2,
        centerY: container.scrollTop + container.clientHeight / 2,
        scrollWidth: container.scrollWidth,
        scrollHeight: container.scrollHeight,
    };
}

function restoreViewportCenter(snapshot) {
    const container = stage.value;
    if (!container || !snapshot) {
        return;
    }

    const widthRatio = snapshot.scrollWidth
        ? container.scrollWidth / snapshot.scrollWidth
        : 1;
    const heightRatio = snapshot.scrollHeight
        ? container.scrollHeight / snapshot.scrollHeight
        : 1;

    container.scrollLeft =
        snapshot.centerX * widthRatio - container.clientWidth / 2;
    container.scrollTop =
        snapshot.centerY * heightRatio - container.clientHeight / 2;
}

function centerViewport() {
    const container = stage.value;
    if (!container) {
        return;
    }

    container.scrollLeft = Math.max(
        0,
        (container.scrollWidth - container.clientWidth) / 2,
    );
}

function updateFitScale({ resetUserScale = false, center = false } = {}) {
    const container = stage.value;
    const width = contentSize.value.width;

    if (!container || !width || !container.clientWidth) {
        return;
    }

    const snapshot = center ? null : getViewportCenterSnapshot();
    const availableWidth = Math.max(
        container.clientWidth - STAGE_HORIZONTAL_PADDING,
        1,
    );
    const nextFitScale = clamp(availableWidth / width, MIN_ZOOM, 1);

    if (resetUserScale) {
        userScale.value = 1;
    }

    fitScale.value = Number(nextFitScale.toFixed(4));

    void nextTick(() => {
        if (center) {
            centerViewport();
        } else {
            restoreViewportCenter(snapshot);
        }
    });
}

function queueFitScale(options) {
    if (fitFrame) {
        cancelAnimationFrame(fitFrame);
    }

    fitFrame = requestAnimationFrame(() => {
        fitFrame = 0;
        updateFitScale(options);
    });
}

function updateContentSize() {
    const container = host.value;
    if (!container) {
        contentSize.value = { width: 0, height: 0 };
        return;
    }

    contentSize.value = {
        width: container.scrollWidth,
        height: container.scrollHeight,
    };
}

function createPreviewer(container) {
    previewer?.destroy?.();
    container.replaceChildren();

    return import("pptx-preview").then(({ init }) =>
        init(container, {
            width: BASE_WIDTH,
            mode: "list",
        }),
    );
}

function handleWheel(event) {
    if (!event.ctrlKey) {
        return;
    }

    event.preventDefault();

    const direction = event.deltaY < 0 ? 1 : -1;
    const currentZoom = zoom.value;
    const nextZoom = clamp(
        currentZoom + direction * ZOOM_STEP,
        MIN_ZOOM,
        MAX_ZOOM,
    );

    if (nextZoom !== currentZoom && fitScale.value) {
        const snapshot = getViewportCenterSnapshot();
        userScale.value = Number((nextZoom / fitScale.value).toFixed(4));
        void nextTick(() => restoreViewportCenter(snapshot));
    }
}

function observeStageSize() {
    if (!stage.value || resizeObserver) {
        return;
    }

    resizeObserver = new ResizeObserver(() =>
        queueFitScale({ resetUserScale: false }),
    );
    resizeObserver.observe(stage.value);
}

async function renderSource(source) {
    const currentRender = ++renderSequence;
    const container = host.value;

    if (!container) {
        return;
    }

    previewer?.destroy?.();
    previewer = null;
    container.replaceChildren();
    contentSize.value = { width: 0, height: 0 };
    slideCount.value = 0;
    fitScale.value = 1;
    userScale.value = 1;

    if (!source) {
        loading.value = false;
        return;
    }

    loading.value = true;

    try {
        const nextPreviewer = await createPreviewer(container);

        if (currentRender !== renderSequence) {
            nextPreviewer?.destroy?.();
            return;
        }

        const pptx = await nextPreviewer.preview(source);

        if (currentRender !== renderSequence) {
            nextPreviewer?.destroy?.();
            return;
        }

        previewer = nextPreviewer;
        slideCount.value =
            pptx?.slides?.length ?? nextPreviewer.slideCount ?? 0;
        await nextTick();
        updateContentSize();
        updateFitScale({ resetUserScale: true, center: true });
        emit("rendered");
    } catch (error) {
        if (currentRender !== renderSequence) {
            return;
        }

        container.replaceChildren();
        emit("error", error);
    } finally {
        if (currentRender === renderSequence) {
            loading.value = false;
        }
    }
}

watch(
    [() => props.src, host],
    ([nextSource]) => {
        if (!nextSource) {
            return;
        }

        fitScale.value = 1;
        userScale.value = 1;
        void renderSource(nextSource);
    },
    { immediate: true, flush: "post" },
);

onMounted(() => {
    observeStageSize();
});

onBeforeUnmount(() => {
    renderSequence += 1;

    if (fitFrame) {
        cancelAnimationFrame(fitFrame);
        fitFrame = 0;
    }

    resizeObserver?.disconnect();
    resizeObserver = null;

    previewer?.destroy?.();
    previewer = null;
    host.value?.replaceChildren();
    contentSize.value = { width: 0, height: 0 };
    slideCount.value = 0;
});
</script>

<style scoped>
.ppt-preview {
    display: flex;
    flex: 1;
    min-width: 0;
    min-height: 0;
    height: 100%;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-preview);
    color: var(--text-heading);
}

.ppt-preview__toolbar {
    display: flex;
    min-height: 42px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border-toolbar);
    background: var(--bg-toolbar-ppt);
    font-size: 13px;
    color: var(--text-toolbar);
}

.ppt-preview__hint {
    color: var(--text-disabled);
}

.ppt-preview__stage {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    padding: 24px 0;
}

.ppt-preview__zoom-frame {
    position: relative;
    width: fit-content;
    margin: 0 auto;
}

.ppt-preview__render-host {
    width: fit-content;
    transform-origin: top left;
}

.ppt-preview__stage :deep(.pptx-preview-wrapper) {
    max-width: none;
    background: transparent !important;
}

.ppt-preview__stage :deep(.pptx-preview-slide-wrapper) {
    box-shadow: var(--shadow-page);
}

@media (max-width: 768px) {
    .ppt-preview {
        min-height: 360px;
    }

    .ppt-preview__toolbar {
        min-height: 38px;
        padding: 0 12px;
    }

    .ppt-preview__stage {
        padding: 16px 0;
    }
}
</style>
