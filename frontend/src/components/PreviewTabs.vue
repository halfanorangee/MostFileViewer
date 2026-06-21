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
                            v-else-if="
                                ['excel', 'csv'].includes(tab.previewType)
                            "
                            class="excel-preview"
                            :src="tab.source"
                            :extension="tab.extension"
                            :encoding="tab.encoding"
                            @error="(err) => handleRenderError(tab.path, err)"
                        />

                        <PptPreview
                            v-else-if="tab.previewType === 'ppt'"
                            class="ppt-preview"
                            :src="tab.source"
                            @error="(err) => handleRenderError(tab.path, err)"
                            @rendered="emit('preview-rendered', tab.path)"
                        />

                        <PdfPreview
                            v-else-if="tab.previewType === 'pdf'"
                            class="pdf-preview"
                            :src="tab.source"
                            :name="tab.name"
                            @error="(err) => handleRenderError(tab.path, err)"
                        />

                        <ImagePreview
                            v-else-if="tab.previewType === 'image'"
                            class="image-preview"
                            :src="tab.source"
                            :extension="tab.extension"
                            :name="tab.name"
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
                            :name="tab.name"
                            :encoding="tab.encoding"
                            @dirty="handleContentChange(tab.path)"
                            @encoding-change="
                                (encoding) =>
                                    emit('encoding-change', tab.path, encoding)
                            "
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
const PptPreview = defineAsyncComponent(() => import("./PptPreview.vue"));
const PdfPreview = defineAsyncComponent(() => import("./PdfPreview.vue"));
const ImagePreview = defineAsyncComponent(() => import("./ImagePreview.vue"));
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
    "preview-rendered",
    "content-change",
    "encoding-change",
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
    const current = codePreviewRefs.value[path] ?? null;
    const next = component ?? null;
    if (current === next) return;
    const newRefs = { ...codePreviewRefs.value };
    if (next) {
        newRefs[path] = next;
    } else {
        delete newRefs[path];
    }
    codePreviewRefs.value = newRefs;
}

function getCodeContent(path) {
    return codePreviewRefs.value[path]?.getContent?.();
}

defineExpose({
    getCodeContent,
});

function renderTabTitle(tab) {
    // 确定状态图标
    let statusIcon = "";
    let statusTitle = "";
    let statusClass = "";

    if (tab.saving) {
        statusIcon = "↻"; // 旋转箭头，表示保存中
        statusTitle = "保存中...";
        statusClass = "preview-tabs__status preview-tabs__status--saving";
    } else if (tab.saveError) {
        statusIcon = "!";
        statusTitle = tab.saveError;
        statusClass = "preview-tabs__status preview-tabs__status--error";
    } else if (tab.dirty && tab.previewType === "code") {
        statusIcon = "●"; // 圆点，表示已修改
        statusTitle = "有未保存的修改";
        statusClass = "preview-tabs__status preview-tabs__status--dirty";
    }

    const titleChildren = [];

    // 添加状态图标（如果有）
    if (statusIcon) {
        titleChildren.push(
            h(
                "span",
                {
                    class: statusClass,
                    title: statusTitle,
                    "aria-label": statusTitle,
                },
                statusIcon,
            ),
        );
    }

    // 添加文件名
    titleChildren.push(
        h(
            "span",
            {
                class: "preview-tabs__title-text",
                title: tab.name,
            },
            tab.name,
        ),
    );

    // 添加关闭按钮
    titleChildren.push(
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
    );

    return () => h("span", { class: "preview-tabs__title" }, titleChildren);
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
