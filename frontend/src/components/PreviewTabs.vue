<template>
    <section
        ref="rootRef"
        class="preview-tabs"
        :class="{
            'preview-tabs--tabbar-visible': tabBarScrollVisible,
            'preview-tabs--drop-target': isDropTargetPane,
        }"
        @wheel.capture="onTabBarWheelCapture"
        @pointerup="onPanePointerUp"
        @dragstart="onDragStart"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="handleDrop"
        @dragend="onDragEnd"
    >
        <div
            class="preview-tabs__container"
            :class="{
                'preview-tabs__container--empty': !tabs.length,
            }"
            @dblclick="handleContainerDblClick"
        >
            <!-- lay-tab 始终渲染：即使没有 tab也保留 tab 签条的 ul 结构
                 （.layui-tab-title > ul），保证首页等场景下样式与工作区一致 -->
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

                            <EBookPreview
                                v-else-if="tab.previewType === 'ebook'"
                                class="ebook-preview"
                                :source="tab.source"
                                :name="tab.name"
                                :extension="tab.extension"
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

                            <template v-else-if="tab.previewType === 'audio'">
                                <AudioPreview
                                    v-if="tab.path === activeTabPath"
                                    class="audio-preview"
                                    :path="tab.path"
                                    :src="tab.source"
                                    :extension="tab.extension"
                                    :media="tab.media"
                                    :queue-mode="audioQueueMode"
                                    :has-prev="audioHasPrev"
                                    :has-next="audioHasNext"
                                    @media-error="
                                        (payload) => emit('media-error', tab.path, payload)
                                    "
                                    @media-reload="emit('media-reload', tab.path)"
                                    @media-open-system="
                                        emit('media-open-system', tab.path)
                                    "
                                    @request-prev="emit('request-prev', tab.path)"
                                    @request-next="(manual) => emit('request-next', tab.path, manual)"
                                    @set-queue-mode="emit('set-queue-mode', $event)"
                                />
                            </template>

                            <template v-else-if="tab.previewType === 'video'">
                                <VideoPreview
                                    v-if="tab.path === activeTabPath"
                                    class="video-preview"
                                    :src="tab.source"
                                    :path="tab.path"
                                    :extension="tab.extension"
                                    :media="tab.media"
                                    @media-error="
                                        (payload) => emit('media-error', tab.path, payload)
                                    "
                                    @media-reload="emit('media-reload', tab.path)"
                                    @media-open-system="
                                        emit('media-open-system', tab.path)
                                    "
                                />
                            </template>

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
            <!-- 「新增空白 tab」专属区域：固定占位于 tab 签条最右侧，
                 详见 style.css 中 .preview-tabs__new-btn-area 的注释 -->
            <!-- 内部拖拽投放层：拖拽期间覆盖整个 pane（含 tab 条）。
                 iframe / PDF / Office 画布会截获 dragover，无法依赖原生目标元素，
                 因此用覆盖层统一接管落点判定；无内部拖拽时本层不渲染，
                 外部文件拖入行为完全不变。 -->
            <div
                v-if="isDragging"
                class="preview-tabs__drop-layer"
                @dragover.prevent="onDropLayerDragOver"
                @drop="handleDrop"
                @dragleave="onDropLayerDragLeave"
            >
                <div
                    v-if="zonePreview"
                    class="preview-tabs__drop-preview"
                    :class="`preview-tabs__drop-preview--${zonePreview}`"
                ></div>
            </div>
            <div class="preview-tabs__new-btn-area">
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
        </div>
        <!-- 拖拽跟随浮层：原生 drag image 在 WebView 宿主下拿不到，
             改由 pointer 状态机渲染自己的半透明 tab 副本作为拖动反馈。
             位置只由命令式写入 transform 跟随指针：这里刻意不绑定响应式坐标，
             否则拖拽期间落点变化引发的重渲染会把 transform 回写为拖拽起点的
             旧值，ghost 跳回原位再被 pointermove 拉回，表现为「抽搐一下」。 -->
        <div
            v-if="dragGhost"
            ref="dragGhostRef"
            class="preview-tabs__drag-ghost"
            aria-hidden="true"
        >
            <span class="preview-tabs__drag-ghost-text">{{ dragGhost.name }}</span>
        </div>
    </section>
</template>

<script setup>
import {
    computed,
    defineAsyncComponent,
    h,
    nextTick,
    onBeforeUnmount,
    onMounted,
    ref,
    watch,
} from "vue";
import { resolvePreviewDropZone } from "../composables/usePreviewLayout";
import {
    TAB_DRAG_TYPE,
    dragIndicatorGeometry,
    registerDropResolver,
    resolveDropAtPoint,
    setDragIndicatorGeometry,
    unregisterDropResolver,
    useTabDragState,
} from "../composables/useTabDragState";

const ExcelPreview = defineAsyncComponent(() => import("./ExcelPreview.vue"));
const WordPreview = defineAsyncComponent(() => import("./WordPreview.vue"));
const PptPreview = defineAsyncComponent(() => import("./PptPreview.vue"));
const PdfPreview = defineAsyncComponent(() => import("./PdfPreview.vue"));
const EBookPreview = defineAsyncComponent(() => import("./EBookPreview.vue"));
const ImagePreview = defineAsyncComponent(() => import("./ImagePreview.vue"));
const AudioPreview = defineAsyncComponent(() => import("./AudioPreview.vue"));
const VideoPreview = defineAsyncComponent(() => import("./VideoPreview.vue"));
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
    audioQueueMode: {
        type: String,
        default: "off",
    },
    audioHasPrev: {
        type: Boolean,
        default: false,
    },
    audioHasNext: {
        type: Boolean,
        default: false,
    },
    paneId: {
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
    "save-as-tab",
    "open-in-new-tab",
    "reorder-tab",
    "move-tab",
    "split-tab",
    "new-tab",
    "auto-save-toggle",
    "media-error",
    "media-reload",
    "media-open-system",
    "request-prev",
    "request-next",
    "set-queue-mode",
    "focus-pane",
]);

const codePreviewRefs = ref({});

const rootRef = ref(null);

// tab 溢出后改用原生滚动条，但 layui 仍在 ul 上监听 wheel 并 preventDefault +
// translate 平移标题（useTabHeader.handleUlScroll），会与原生滚动冲突。
// 在捕获阶段拦截 tab 标题栏内的 wheel 事件，阻止到达 layui 的监听，
// 同时不阻止默认行为，让浏览器执行原生横向滚动。
function onTabBarWheelCapture(event) {
    if (event.target?.closest?.(".layui-tab-title")) {
        event.stopPropagation();
    }
}

// ---- tab 栏滚动条显隐（JS 维护，见 style.css 中 .preview-tabs--tabbar-visible）----
// 不能只靠 CSS :hover：按住滚动条拖动时指针被滚动条捕获，页面收不到
// mousemove/mouseleave，「拖出 tab 栏再松开」后 :hover 状态会滞留，滚动条不消失。
// 改为在 window 上同步指针位置：
// - 拖动滚动条期间鼠标事件被捕获、不派发给页面，状态保持不变（滚动条持续可见）；
// - 松开时按 mouseup 的实际位置立即纠正；若松开事件未派发，指针下一次移动
//   时的 mousemove 也会纠正。
const tabBarScrollVisible = ref(false);

function isInsideTabBar(target) {
    return target instanceof Element && !!target.closest(".layui-tab-title");
}

function syncTabBarScrollVisible(event) {
    // 分屏后存在多个 PreviewTabs 实例，只有事件发生在自己内部时才改变状态。
    const root = rootRef.value;
    if (!root || !(event.target instanceof Node) || !root.contains(event.target)) {
        if (tabBarScrollVisible.value) {
            tabBarScrollVisible.value = false;
        }
        return;
    }
    tabBarScrollVisible.value = isInsideTabBar(event.target);
}

onMounted(() => {
    window.addEventListener("mouseup", syncTabBarScrollVisible);
    window.addEventListener("mousemove", syncTabBarScrollVisible);
    // 注册落点解析器：pointer 拖拽时按指针坐标定位目标 pane。
    registerDropResolver(props.paneId, {
        getRect: () => rootRef.value?.getBoundingClientRect() || null,
        resolve: (clientX, clientY) => resolveDropAt(clientX, clientY),
    });
});

onBeforeUnmount(() => {
    window.removeEventListener("mouseup", syncTabBarScrollVisible);
    window.removeEventListener("mousemove", syncTabBarScrollVisible);
    unregisterDropResolver(props.paneId);
});

// 激活 tab 落在可视区外（如恢复会话、从侧栏打开新文件）时，
// 将其滚动到 tab 栏可视范围内。layui 原生的平移滚动已被禁用，需要自己处理。
watch(
    () => [props.activeTabPath, props.tabs.length],
    () => {
        nextTick(() => {
            const root = rootRef.value;
            if (!root) return;
            const ul = root.querySelector(".layui-tab-title");
            const active = ul?.querySelector("li.layui-this");
            if (!ul || !active) return;
            const left = active.offsetLeft;
            const right = left + active.offsetWidth;
            if (left < ul.scrollLeft) {
                ul.scrollLeft = left;
            } else if (right > ul.scrollLeft + ul.clientWidth) {
                ul.scrollLeft = right - ul.clientWidth;
            }
        });
    },
    { flush: "post" },
);

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
                // 用 SVG 代替文本「×」：文本字形受字体基线度量影响，
                // 在按钮内视觉上不居中；SVG 是替换元素，flex 可精确居中。
                h("svg", {
                    class: "preview-tabs__close-icon",
                    viewBox: "0 0 10 10",
                    width: 10,
                    height: 10,
                    "aria-hidden": "true",
                }, [
                    h("line", {
                        x1: 1.5,
                        y1: 1.5,
                        x2: 8.5,
                        y2: 8.5,
                        stroke: "currentColor",
                        "stroke-width": 1.4,
                        "stroke-linecap": "round",
                    }),
                    h("line", {
                        x1: 8.5,
                        y1: 1.5,
                        x2: 1.5,
                        y2: 8.5,
                        stroke: "currentColor",
                        "stroke-width": 1.4,
                        "stroke-linecap": "round",
                    }),
                ]),
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
                "data-tab-path": tab.path,
                // 拖拽完全由 pointer 事件实现。这里刻意不设 draggable：
                // 一旦元素是原生拖拽源，浏览器会在拖动开始时派发 pointercancel
                // 接管指针，pointermove / pointerup 随之停发，自实现的拖拽会被腰斩。
                onPointerdown: (event) => onTabPointerDown(event, tab.path),
                // 中键按下时阻止浏览器默认的自动滚动态，避免干扰关闭操作
                onMousedown: (event) => {
                    if (event.button === 1) {
                        event.preventDefault();
                    }
                },
                // 鼠标中键（button === 1）点击关闭 tab（auxclick 才会响应中键）
                onAuxclick: (event) => {
                    if (event.button !== 1) return;
                    event.preventDefault();
                    event.stopPropagation();
                    emit("close-tab", tab.path);
                },
            },
            titleChildren,
        );
    };
}

// ---- Tab 拖动排序 ----

// 拖拽状态来自 useTabDragState（模块级单例）：分屏后拖拽会跨越多个
// PreviewTabs 实例，源状态必须共享，目标实例才能识别正在拖动的 tab。
const {
    dragSource,
    dropTarget,
    dragSession,
    isDragging,
    beginTabDrag,
    endTabDrag,
    setTabDropTarget,
    clearTabDropTarget,
} = useTabDragState();

// 正在拖动且来源为本 pane 的 tab path。
const dragSourcePath = computed(() =>
    dragSource.value && dragSource.value.paneId === props.paneId
        ? dragSource.value.path
        : "",
);

// 本 pane tab 条上的落点（用于插入指示线）。
const tabDropTarget = computed(() =>
    dropTarget.value?.kind === "tab" && dropTarget.value.paneId === props.paneId
        ? dropTarget.value
        : null,
);

const dragOverPath = computed(() => tabDropTarget.value?.path || "");
const dragInsertAfter = computed(() => !!tabDropTarget.value?.after);

// 拖拽期间本 pane 是否为落点（用于落点高亮）。
// 同 pane 的 tab 条落点（tab / pane-end）不算数：同 pane 重排有插入指示线提示，
// 再给整个 pane 加一圈蓝色 inset 描边只会让 tab 区域凭空多出蓝框；
// 只有跨 pane 移动 / 分屏（zone 落点、跨 pane 的 tab / pane-end）才描边。
const isDropTargetPane = computed(() => {
    if (!isDragging.value) return false;
    const target = dropTarget.value;
    if (!target || target.paneId !== props.paneId) return false;
    if (target.kind === "tab" || target.kind === "pane-end") {
        return target.paneId !== dragSource.value?.paneId;
    }
    return true;
});

// 内容区四分区高亮：center 表示「移入该 pane」，不显示高亮块。
const zonePreview = computed(() => {
    const target = dropTarget.value;
    if (!target || target.paneId !== props.paneId || target.kind !== "zone") {
        return "";
    }
    return target.zone === "center" ? "" : target.zone;
});

// tab 条与 tab 签几何缓存：拖拽期间布局不变，每个拖拽会话只采样一次。
let tabBarRect = null;
let tabRects = null;
let cachedDragSession = -1;

function cacheTabGeometry() {
    const root = rootRef.value;
    if (!root) return;
    const titleEl = root.querySelector(".layui-tab-title");
    const items = root.querySelectorAll(".layui-tab-title > li");
    tabBarRect = titleEl ? titleEl.getBoundingClientRect() : null;
    tabRects = Array.from(items).map((li) => ({
        path: li.querySelector("[data-tab-path]")?.getAttribute("data-tab-path") || "",
        rect: li.getBoundingClientRect(),
    }));
}

/** 同一拖拽会话内复用几何缓存；跨 pane 拖拽时目标 pane 也会惰性采样一次。 */
function ensureTabGeometry() {
    if (!dragSource.value || cachedDragSession === dragSession.value) {
        return;
    }
    cachedDragSession = dragSession.value;
    cacheTabGeometry();
}

function clearTabGeometry() {
    tabBarRect = null;
    tabRects = null;
    cachedDragSession = -1;
}

/** 在 tab 条内命中目标 tab 签，返回 { path, after }；未命中返回 null。 */
function hitTestTab(clientX, clientY) {
    if (!tabBarRect || !tabRects) {
        return null;
    }
    if (clientY < tabBarRect.top || clientY > tabBarRect.bottom) {
        return null;
    }
    for (const item of tabRects) {
        if (!item.path) continue;
        if (clientX < item.rect.left) {
            return { path: item.path, after: false };
        }
        if (clientX <= item.rect.right) {
            return {
                path: item.path,
                after: clientX > item.rect.left + item.rect.width / 2,
            };
        }
    }
    const last = [...tabRects].reverse().find((item) => item.path);
    return last ? { path: last.path, after: true } : null;
}

/**
 * 计算指针位置的落点：
 * - tab 条内命中 tab 签 → 插入到其前 / 后；
 * - tab 条空白 → 追加到该 pane 的 tab 列表末尾；
 * - 内容区四边 1/4 → 分屏；中央 → 移入该 pane。
 */
function resolveDropAt(clientX, clientY) {
    const source = dragSource.value;
    if (!source) return null;

    ensureTabGeometry();

    if (tabBarRect && clientY <= tabBarRect.bottom) {
        const hit = hitTestTab(clientX, clientY);
        if (!hit) {
            return { kind: "pane-end", paneId: props.paneId };
        }
        if (hit.path === source.path) {
            return null;
        }
        return {
            kind: "tab",
            paneId: props.paneId,
            path: hit.path,
            after: hit.after,
        };
    }

    const root = rootRef.value;
    if (!root) return null;
    const rect = root.getBoundingClientRect();
    const contentTop = tabBarRect ? tabBarRect.height : 0;
    const contentRect = {
        left: rect.left,
        top: rect.top + contentTop,
        width: rect.width,
        height: Math.max(rect.height - contentTop, 0),
    };
    return {
        kind: "zone",
        paneId: props.paneId,
        zone: resolvePreviewDropZone(clientX, clientY, contentRect),
    };
}

/**
 * 应用落点：同 pane 重排、跨 pane 移动、分屏三种语义分别上抛给 PreviewArea。
 */
function applyDrop(source, target) {
    if (!source || !target) return;
    if (target.kind === "zone") {
        if (target.zone === "center") {
            if (target.paneId !== source.paneId) {
                emit("move-tab", {
                    fromPath: source.path,
                    toPaneId: target.paneId,
                    beforePath: "",
                    after: true,
                });
            }
            return;
        }
        emit("split-tab", {
            fromPath: source.path,
            targetPaneId: target.paneId,
            zone: target.zone,
        });
        return;
    }

    if (target.kind === "tab") {
        if (target.paneId === source.paneId) {
            emit("reorder-tab", {
                paneId: source.paneId,
                fromPath: source.path,
                toPath: target.path,
                after: target.after,
            });
        } else {
            emit("move-tab", {
                fromPath: source.path,
                toPaneId: target.paneId,
                beforePath: target.path,
                after: target.after,
            });
        }
        return;
    }

    if (target.kind === "pane-end") {
        if (target.paneId === source.paneId) {
            const lastTab = props.tabs[props.tabs.length - 1];
            if (!lastTab || lastTab.path === source.path) return;
            emit("reorder-tab", {
                paneId: source.paneId,
                fromPath: source.path,
                toPath: lastTab.path,
                after: true,
            });
            return;
        }
        emit("move-tab", {
            fromPath: source.path,
            toPaneId: target.paneId,
            beforePath: "",
            after: true,
        });
    }
}

// 拖拽刚结束时抑制一次焦点写入：HTML5 拖拽结束后若仍派发 pointerup，
// 其事件源是拖拽起点，会让焦点回跳到源 pane、覆盖落点逻辑设置的焦点。
let lastDragEndAt = 0;

// 抬起指针时把焦点切到该 pane（决定后续新打开文件的归属）。
// 刻意不用 pointerdown：按下瞬间写入全局焦点会在 mousedown 与 dragstart 之间
// 触发一次全树重渲染，正好让 tab 的拖动无法启动。
function onPanePointerUp(event) {
    if (!props.paneId) return;
    if (event && event.button !== 0) return;
    if (isDragging.value || Date.now() - lastDragEndAt < 300) return;
    emit("focus-pane", props.paneId);
}

// 拖拽指示线几何是模块级状态（useTabDragState 的 dragIndicatorGeometry）：
// 跨 pane 拖拽时由目标 pane 的实例写入，保证指示线画在正确位置。

function resolveTabPath(target) {
    const el = target?.closest?.("[data-tab-path]");
    if (el) {
        return el.getAttribute("data-tab-path") || "";
    }
    // 兜底：data-tab-path 未取到时按 tab 条顺序反查（tab 条顺序与 props.tabs 一致），
    // 避免任何 DOM 属性异常都导致「拖动完全无反应」。
    const li = target?.closest?.(".layui-tab-title > li");
    const items = li?.parentElement ? Array.from(li.parentElement.children) : [];
    const index = li ? items.indexOf(li) : -1;
    return index >= 0 && index < props.tabs.length ? props.tabs[index].path : "";
}

function onDragStart(event) {
    // draggable=false 不能保证阻止可拖动祖先启动拖拽，需在事件入口显式过滤关闭按钮。
    if (event.target?.closest?.(".preview-tabs__close")) {
        event.preventDefault();
        return;
    }

    const path = resolveTabPath(event.target);
    if (!path) return;

    startTabDrag(event, path);
}

/**
 * 开始一次内部 tab 拖拽。
 * 既由 tab 标题节点直接绑定调用（携带 path，最可靠），也由容器委托调用（兜底）。
 */
function startTabDrag(event, path) {
    if (!path) return;

    beginTabDrag(props.paneId, path);
    ensureTabGeometry();

    // 使用自定义 MIME 标记，避免与外部文件/文本拖入混淆；
    // 部分浏览器需要 setData 才能触发后续 drop。
    if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData(TAB_DRAG_TYPE, path);
    }
}

/**
 * 根容器 dragover（兜底路径）：拖动刚开始时投放层尚未渲染（需要一次重渲染），
 * 首次 dragover 会落在真实元素上，由这里接管；投放层出现后由它接管。
 */
function onDragOver(event) {
    // 仅在内部 tab 拖动场景下阻止默认行为，避免影响 Wails 文件拖入等其它拖放交互。
    if (!dragSource.value) return;

    event.preventDefault();
    setTabDropTarget(resolveDropAt(event.clientX, event.clientY));
    updateIndicatorFromTarget();
    if (event.dataTransfer) {
        event.dataTransfer.dropEffect = "move";
    }
}

/** 投放层（覆盖整个 pane，含 tab 条）的 dragover：统一落点判定。 */
function onDropLayerDragOver(event) {
    // 投放层只在内部拖拽时渲染，这里的 dragSource 必然存在；仍做一次防御。
    if (!dragSource.value) return;

    event.preventDefault();
    event.stopPropagation();
    setTabDropTarget(resolveDropAt(event.clientX, event.clientY));
    updateIndicatorFromTarget();
    if (event.dataTransfer) {
        event.dataTransfer.dropEffect = "move";
    }
}

function onDropLayerDragLeave(event) {
    if (event.currentTarget.contains(event.relatedTarget)) {
        return;
    }
    clearTabDropTarget(props.paneId);
    clearTabGeometry();
}

/** 依据当前落点刷新 tab 插入指示线的几何位置。 */
function updateIndicatorFromTarget() {
    const target = tabDropTarget.value;
    if (!target || !tabRects) return;
    const hit = tabRects.find((item) => item.path === target.path);
    if (hit) {
        // 指示线几何是模块级状态：拖到别的 pane 的 tab 条上时那边也要显示。
        setDragIndicatorGeometry(hit.rect);
    }
}

// 落点变化时，由「落点所属 pane」的实例刷新指示线几何，
// 这样跨 pane 拖动时指示线画在目标 pane 上、位置才正确。
watch(
    () => [
        dropTarget.value?.kind,
        dropTarget.value?.paneId,
        dropTarget.value?.path,
        dropTarget.value?.after,
    ],
    () => {
        if (!dragSource.value) return;
        if (dropTarget.value?.paneId !== props.paneId) return;
        ensureTabGeometry();
        updateIndicatorFromTarget();
    },
);

// ---- tab 拖拽（pointer 事件实现） ----
// HTML5 draggable 在部分 WebView 宿主里根本不会启动（既不派发 dragstart、
// 也没有拖拽快照），而点击链路一定可用，因此以 pointer 事件为主路径：
// 按下 → 位移超过阈值 → 进入拖拽；配合指针捕获，
// 指针移到 iframe / 预览画布上方时依然能收到 pointermove。
const TAB_DRAG_THRESHOLD = 4;
let tabPointerDrag = null;

// 拖拽跟随浮层（替代原生 drag image）。只在拖拽源所在 pane 渲染一份；
// 位置完全由命令式 transform 更新（见 setDragGhostPosition 的注释），
// pointermove 与落点变化引发的重渲染期间都不触发组件重渲染。
const dragGhost = ref(null);
const dragGhostRef = ref(null);

function dragGhostLabel(path) {
    const tab = props.tabs.find((item) => item.path === path);
    if (tab?.name) return tab.name;
    return path.split(/[\\/]/).pop() || "tab";
}

function setDragGhostPosition(clientX, clientY) {
    const el = dragGhostRef.value;
    // ghost 尚未挂载（v-if 刚触发）时跳过，由进入拖拽处的 nextTick 回调补定位。
    if (!el) return;
    el.style.transform = `translate3d(${clientX}px, ${clientY}px, 0) translate(-50%, -50%)`;
}

function clearTabPointerDrag() {
    const drag = tabPointerDrag;
    tabPointerDrag = null;
    // 跟随浮层与拖拽同生共死（它同时也承担「拖拽中」的视觉反馈）。
    dragGhost.value = null;
    window.removeEventListener("pointermove", onTabPointerMove);
    window.removeEventListener("pointerup", onTabPointerUp);
    window.removeEventListener("pointercancel", onTabPointerCancel);
    if (typeof document !== "undefined") {
        document.body.classList.remove("mfv-tab-dragging");
    }
    // 仅在真正捕获过时释放：纯点击从未捕获，不能对未捕获的指针调用，
    // 否则 Chrome 会直接抛错。
    if (drag?.captured && drag.pointerId != null) {
        try {
            rootRef.value?.releasePointerCapture?.(drag.pointerId);
        } catch {
            // 指针已释放时忽略。
        }
    }
}

function onTabPointerDown(event, path) {
    if (event.button !== 0 || !path) return;
    // 关闭按钮只负责关闭，不参与拖拽。
    if (event.target?.closest?.(".preview-tabs__close")) return;
    // 这里刻意不调用 preventDefault：它会让兼容的 mouse 事件（进而 click）失效，
    // tab 就无法点击切换了。tab 标题上没有 draggable，所以不会有原生拖拽被触发；
    // 拖拽时的文本选择由 CSS 的 user-select 与 body 上的 mfv-tab-dragging 类禁用。
    //
    // 同理，这里也刻意不设置指针捕获：捕获会把后续 pointerup（进而 click）
    // 的目标重定向到捕获元素（pane 根节点），layui 的 tab 点击监听收不到
    // click，点击切换 tab 就会失效。捕获推迟到真正进入拖拽时再设置。
    clearTabPointerDrag();
    tabPointerDrag = {
        pointerId: event.pointerId,
        path,
        startX: event.clientX,
        startY: event.clientY,
        active: false,
        captured: false,
    };
    window.addEventListener("pointermove", onTabPointerMove);
    window.addEventListener("pointerup", onTabPointerUp);
    window.addEventListener("pointercancel", onTabPointerCancel);
}

function onTabPointerMove(event) {
    const drag = tabPointerDrag;
    if (!drag || event.pointerId !== drag.pointerId) return;

    if (!drag.active) {
        const moved = Math.hypot(
            event.clientX - drag.startX,
            event.clientY - drag.startY,
        );
        if (moved < TAB_DRAG_THRESHOLD) return;
        // 按键已松开（pointerup 丢失）时不要进入拖拽。
        if (event.buttons === 0) {
            clearTabPointerDrag();
            return;
        }
        drag.active = true;
        // 进入拖拽后才捕获指针（纯点击不受捕获影响，click 正常派发）。
        // 捕获绑定在稳定的根元素上：tab 标题 span 可能在拖拽期间被重新渲染，
        // 绑在它身上会丢掉捕获，指针移到 iframe / 画布上方时就收不到事件了。
        try {
            rootRef.value?.setPointerCapture?.(event.pointerId);
            drag.captured = true;
        } catch {
            // 捕获失败时退化为 window 监听：只要指针不在 iframe 上仍能工作。
        }
        // 拖拽期间禁止文本选择 / 显示抓手光标。
        document.body.classList.add("mfv-tab-dragging");
        beginTabDrag(props.paneId, drag.path);
        ensureTabGeometry();
        // 跟随指针的半透明 tab 副本：原生 drag image 在 WebView 下不可靠，
        // 这里由 pointer 状态机自己渲染。只放 name，不放坐标——坐标一旦进
        // 响应式，拖拽期间任何重渲染（落点变化、指示线刷新）都会把 :style
        // 里的旧 transform 重新写回 DOM，ghost 跳回拖拽起点造成抽搐。
        dragGhost.value = { name: dragGhostLabel(drag.path) };
        // ghost 尚未挂载，挂载后用当前指针坐标补一次初始定位。
        const ghostInitX = event.clientX;
        const ghostInitY = event.clientY;
        nextTick(() => setDragGhostPosition(ghostInitX, ghostInitY));
    }

    setDragGhostPosition(event.clientX, event.clientY);
    setTabDropTarget(resolveDropAtPoint(event.clientX, event.clientY));
    updateIndicatorFromTarget();
}

function onTabPointerUp(event) {
    const drag = tabPointerDrag;
    if (!drag || event.pointerId !== drag.pointerId) return;

    const source = dragSource.value;
    const active = drag.active;
    clearTabPointerDrag();
    if (!active || !source) return;

    // 指针捕获会把 pointerup 派发给捕获元素，这里用坐标兜底重算一次落点，
    // 避免落点被中途清空导致「拖到了却没反应」。
    const target =
        dropTarget.value || resolveDropAtPoint(event.clientX, event.clientY);
    applyDrop(source, target);
    lastDragEndAt = Date.now();
    endTabDrag();
    clearTabGeometry();
}

function onTabPointerCancel() {
    clearTabPointerDrag();
    lastDragEndAt = Date.now();
    endTabDrag();
    clearTabGeometry();
}

function onDragLeave(event) {
    // 仅当离开整个 pane 才清空，避免子元素间冒泡造成的抖动。
    if (!event.currentTarget.contains(event.relatedTarget)) {
        clearTabDropTarget(props.paneId);
    }
}

function handleDrop(event) {
    const source = dragSource.value;
    // 外部文件或其他组件的拖放交给原有逻辑处理。
    if (!source) return;
    event.preventDefault();
    event.stopPropagation();

    // 落点可能已被 dragleave / 重渲染清空，这里用松手坐标兜底重算一次，
    // 避免「拖到了却没有反应」。
    const target = dropTarget.value || resolveDropAt(event.clientX, event.clientY);
    applyDrop(source, target);
    endTabDrag();
    clearTabGeometry();
}

function onDragEnd() {
    lastDragEndAt = Date.now();
    endTabDrag();
    clearTabGeometry();
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
// - tab 签条最右侧的预留留白（`.layui-tab-title` 内除 li 之外的区域，
//   含最右侧「新增空白 tab」专属区域——该区域 pointer-events: none，
//   双击事件会穿透到 `.layui-tab-title` 上）
// - 无 tab 时的容器剩余空白区域
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
    // 无 tab 时的容器剩余空白区域
    if (target === event.currentTarget && props.tabs.length === 0) {
        emit("new-tab");
    }
}
</script>
