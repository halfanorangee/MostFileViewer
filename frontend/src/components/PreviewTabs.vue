<template>
    <section
        class="preview-tabs"
        @dragstart="onDragStart"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
        @dragend="onDragEnd"
    >
        <div
            class="preview-tabs__container"
            :class="{
                'preview-tabs__container--empty': !tabs.length,
            }"
            @dblclick="handleContainerDblClick"
        >
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
                            class="preview-tabs__content preview-tabs__content--loading"
                        ></div>

                        <div
                            v-else-if="tab.status === 'error'"
                            class="preview-tabs__state preview-tabs__state--error"
                        >
                            <p>{{ tab.error || "预览失败" }}</p>
                        </div>

                        <div v-else class="preview-tabs__content">
                            <PreviewPane
                                v-if="tab.previewType === 'preview'"
                                class="preview-pane-tab"
                                :content="tab.content"
                                :extension="tab.extension"
                                :name="tab.name"
                            />

                            <WordPreview
                                v-else-if="tab.previewType === 'word'"
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
                                :content-version="tab.contentVersion"
                                :extension="tab.extension"
                                :name="tab.name"
                                :encoding="tab.encoding"
                                :encoding-loading="tab.encodingLoading"
                                :is-virtual="!!tab.virtual"
                                :is-dirty="!!tab.dirty"
                                :is-saving="!!tab.saving"
                                :auto-save-enabled="autoSaveEnabled"
                                @dirty="handleContentChange(tab.path)"
                                @encoding-change="
                                    (encoding) =>
                                        emit('encoding-change', tab.path, encoding)
                                "
                                @save="emit('save-tab', tab.path)"
                                @save-as="emit('save-as-tab', tab.path)"
                                @open-in-new-tab="
                                    (payload) =>
                                        emit('open-in-new-tab', tab.path, payload)
                                "
                                @auto-save-toggle="
                                    (enabled) =>
                                        emit('auto-save-toggle', enabled)
                                "
                            />
                        </div>
                    </lay-tab-item>
                </lay-tab>
            </template>
            <div v-else class="preview-tabs__empty-hint">
                <p>当前没有打开的标签</p>
                <p class="preview-tabs__empty-hint-sub">
                    点击右上角 <span class="preview-tabs__empty-hint-icon">+</span>
                    新建一个可编辑的空白标签
                </p>
            </div>
            <button
                type="button"
                class="preview-tabs__new-btn"
                title="新建空白标签"
                aria-label="新建空白标签"
                @click="emit('new-tab')"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="preview-tabs__new-btn-icon"
                    aria-hidden="true"
                    focusable="false"
                >
                    <line x1="12" y1="5" x2="12" y2="19" />
                    <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
            </button>
        </div>
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
const PreviewPane = defineAsyncComponent(() => import("./PreviewPane.vue"));

const props = defineProps({
    tabs: {
        type: Array,
        default: () => [],
    },
    activeTabPath: {
        type: String,
        default: "",
    },
    autoSaveEnabled: {
        type: Boolean,
        default: true,
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
    "new-tab",
    "auto-save-toggle",
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
    // 返回 render 函数：在每次重渲染时读取 tab 的响应式字段以及拖拽状态，
    // 确保状态图标、脏标记、拖拽指示线都能随之刷新。
    return () => {
        // 确定状态图标
        let statusIcon = "";
        let statusTitle = "";
        let statusClass = "";

        if (tab.status === "loading") {
            statusIcon = "";
            statusTitle = "正在加载...";
            statusClass =
                "preview-tabs__status preview-tabs__status--loading";
        } else if (tab.saving) {
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
        if (statusClass) {
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
                    draggable: "false",
                    onClick: (event) => {
                        event.stopPropagation();
                        emit("close-tab", tab.path);
                    },
                },
                "×",
            ),
        );

        const dragClass = {
            "preview-tabs__title--drag-over-left":
                dragOverPath.value === tab.path && !dragInsertAfter.value,
            "preview-tabs__title--drag-over-right":
                dragOverPath.value === tab.path && dragInsertAfter.value,
            "preview-tabs__title--dragging":
                dragSourcePath.value === tab.path,
        };
        const dragStyle =
            dragOverPath.value === tab.path
                ? {
                      "--drag-indicator-left":
                          dragIndicatorGeometry.value.left,
                      "--drag-indicator-right":
                          dragIndicatorGeometry.value.right,
                      "--drag-indicator-width":
                          dragIndicatorGeometry.value.width,
                  }
                : undefined;
        return h(
            "span",
            {
                class: ["preview-tabs__title", dragClass],
                style: dragStyle,
                draggable: "true",
                "data-tab-path": tab.path,
            },
            titleChildren,
        );
    };
}

// ---- Tab 拖动排序 ----

const TAB_DRAG_TYPE = "application/x-most-file-viewer-tab";

const dragSourcePath = ref(""); // 正在拖动的 tab path
const dragOverPath = ref(""); // 当前悬停的目标 tab path
const dragInsertAfter = ref(false); // 落点在目标左侧(false)还是右侧(true)
const dragIndicatorGeometry = ref({
    left: "0px",
    right: "0px",
    width: "2px",
});

function resolveTabPath(target) {
    const el = target?.closest?.("[data-tab-path]");
    return el ? el.getAttribute("data-tab-path") : "";
}

function onDragStart(event) {
    // draggable=false 不能保证阻止可拖动祖先启动拖拽，需在事件入口显式过滤关闭按钮。
    if (event.target?.closest?.(".preview-tabs__close")) {
        event.preventDefault();
        return;
    }

    const path = resolveTabPath(event.target);
    if (!path) return;

    dragSourcePath.value = path;

    // 使用自定义 MIME 标记，避免与外部文件/文本拖入混淆；
    // 部分浏览器需要 setData 才能触发后续 drop。
    if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData(TAB_DRAG_TYPE, path);
    }
}

function onDragOver(event) {
    // 仅在内部 tab 拖动场景下阻止默认行为，避免影响 Wails 文件拖入等其它拖放交互。
    if (!dragSourcePath.value) return;

    event.preventDefault();

    const path = resolveTabPath(event.target);
    if (!path || path === dragSourcePath.value) {
        dragOverPath.value = "";
        return;
    }

    const el = event.target.closest("[data-tab-path]");
    const tabItem = el?.closest(".layui-tab-title > li");
    const rect = tabItem?.getBoundingClientRect() ?? el.getBoundingClientRect();
    dragInsertAfter.value = event.clientX > rect.left + rect.width / 2;
    updateDragIndicatorGeometry(rect);
    dragOverPath.value = path;
    if (event.dataTransfer) {
        event.dataTransfer.dropEffect = "move";
    }
}

function updateDragIndicatorGeometry(rect) {
    const devicePixelRatio =
        typeof window === "undefined" ? 1 : window.devicePixelRatio || 1;
    const snap = (value) => Math.round(value * devicePixelRatio) / devicePixelRatio;

    // Use two physical pixels for the marker and snap both outer edges. This
    // prevents different fractional tab boundaries from producing different
    // antialiasing widths in the WebView.
    dragIndicatorGeometry.value = {
        left: `${snap(rect.left) - rect.left}px`,
        right: `${rect.right - snap(rect.right)}px`,
        width: `${2 / devicePixelRatio}px`,
    };
}

function onDragLeave(event) {
    // 仅当离开整个 tab 头区域才清空，避免子元素间冒泡造成的抖动。
    if (!event.currentTarget.contains(event.relatedTarget)) {
        dragOverPath.value = "";
    }
}

function onDrop(event) {
    // 外部文件或其他组件的拖放交给原有逻辑处理。
    if (!dragSourcePath.value) return;
    event.preventDefault();

    const from = dragSourcePath.value;
    const to = dragOverPath.value;
    if (from && to && from !== to) {
        emit("reorder-tab", {
            fromPath: from,
            toPath: to,
            after: dragInsertAfter.value,
        });
    }
    resetDragState();
}

function onDragEnd() {
    resetDragState();
}

function resetDragState() {
    dragSourcePath.value = "";
    dragOverPath.value = "";
    dragInsertAfter.value = false;
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

// 在 tab 签区域的空白处双击即可新增空白 tab：
// - tab 签条最右侧的 padding 留白（`.layui-tab-title` 内除 li 之外的区域）
// - 无 tab 时的整片空白提示区
// 双击已有的 tab 签本身、关闭按钮、+ 按钮或 tab 内容区均不触发，避免误操作。
function handleContainerDblClick(event) {
    const target = event.target;
    if (!(target instanceof Element)) {
        return;
    }
    // 「+」按钮自带单击逻辑，避免重复触发
    if (target.closest(".preview-tabs__new-btn")) {
        return;
    }
    // 双击已有 tab 签不触发（保留给 layui 自带行为）
    if (target.closest(".layui-tab-title li")) {
        return;
    }
    // tab 签条最右侧 padding 留白
    if (target.closest(".layui-tab-title")) {
        emit("new-tab");
        return;
    }
    // 无 tab 时的提示区（pointer-events: none，事件实际落在容器上）
    if (
        target.closest(".preview-tabs__empty-hint") ||
        (target === event.currentTarget && props.tabs.length === 0)
    ) {
        emit("new-tab");
    }
}
</script>
