// 网页 / Markdown 预览面板的拖拽调宽逻辑。
// 通过 pointer 事件监听拖拽，按百分比限定在 [MIN, MAX] 区间；
// 拖拽结束后回调 onResizeEnd，供调用方重建滚动同步锚点等。
import { computed, ref } from "vue";

const MIN_WEB_PREVIEW_WIDTH_PERCENT = 20;
const MAX_WEB_PREVIEW_WIDTH_PERCENT = 80;

function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
}

/**
 * 创建预览面板调宽控制器。
 * @param {() => HTMLElement | null} getPreviewBody 获取用于测量的容器（预览体）
 * @param {{ onResizeEnd?: () => void }} [options]
 * @returns {{
 *   widthPercent: import("vue").Ref<number>,
 *   resizing: import("vue").Ref<boolean>,
 *   panelStyle: import("vue").ComputedRef<{ flexBasis: string }>,
 *   handleResizeStart: (event: PointerEvent) => void,
 *   stop: () => void,
 * }}
 */
export function createWebPreviewResizer(getPreviewBody, options = {}) {
    const { onResizeEnd } = options;
    const widthPercent = ref(45);
    const resizing = ref(false);
    const panelStyle = computed(() => ({
        flexBasis: `${widthPercent.value}%`,
    }));

    function handleResizeMove(event) {
        const body = getPreviewBody();
        if (!body) {
            return;
        }

        const rect = body.getBoundingClientRect();
        if (rect.width <= 0) {
            return;
        }

        const previewWidth = rect.right - event.clientX;
        const nextPercent = (previewWidth / rect.width) * 100;
        widthPercent.value = clamp(
            nextPercent,
            MIN_WEB_PREVIEW_WIDTH_PERCENT,
            MAX_WEB_PREVIEW_WIDTH_PERCENT,
        );
    }

    function stop() {
        if (!resizing.value) {
            return;
        }

        resizing.value = false;
        window.removeEventListener("pointermove", handleResizeMove);
        window.removeEventListener("pointerup", stop);
        window.removeEventListener("pointercancel", stop);
        // 面板宽度变化会影响编辑器折行高度与预览元素偏移，需重建锚点。
        onResizeEnd?.();
    }

    function handleResizeStart(event) {
        if (!getPreviewBody()) {
            return;
        }

        event.preventDefault();
        resizing.value = true;
        window.addEventListener("pointermove", handleResizeMove);
        window.addEventListener("pointerup", stop, { once: true });
        window.addEventListener("pointercancel", stop, { once: true });
        handleResizeMove(event);
    }

    return { widthPercent, resizing, panelStyle, handleResizeStart, stop };
}
