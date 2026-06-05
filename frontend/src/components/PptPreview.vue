<template>
    <div class="ppt-preview">
        <div class="ppt-preview__toolbar">
            <span>{{ statusText }}</span>
            <span class="ppt-preview__hint">Ctrl + 滚轮缩放</span>
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
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";

const props = defineProps({
    src: {
        type: ArrayBuffer,
        default: null,
    },
});

const emit = defineEmits(["error"]);

const BASE_WIDTH = 960;
const ZOOM_STEP = 0.1;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2.5;

const stage = ref(null);
const host = ref(null);
const loading = ref(false);
const slideCount = ref(0);
const zoom = ref(1);
const contentSize = ref({ width: 0, height: 0 });

let previewer = null;
let renderSequence = 0;

const statusText = computed(() => {
    if (loading.value) {
        return "正在解析 PPT...";
    }
    if (slideCount.value > 0) {
        return `共 ${slideCount.value} 页 · ${Math.round(zoom.value * 100)}%`;
    }
    return "PPT 预览";
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

function getPreviewWidth() {
    const container = stage.value;
    if (!container) {
        return BASE_WIDTH;
    }

    const availableWidth = Math.max(container.clientWidth - 48, 320);
    return Math.round(availableWidth);
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
            width: getPreviewWidth(),
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
    const nextZoom = clamp(
        Number((zoom.value + direction * ZOOM_STEP).toFixed(2)),
        MIN_ZOOM,
        MAX_ZOOM,
    );

    if (nextZoom !== zoom.value) {
        zoom.value = nextZoom;
    }
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
        zoom.value = 1;
        void renderSource(nextSource);
    },
    { immediate: true, flush: "post" },
);

onBeforeUnmount(() => {
    renderSequence += 1;
    previewer?.destroy?.();
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
    background: #edf2f7;
    color: #111827;
}

.ppt-preview__toolbar {
    display: flex;
    min-height: 42px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 0 16px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.28);
    background: rgba(255, 255, 255, 0.86);
    font-size: 13px;
    color: #475569;
}

.ppt-preview__hint {
    color: #94a3b8;
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
    box-shadow: 0 18px 40px rgba(15, 23, 42, 0.16);
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
