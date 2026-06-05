<template>
    <section class="preview-tabs">
        <template v-if="tabs.length">
            <lay-tab
                class="preview-tabs__lay"
                :model-value="activeTabPath"
                @update:model-value="$emit('change-tab', $event)"
            >
                <lay-tab-item
                    v-for="tab in tabs"
                    :key="tab.path"
                    :id="tab.path"
                    :title="renderTabTitle(tab)"
                >
                    <div
                        v-if="tab.status === 'loading'"
                        class="preview-tabs__state"
                    >
                        <lay-loading />
                        <p>正在加载 {{ tab.name }}</p>
                    </div>

                    <div
                        v-else-if="tab.status === 'error'"
                        class="preview-tabs__state preview-tabs__state--error"
                    >
                        <p>{{ tab.error || "预览失败" }}</p>
                    </div>

                    <div v-else class="preview-tabs__content">
                        <WordPreview
                            v-if="tab.previewType === 'word'"
                            class="office-preview"
                            :src="tab.source"
                            @error="(err) => handleRenderError(tab.path, err)"
                        />

                        <ExcelPreview
                            v-else-if="tab.previewType === 'excel'"
                            class="excel-preview"
                            :src="tab.source"
                            :extension="tab.extension"
                            :encoding="tab.encoding"
                            @error="(err) => handleRenderError(tab.path, err)"
                        />

                        <div
                            v-else-if="tab.previewType === 'unsupported'"
                            class="preview-tabs__state preview-tabs__state--error"
                        >
                            <p>{{ getUnsupportedMessage(tab) }}</p>
                        </div>

                        <CodePreview
                            v-else
                            :ref="
                                (component) =>
                                    setCodePreviewRef(tab.path, component)
                            "
                            class="code-preview"
                            :content="tab.content"
                            :extension="tab.extension"
                            @dirty="handleContentChange(tab.path)"
                            @save="emit('save-tab', tab.path)"
                        />
                    </div>
                </lay-tab-item>
            </lay-tab>
        </template>
    </section>
</template>

<script setup>
import { defineAsyncComponent, h, ref, watch } from "vue";

const ExcelPreview = defineAsyncComponent(() => import("./ExcelPreview.vue"));
const WordPreview = defineAsyncComponent(() => import("./WordPreview.vue"));
const CodePreview = defineAsyncComponent(() => import("./CodePreview.vue"));

const props = defineProps({
    tabs: {
        type: Array,
        default: () => [],
    },
    activeTabPath: {
        type: String,
        default: "",
    },
});

const emit = defineEmits([
    "change-tab",
    "close-tab",
    "preview-error",
    "content-change",
    "save-tab",
]);

const codePreviewRefs = ref({});

watch(
    () => props.tabs.map((tab) => tab.path),
    (paths) => {
        const activePaths = new Set(paths);
        codePreviewRefs.value = Object.fromEntries(
            Object.entries(codePreviewRefs.value).filter(([path]) =>
                activePaths.has(path),
            ),
        );
    },
);

function setCodePreviewRef(path, component) {
    if (!component) {
        const nextRefs = { ...codePreviewRefs.value };
        delete nextRefs[path];
        codePreviewRefs.value = nextRefs;
        return;
    }

    codePreviewRefs.value = {
        ...codePreviewRefs.value,
        [path]: component,
    };
}

function getCodeContent(path) {
    return codePreviewRefs.value[path]?.getContent?.() ?? "";
}

defineExpose({
    getCodeContent,
});

function renderTabTitle(tab) {
    return () =>
        h("span", { class: "preview-tabs__title" }, [
            h(
                "span",
                {
                    class: "preview-tabs__title-text",
                    title: tab.name,
                },
                tab.name,
            ),
            h(
                "button",
                {
                    type: "button",
                    class: "preview-tabs__close",
                    title: "关闭",
                    "aria-label": `关闭 ${tab.name}`,
                    onClick: (event) => {
                        event.stopPropagation();
                        emit("close-tab", tab.path);
                    },
                },
                "×",
            ),
        ]);
}

function getUnsupportedMessage(tab) {
    if (String(tab.extension || "").toLowerCase() === ".xls") {
        return "暂不支持预览旧版 .xls（二进制 Excel）文件，请另存为 .xlsx 后再打开。";
    }
    return "暂不支持预览该文件类型。";
}

function handleContentChange(path) {
    emit("content-change", path);
}

function handleRenderError(path, error) {
    emit("preview-error", path, error);
}
</script>
