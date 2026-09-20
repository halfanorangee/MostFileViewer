<template>
    <div class="ebook-preview">
        <div class="ebook-preview__toolbar">
            <button
                type="button"
                class="ebook-preview__btn"
                :class="{ 'ebook-preview__btn--active': tocVisible }"
                title="目录"
                aria-label="目录"
                @click="toggleToc"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    class="ebook-preview__icon"
                    aria-hidden="true"
                >
                    <line x1="4" y1="6" x2="20" y2="6" />
                    <line x1="4" y1="12" x2="20" y2="12" />
                    <line x1="4" y1="18" x2="14" y2="18" />
                </svg>
            </button>

            <div class="ebook-preview__spacer"></div>

            <button
                type="button"
                class="ebook-preview__btn ebook-preview__btn--text"
                title="减小字号"
                aria-label="减小字号"
                @click="zoomOut"
            >
                A−
            </button>
            <span class="ebook-preview__font-label">{{ fontPercent }}</span>
            <button
                type="button"
                class="ebook-preview__btn ebook-preview__btn--text"
                title="增大字号"
                aria-label="增大字号"
                @click="zoomIn"
            >
                A+
            </button>

            <div class="ebook-preview__divider"></div>

            <button
                type="button"
                class="ebook-preview__btn"
                title="上一页"
                aria-label="上一页"
                @click="turn(-1)"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="ebook-preview__icon"
                    aria-hidden="true"
                >
                    <polyline points="15 18 9 12 15 6" />
                </svg>
            </button>
            <button
                type="button"
                class="ebook-preview__btn"
                title="下一页"
                aria-label="下一页"
                @click="turn(1)"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    class="ebook-preview__icon"
                    aria-hidden="true"
                >
                    <polyline points="9 18 15 12 9 6" />
                </svg>
            </button>
        </div>

        <div class="ebook-preview__body">
            <aside v-if="tocVisible" class="ebook-preview__toc">
                <p v-if="!tocItems.length" class="ebook-preview__toc-empty">
                    此书未提供目录
                </p>
                <button
                    v-for="(item, index) in tocItems"
                    :key="index"
                    type="button"
                    class="ebook-preview__toc-item"
                    :style="{ paddingLeft: `${12 + item.depth * 14}px` }"
                    :title="item.label"
                    @click="goToc(item)"
                >
                    <span class="ebook-preview__toc-text">{{ item.label }}</span>
                </button>
            </aside>

            <div class="ebook-preview__stage" ref="stageRef">
                <div v-if="opening" class="ebook-preview__loading">
                    <span class="ebook-preview__spinner"></span>
                </div>
            </div>
        </div>

        <div class="ebook-preview__footer">
            <div class="media-controls__track">
                <input
                    type="range"
                    class="media-controls__progress-bar"
                    :style="{ '--progress': trackPercent }"
                    min="0"
                    max="1"
                    step="0.001"
                    :value="displayProgress"
                    :disabled="opening"
                    aria-label="阅读进度"
                    title="阅读进度"
                    @input="onScrubInput"
                    @change="onScrubCommit"
                />
            </div>
            <span class="ebook-preview__percent">{{ percentText }}</span>
        </div>
    </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useTheme } from "../composables/useTheme.js";
// 副作用导入：注册 <foliate-view> 自定义元素。其内部在 open() 时按文件
// 魔数/扩展名动态加载 epub / mobi / fb2 / comic-book 解析模块与 vendor
// zip.js / fflate，因此只有真正打开电子书时才会加载这些解析代码。
import "foliate-js/view.js";
// 进度条复用音频/视频(MediaControls)的滑条样式,见 media-progress.css。
import "./media-progress.css";

const props = defineProps({
    // ArrayBuffer：由 usePreviewLoader 分块读取组装（与 Word/PDF 相同链路）。
    source: {
        type: Object,
        default: null,
    },
    name: {
        type: String,
        default: "",
    },
    extension: {
        type: String,
        default: "",
    },
});

const emit = defineEmits(["error"]);

// MIME 只影响 File 对象的类型标注；foliate-js 实际按「魔数 + 文件名后缀」
// 判别格式（EPUB/CBZ 检查 ZIP 魔数，MOBI/AZW3 检查 BOOKMOBI 魔数，
// FB2/CBZ/FBZ 兜底看扩展名），因此必须传入带原始扩展名的文件名。
const EBOOK_MIME = {
    ".epub": "application/epub+zip",
    ".fb2": "application/x-fictionbook+xml",
    ".fbz": "application/x-zip-compressed-fb2",
    ".cbz": "application/vnd.comicbook+zip",
    ".mobi": "application/x-mobipocket-ebook",
    ".prc": "application/x-mobipocket-ebook",
    ".azw": "application/x-mobipocket-ebook",
    ".azw3": "application/x-mobipocket-ebook",
};

const FONT_MIN = 0.7;
const FONT_MAX = 2;
const FONT_STEP = 0.1;

const stageRef = ref(null);
const opening = ref(true);
const progress = ref(0);
// 拖动进度条期间的预览值(null = 未在拖动),与 MediaControls 的 scrub 语义一致。
const scrubValue = ref(null);
// 已提交但尚未被 relocate 确认的目标位置。goToFraction 是异步渲染,
// 期间 relocate 未回来时进度条停留在点击处,避免「跳回原点再跳过去」的闪烁。
const pendingSeek = ref(null);
const tocVisible = ref(false);
const tocItems = ref([]);
const fontScale = ref(1);
const { currentTheme } = useTheme();

let view = null;
// 竞态防护：source 快速替换 / 组件卸载时，旧 open 流程的结果全部丢弃。
let openVersion = 0;

async function openBook() {
    if (!props.source) {
        return;
    }
    const version = ++openVersion;
    destroyView();
    opening.value = true;
    await nextTick();
    if (version !== openVersion || !stageRef.value) {
        return;
    }

    try {
        const extension = (props.extension || "").toLowerCase();
        const fileName = props.name || `book${extension || ".epub"}`;
        const file = new File([props.source], fileName, {
            type: EBOOK_MIME[extension] || "",
        });

        const nextView = document.createElement("foliate-view");
        nextView.addEventListener("relocate", onRelocate);
        nextView.addEventListener("load", onLoad);
        stageRef.value.append(nextView);
        view = nextView;

        await view.open(file);
        if (version !== openVersion) {
            return;
        }
        await view.init({});
        if (version !== openVersion) {
            return;
        }

        applyStyles();
        tocItems.value = flattenToc(view.book?.toc);
        opening.value = false;
    } catch (error) {
        if (version !== openVersion) {
            return;
        }
        destroyView();
        opening.value = false;
        emit("error", normalizeOpenError(error));
    }
}

function destroyView() {
    if (!view) {
        return;
    }
    const current = view;
    view = null;
    try {
        current.close();
    } catch (error) {
        // close 过程中渲染器可能已部分销毁，忽略即可。
    }
    current.remove();
    progress.value = 0;
    scrubValue.value = null;
    pendingSeek.value = null;
    tocItems.value = [];
}

// foliate 分页器会把注入的 CSS 持久化到 #styles，并对之后加载的每个
// 章节自动重新应用；此处只需在打开完成 / 主题或字号变化时调用一次。
function applyStyles() {
    view?.renderer?.setStyles?.(buildContentCSS());
}

function buildContentCSS() {
    const dark = currentTheme.value === "dark";
    const lines = [
        '@namespace epub "http://www.idpf.org/2007/ops";',
        "html {",
        `    color-scheme: ${dark ? "dark" : "light"};`,
        `    font-size: ${Math.round(fontScale.value * 100)}%;`,
        "}",
        "p, li, blockquote, dd {",
        "    line-height: 1.6;",
        "}",
        "pre {",
        "    white-space: pre-wrap !important;",
        "}",
        // 防图片拉伸变形:书籍常以 HTML 属性或 CSS 固定 img 高度,与
        // 分栏宽度限制叠加后盒子比例失配。强制高度按固有比例、内容
        // contain 显示,配合 foliate 已设置的 max-width/max-height 等比缩放。
        "img, video {",
        "    object-fit: contain !important;",
        "}",
        "img {",
        "    height: auto !important;",
        "}",
    ];
    if (dark) {
        // 暗色模式覆盖默认白底；书籍自带的高特异性样式（如封面页）仍按原样显示。
        lines.push(
            "html { background: #1e2126; }",
            "body { color: #d6d9de; }",
            "a:link { color: #7ab8f5; }",
        );
    }
    return lines.join("\n");
}

function onRelocate(event) {
    // 渲染完成:实际阅读位置已回来,清除跳转占位。
    pendingSeek.value = null;
    const fraction = event.detail?.fraction;
    if (Number.isFinite(fraction)) {
        progress.value = fraction;
    }
}

// 内容 iframe 与宿主文档不同 window，按键不会自然冒泡，需要逐章节挂接
// （与 foliate 官方 reader 相同的做法）。doc 随 iframe 销毁，无需手动解绑。
function onLoad(event) {
    const doc = event.detail?.doc;
    if (doc && !doc.__mfvEbookKeydown) {
        doc.__mfvEbookKeydown = true;
        // svg 不是 replaced element，object-fit 对其不生效；显式声明
        // preserveAspectRatio="none" 的 svg/image 会被拉伸，统一改回等比。
        // 默认值本就是 xMidYMid meet，此处仅修正被书写为 none 的情况，
        // 不影响正常书籍的任何表现。
        const distorted = doc.querySelectorAll(
            'svg[preserveAspectRatio="none"], svg image[preserveAspectRatio="none"]',
        );
        for (const el of distorted) {
            el.setAttribute("preserveAspectRatio", "xMidYMid meet");
        }
        doc.addEventListener("keydown", (event) => {
            if (event.key === "ArrowLeft") {
                event.preventDefault();
                turn(-1);
            } else if (event.key === "ArrowRight") {
                event.preventDefault();
                turn(1);
            }
        });
    }
}

function onGlobalKeydown(event) {
    // 焦点在输入控件时不劫持左右键（如进度滑块的键盘微调）。
    if (event.target instanceof Element) {
        const tag = event.target.tagName;
        if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
            return;
        }
    }
    // 非激活 tab 被 layui 以 display:none 隐藏，此时不响应。
    const stage = stageRef.value;
    if (!stage || stage.offsetParent === null) {
        return;
    }
    if (event.key === "ArrowLeft") {
        event.preventDefault();
        turn(-1);
    } else if (event.key === "ArrowRight") {
        event.preventDefault();
        turn(1);
    }
}

function turn(direction) {
    if (!view) {
        return;
    }
    if (direction < 0) {
        view.goLeft?.();
    } else {
        view.goRight?.();
    }
}

function onScrubInput(event) {
    const value = Number.parseFloat(event.target.value);
    if (Number.isFinite(value)) {
        scrubValue.value = value;
    }
}

function onScrubCommit(event) {
    const value = Number.parseFloat(event.target.value);
    scrubValue.value = null;
    if (!Number.isFinite(value)) {
        return;
    }
    // 占住显示直到 relocate 带回实际位置;跳转失败时回退显示当前进度。
    pendingSeek.value = value;
    Promise.resolve(view?.goToFraction?.(value)).catch((error) => {
        console.error(error);
        pendingSeek.value = null;
    });
}

function toggleToc() {
    tocVisible.value = !tocVisible.value;
}

function goToc(item) {
    if (item?.href && view) {
        view.goTo?.(item.href);
    }
}

function zoomIn() {
    fontScale.value = Math.min(
        FONT_MAX,
        Number((fontScale.value + FONT_STEP).toFixed(2)),
    );
}

function zoomOut() {
    fontScale.value = Math.max(
        FONT_MIN,
        Number((fontScale.value - FONT_STEP).toFixed(2)),
    );
}

const fontPercent = computed(() => `${Math.round(fontScale.value * 100)}%`);
// 进度条显示优先级:拖动预览 > 已提交待确认的跳转位置 > 实际阅读位置。
const displayProgress = computed(
    () => scrubValue.value ?? pendingSeek.value ?? progress.value,
);
const trackPercent = computed(() => `${(displayProgress.value || 0) * 100}%`);
const percentText = computed(
    () => `${Math.round((displayProgress.value || 0) * 100)}%`,
);

// EPUB/MOBI 的 toc label 可能是字符串、多语言对象或其数组，统一拍平为文本。
function formatTocLabel(label) {
    if (typeof label === "string") {
        return label;
    }
    if (Array.isArray(label)) {
        return label.length ? formatTocLabel(label[0]) : "";
    }
    if (label && typeof label === "object") {
        const values = Object.values(label);
        return values.length ? formatTocLabel(values[0]) : "";
    }
    return "";
}

function flattenToc(items, depth = 0, out = []) {
    if (!Array.isArray(items)) {
        return out;
    }
    for (const item of items) {
        if (!item) continue;
        const label = formatTocLabel(item.label);
        if (label) {
            out.push({ label, href: item.href, depth });
        }
        if (Array.isArray(item.subitems)) {
            flattenToc(item.subitems, depth + 1, out);
        }
    }
    return out;
}

function normalizeOpenError(error) {
    const message = String(error?.message || error || "").trim();
    if (/not supported/i.test(message)) {
        return "暂不支持预览该电子书格式（或文件已损坏）。";
    }
    if (/not found|empty/i.test(message)) {
        return "文件内容为空或无法读取。";
    }
    if (/decrypt|drm|password|encrypted/i.test(message)) {
        return "该电子书受 DRM 保护，暂不支持预览。";
    }
    return message || "电子书解析失败。";
}

watch(() => props.source, openBook, { immediate: true });
watch(currentTheme, applyStyles);
watch(fontScale, applyStyles);

onMounted(() => {
    window.addEventListener("keydown", onGlobalKeydown);
});

onBeforeUnmount(() => {
    window.removeEventListener("keydown", onGlobalKeydown);
    openVersion++;
    destroyView();
});
</script>

<style scoped>
.ebook-preview {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    height: 100%;
    background: var(--bg-preview);
}

.ebook-preview__toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 36px;
    padding: 0 8px;
    background: var(--bg-titlebar);
    border-bottom: 1px solid var(--border-default);
    flex-shrink: 0;
}

.ebook-preview__btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 26px;
    min-width: 26px;
    padding: 0 4px;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 12px;
    line-height: 1;
}

.ebook-preview__btn:hover {
    background: var(--bg-hover-accent);
    color: var(--text-primary);
}

.ebook-preview__btn--active {
    background: var(--bg-active);
    color: var(--text-active);
}

.ebook-preview__btn--text {
    font-weight: 600;
    padding: 0 6px;
}

.ebook-preview__icon {
    width: 15px;
    height: 15px;
}

.ebook-preview__spacer {
    flex: 1;
}

.ebook-preview__font-label {
    min-width: 36px;
    text-align: center;
    font-size: 11px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
}

.ebook-preview__divider {
    width: 1px;
    height: 16px;
    margin: 0 6px;
    background: var(--border-default);
}

.ebook-preview__body {
    flex: 1;
    display: flex;
    min-height: 0;
}

.ebook-preview__toc {
    width: 220px;
    flex-shrink: 0;
    overflow: auto;
    padding: 8px 6px;
    background: var(--bg-surface);
    border-right: 1px solid var(--border-default);
    display: flex;
    flex-direction: column;
    gap: 1px;
}

.ebook-preview__toc-empty {
    margin: 8px 6px;
    font-size: 12px;
    color: var(--text-muted);
}

.ebook-preview__toc-item {
    display: block;
    width: 100%;
    text-align: left;
    padding: 5px 8px;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: var(--text-secondary);
    font-size: 12px;
    cursor: pointer;
}

.ebook-preview__toc-item:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
}

.ebook-preview__toc-text {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.ebook-preview__stage {
    position: relative;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    display: flex;
}

/* foliate-view 是 shadow DOM 自定义元素（非 Vue 组件），需 :deep 控制。 */
.ebook-preview__stage :deep(foliate-view) {
    flex: 1;
    width: 100%;
    min-width: 0;
}

.ebook-preview__loading {
    position: absolute;
    inset: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-preview);
}

.ebook-preview__spinner {
    width: 22px;
    height: 22px;
    border: 2px solid var(--border-default);
    border-top-color: var(--text-muted);
    border-radius: 50%;
    animation: ebook-spin 0.8s linear infinite;
}

@keyframes ebook-spin {
    to {
        transform: rotate(360deg);
    }
}

.ebook-preview__footer {
    display: flex;
    align-items: center;
    gap: 14px;
    flex-shrink: 0;
    padding: 5px 16px;
    background: var(--bg-titlebar);
    border-top: 1px solid var(--border-default);
}

.ebook-preview__footer .media-controls__track {
    min-height: 14px;
}

.ebook-preview__percent {
    min-width: 42px;
    text-align: right;
    font-size: 11px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
}
</style>

