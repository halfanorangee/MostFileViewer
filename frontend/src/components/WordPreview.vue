<template>
    <div ref="host" class="word-preview" @wheel="handleWheel"></div>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref, watch } from "vue";

const props = defineProps({
    src: {
        type: ArrayBuffer,
        default: null,
    },
});

const ZOOM_STEP = 0.1;
const MIN_USER_SCALE = 0.5;
const MAX_USER_SCALE = 3;

const emit = defineEmits(["error"]);
const host = ref(null);
let renderSequence = 0;
let fitFrame = 0;
let userScale = 1;

function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
}

function queueFitPages() {
    if (fitFrame) {
        cancelAnimationFrame(fitFrame);
    }

    fitFrame = requestAnimationFrame(() => {
        fitFrame = 0;
        fitPagesToContainer();
    });
}

function fitPagesToContainer() {
    const container = host.value;
    const wrapper = container?.querySelector(".docx-wrapper");

    if (!container || !wrapper) {
        return;
    }

    const firstPage = wrapper.querySelector("section.docx");

    if (!firstPage) {
        return;
    }

    wrapper.style.setProperty("--docx-page-scale", "1");

    const wrapperStyle = getComputedStyle(wrapper);
    const horizontalPadding =
        parseFloat(wrapperStyle.paddingLeft || "0") +
        parseFloat(wrapperStyle.paddingRight || "0");
    const availableWidth = Math.max(
        0,
        container.clientWidth - horizontalPadding,
    );
    const pageWidth = firstPage.getBoundingClientRect().width;
    const fitScale = Math.min(1, availableWidth / pageWidth);
    const scale = fitScale * userScale;
    const pageScale = Number.isFinite(scale) ? Math.max(0.1, scale) : 1;
    const contentWidth = Math.ceil(pageWidth * pageScale + horizontalPadding);

    wrapper.style.setProperty("--docx-page-scale", pageScale.toFixed(4));
    wrapper.style.minWidth = `max(100%, ${contentWidth}px)`;
}

function handleWheel(event) {
    if (!event.ctrlKey) {
        return;
    }

    event.preventDefault();

    const direction = event.deltaY < 0 ? 1 : -1;
    userScale = clamp(
        Number((userScale + direction * ZOOM_STEP).toFixed(2)),
        MIN_USER_SCALE,
        MAX_USER_SCALE,
    );
    queueFitPages();
}

watch(
    [() => props.src, host],
    async ([nextSource, container]) => {
        const currentRender = ++renderSequence;

        if (!container) {
            return;
        }

        container.replaceChildren();
        userScale = 1;

        if (!nextSource) {
            return;
        }

        const renderRoot = document.createElement("div");

        try {
            const { renderAsync } = await import("docx-preview");

            await renderAsync(nextSource, renderRoot, renderRoot, {
                className: "docx",
                inWrapper: true,
                breakPages: true,
                ignoreLastRenderedPageBreak: true,
                renderHeaders: true,
                renderFooters: true,
                renderFootnotes: true,
                renderEndnotes: true,
                renderComments: true,
                renderAltChunks: true,
                experimental: true,
                useBase64URL: true,
            });

            if (currentRender !== renderSequence) {
                return;
            }

            container.replaceChildren(...Array.from(renderRoot.childNodes));
            await nextTick();
            queueFitPages();
        } catch (error) {
            if (currentRender !== renderSequence) {
                return;
            }

            container.replaceChildren();
            emit("error", error);
        }
    },
    { immediate: true, flush: "post" },
);

onBeforeUnmount(() => {
    renderSequence += 1;

    if (fitFrame) {
        cancelAnimationFrame(fitFrame);
        fitFrame = 0;
    }

    host.value?.replaceChildren();
});
</script>

<style scoped>
.word-preview {
    --word-preview-background:
        radial-gradient(
            circle at top,
            rgba(255, 255, 255, 0.72),
            transparent 32%
        ),
        #edf2f7;
    flex: 1;
    min-width: 0;
    min-height: 0;
    height: 100%;
    overflow: auto;
    background: var(--word-preview-background);
    color: #111827;
}

.word-preview :deep(.docx-wrapper) {
    --docx-page-scale: 1;
    min-width: 100%;
    min-height: 100%;
    padding: 28px;
    background: var(--word-preview-background);
}

.word-preview :deep(.docx-wrapper > section.docx) {
    margin-right: auto !important;
    margin-left: auto !important;
    box-shadow: 0 18px 40px rgba(15, 23, 42, 0.16);
    zoom: var(--docx-page-scale);
}

.word-preview :deep(.docx-wrapper > section.docx:not(:last-child)) {
    margin-bottom: max(18px, calc(28px * var(--docx-page-scale))) !important;
}

.word-preview :deep(.docx-wrapper img) {
    max-width: 100%;
}

@media (max-width: 768px) {
    .word-preview {
        min-height: 360px;
    }

    .word-preview :deep(.docx-wrapper) {
        padding: 16px;
    }
}
</style>
