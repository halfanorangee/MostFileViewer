import { computed, ref } from "vue";

/**
 * Tab 拖动状态（模块级单例）。
 *
 * 分屏后拖拽会跨越多个 PreviewTabs 实例（每个 pane 一个），源状态必须共享，
 * 否则目标实例读不到「正在拖动的 tab」，dragover 会被直接忽略、drop 不触发。
 */

/** 自定义 MIME：与外部文件 / 文本拖入区分。 */
export const TAB_DRAG_TYPE = "application/x-most-file-viewer-tab";

/** 正在拖动的 tab：{ paneId, path }；null 表示当前没有内部拖拽。 */
const dragSource = ref(null);
/** 当前落点：{ kind, paneId, path?, after?, zone? }；null 表示无有效落点。 */
const dropTarget = ref(null);
/** 拖拽会话序号：每次开始拖拽自增，供各 pane 判断几何缓存是否需要重新采样。 */
const dragSession = ref(0);

const isDragging = computed(() => !!dragSource.value);

function sameTarget(left, right) {
    if (!left || !right) {
        return left === right;
    }
    return (
        left.kind === right.kind &&
        left.paneId === right.paneId &&
        left.path === right.path &&
        left.after === right.after &&
        left.zone === right.zone
    );
}

export function beginTabDrag(paneId, path) {
    if (!path) return;
    dragSession.value += 1;
    dragSource.value = { paneId: paneId || "", path };
    dropTarget.value = null;
}

export function endTabDrag() {
    dragSource.value = null;
    dropTarget.value = null;
}

/**
 * tab 条上的插入指示线几何（模块级：拖拽可能跨 pane，目标 pane 也要显示）。
 * 值对齐物理像素，避免不同小数边界的 tab 在 WebView 里出现抗锯齿宽度差。
 */
export const dragIndicatorGeometry = ref({
    left: "0px",
    right: "0px",
    width: "2px",
});

/** 依据目标 tab 的矩形更新指示线几何。 */
export function setDragIndicatorGeometry(rect) {
    if (!rect) return;
    const dpr = typeof window === "undefined" ? 1 : window.devicePixelRatio || 1;
    const snap = (value) => Math.round(value * dpr) / dpr;
    dragIndicatorGeometry.value = {
        left: `${snap(rect.left) - rect.left}px`,
        right: `${rect.right - snap(rect.right)}px`,
        width: `${2 / dpr}px`,
    };
}

/**
 * 各 pane 的落点解析器注册表。
 *
 * pointer 事件拖拽时指针可能停在任意 pane（甚至 iframe / 画布）上方，
 * 无法依赖事件目标，因此由「指针坐标 + 各 pane 矩形」定位目标 pane，
 * 再调用该 pane 自己的命中逻辑；pane 之间互不重叠，取首个命中即可。
 */
const dropResolvers = new Map();

export function registerDropResolver(paneId, resolver) {
    dropResolvers.set(paneId || "", resolver);
}

export function unregisterDropResolver(paneId) {
    dropResolvers.delete(paneId || "");
}

/** 按指针坐标解析全局落点；指针不在任何 pane 内时返回 null。 */
export function resolveDropAtPoint(clientX, clientY) {
    for (const resolver of dropResolvers.values()) {
        const rect = resolver.getRect ? resolver.getRect() : null;
        if (
            !rect ||
            clientX < rect.left ||
            clientX > rect.right ||
            clientY < rect.top ||
            clientY > rect.bottom
        ) {
            continue;
        }
        return resolver.resolve(clientX, clientY);
    }
    return null;
}

/** 写入落点。相同落点不重复赋值，避免 dragover 高频触发引起无谓重渲染。 */
export function setTabDropTarget(target) {
    if (sameTarget(dropTarget.value, target || null)) {
        return;
    }
    dropTarget.value = target || null;
}

/** 离开某个 pane 时清除它自己的落点，不影响其它 pane 的落点。 */
export function clearTabDropTarget(paneId) {
    const current = dropTarget.value;
    if (!current) return;
    if (paneId && current.paneId !== paneId) return;
    dropTarget.value = null;
}

// 兜底清理：拖拽结束的事件（dragend / drop）总是会在 window 上冒泡，
// 即使拖拽源元素在拖拽过程中被重新渲染替换、组件内的 @dragend 没能触发，
// 这里也能保证拖拽状态被复位。缺少兜底时 dragSource 残留会让投放层常驻，
// 整个预览区将无法再点击 / 拖动 tab。
if (typeof window !== "undefined") {
    const finalize = () => {
        if (dragSource.value) {
            endTabDrag();
        }
    };
    window.addEventListener("dragend", finalize);
    // drop 用冒泡阶段：确保组件的投放处理（内部已 stopPropagation）先执行。
    window.addEventListener("drop", finalize);
    window.addEventListener("blur", finalize);
}

export function useTabDragState() {
    return {
        dragSource,
        dropTarget,
        dragSession,
        isDragging,
        beginTabDrag,
        endTabDrag,
        setTabDropTarget,
        clearTabDropTarget,
    };
}
