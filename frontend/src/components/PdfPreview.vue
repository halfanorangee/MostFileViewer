<template>
    <div class="pdf-preview">
        <iframe
            v-if="objectUrl"
            class="pdf-preview__frame"
            :src="objectUrl"
            :title="name || 'PDF 预览'"
            @load="loaded = true"
            @error="handleFrameError"
        ></iframe>
        <div v-else class="pdf-preview__empty">暂无 PDF 内容</div>
    </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";

const props = defineProps({
    src: {
        type: ArrayBuffer,
        default: null,
    },
    name: {
        type: String,
        default: "",
    },
});

const emit = defineEmits(["error"]);

const objectUrl = ref("");
const loaded = ref(false);

const statusText = computed(() => {
    if (!objectUrl.value) {
        return "PDF 预览";
    }
    return loaded.value ? "PDF 预览" : "正在加载 PDF...";
});

watch(
    () => props.src,
    (source) => {
        revokeObjectUrl();
        loaded.value = false;

        if (!source) {
            return;
        }

        const blob = new Blob([source], { type: "application/pdf" });
        objectUrl.value = URL.createObjectURL(blob);
    },
    { immediate: true },
);

function handleFrameError(error) {
    emit("error", error || new Error("PDF 预览加载失败"));
}

function revokeObjectUrl() {
    if (objectUrl.value) {
        URL.revokeObjectURL(objectUrl.value);
        objectUrl.value = "";
    }
}

onBeforeUnmount(() => {
    revokeObjectUrl();
});
</script>

<style scoped>
.pdf-preview {
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

.pdf-preview__toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    min-height: 40px;
    padding: 0 14px;
    border-bottom: 1px solid var(--border-toolbar-pdf);
    background: var(--bg-toolbar-pdf);
    color: var(--text-secondary);
    font-size: 13px;
}

.pdf-preview__stage {
    flex: 1;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
}

.pdf-preview__frame {
    display: block;
    width: 100%;
    height: 100%;
    border: 0;
    background: var(--bg-surface);
}

.pdf-preview__empty {
    display: flex;
    width: 100%;
    height: 100%;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    font-size: 14px;
}

@media (max-width: 768px) {
    .pdf-preview {
        min-height: 360px;
    }

    .pdf-preview__toolbar {
        align-items: flex-start;
        flex-direction: column;
        gap: 4px;
        padding: 8px 12px;
    }
}
</style>
