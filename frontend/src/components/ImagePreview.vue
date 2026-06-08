<template>
    <div class="image-preview" @wheel="handleWheel">
        <div class="image-preview__toolbar">
            <span>{{ statusText }}</span>
            <span class="image-preview__hint">滚轮缩放</span>
        </div>

        <div
            ref="stageRef"
            class="image-preview__stage"
            :class="{
                'image-preview__stage--draggable': objectUrl,
                'image-preview__stage--dragging': dragging,
            }"
            @mousedown="startDrag"
        >
            <img
                v-if="objectUrl"
                ref="imageRef"
                class="image-preview__image"
                :src="objectUrl"
                :alt="name || '图片预览'"
                :style="imageStyle"
                @load="handleLoad"
                @error="handleImageError"
            />
            <div v-else class="image-preview__empty">暂无图片内容</div>
        </div>
    </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";

const props = defineProps({
    src: {
        type: ArrayBuffer,
        default: null,
    },
    extension: {
        type: String,
        default: "",
    },
    name: {
        type: String,
        default: "",
    },
});

const emit = defineEmits(["error"]);

const MIME_BY_EXTENSION = {
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".webp": "image/webp",
    ".bmp": "image/bmp",
    ".svg": "image/svg+xml",
    ".ico": "image/x-icon",
    ".avif": "image/avif",
};
const ZOOM_STEP = 0.1;
const MIN_ZOOM = 0.1;
const MAX_ZOOM = 8;

const objectUrl = ref("");
const zoom = ref(1);
const naturalSize = ref({ width: 0, height: 0 });
const pan = ref({ x: 0, y: 0 });
const dragging = ref(false);
const stageRef = ref(null);
const imageRef = ref(null);

let dragStart = { x: 0, y: 0 };
let panStart = { x: 0, y: 0 };

const imageStyle = computed(() => ({
    transform: `translate(${pan.value.x}px, ${pan.value.y}px) scale(${zoom.value})`,
}));

const statusText = computed(() => {
    const size = naturalSize.value;
    const zoomPercent = `${Math.round(zoom.value * 100)}%`;
    if (size.width && size.height) {
        return `${size.width} × ${size.height} · ${zoomPercent}`;
    }
    return `图片预览 · ${zoomPercent}`;
});

watch(
    () => [props.src, props.extension],
    ([source]) => {
        revokeObjectUrl();
        zoom.value = 1;
        pan.value = { x: 0, y: 0 };
        stopDrag();
        naturalSize.value = { width: 0, height: 0 };

        if (!source) {
            return;
        }

        const blob = new Blob([source], { type: getMimeType(props.extension) });
        objectUrl.value = URL.createObjectURL(blob);
    },
    { immediate: true },
);

function getMimeType(extension) {
    const normalized = String(extension || "").toLowerCase();
    return MIME_BY_EXTENSION[normalized] || "image/*";
}

function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
}

function handleWheel(event) {
    event.preventDefault();

    const direction = event.deltaY < 0 ? 1 : -1;
    zoom.value = clamp(
        Number((zoom.value + direction * ZOOM_STEP).toFixed(2)),
        MIN_ZOOM,
        MAX_ZOOM,
    );

    if (zoom.value === 1) {
        pan.value = { x: 0, y: 0 };
        return;
    }

    pan.value = clampPan(pan.value);
}

function startDrag(event) {
    if (!objectUrl.value || event.button !== 0) {
        return;
    }

    event.preventDefault();
    dragging.value = true;
    dragStart = { x: event.clientX, y: event.clientY };
    panStart = { ...pan.value };

    window.addEventListener("mousemove", handleDragMove);
    window.addEventListener("mouseup", stopDrag);
}

function handleDragMove(event) {
    if (!dragging.value) {
        return;
    }

    pan.value = clampPan({
        x: panStart.x + event.clientX - dragStart.x,
        y: panStart.y + event.clientY - dragStart.y,
    });
}

function clampPan(nextPan) {
    const stage = stageRef.value;
    const image = imageRef.value;
    if (!stage || !image) {
        return nextPan;
    }

    const stageRect = stage.getBoundingClientRect();
    const imageRect = image.getBoundingClientRect();
    const scaledWidth = imageRect.width;
    const scaledHeight = imageRect.height;
    const maxX = Math.max(0, (scaledWidth - stageRect.width) / 2);
    const maxY = Math.max(0, (scaledHeight - stageRect.height) / 2);

    return {
        x: clamp(nextPan.x, -maxX, maxX),
        y: clamp(nextPan.y, -maxY, maxY),
    };
}

function stopDrag() {
    if (!dragging.value) {
        return;
    }

    dragging.value = false;
    window.removeEventListener("mousemove", handleDragMove);
    window.removeEventListener("mouseup", stopDrag);
}

function handleLoad(event) {
    naturalSize.value = {
        width: event.target?.naturalWidth || 0,
        height: event.target?.naturalHeight || 0,
    };
    pan.value = clampPan(pan.value);
}

function handleImageError(error) {
    emit("error", error);
}

function revokeObjectUrl() {
    if (objectUrl.value) {
        URL.revokeObjectURL(objectUrl.value);
        objectUrl.value = "";
    }
}

onBeforeUnmount(() => {
    stopDrag();
    revokeObjectUrl();
});
</script>

<style scoped>
.image-preview {
    display: flex;
    flex: 1;
    min-width: 0;
    min-height: 0;
    height: 100%;
    flex-direction: column;
    overflow: hidden;
    background: #f3f4f6;
    color: #111827;
}

.image-preview__toolbar {
    display: flex;
    height: 42px;
    flex-shrink: 0;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 0 16px;
    border-bottom: 1px solid rgba(17, 24, 39, 0.1);
    background: rgba(255, 255, 255, 0.92);
    font-size: 13px;
}

.image-preview__hint {
    color: #6b7280;
}

.image-preview__stage {
    display: flex;
    flex: 1;
    min-width: 0;
    min-height: 0;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    padding: 32px;
    user-select: none;
}

.image-preview__stage--draggable {
    cursor: grab;
}

.image-preview__stage--dragging {
    cursor: grabbing;
}

.image-preview__image {
    max-width: 100%;
    max-height: 100%;
    transform-origin: center center;
    transition: transform 120ms ease;
    object-fit: contain;
    image-rendering: auto;
    pointer-events: none;
}

.image-preview__empty {
    color: #6b7280;
}
</style>
