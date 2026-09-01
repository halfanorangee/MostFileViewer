// 弹出菜单定位：按视口与裁剪容器边界自动翻转、夹紧，空间不足时改为菜单内部滚动。
// 解决按钮菜单在窗口边缘被裁剪、显示不全的问题（如播放器字幕菜单、标题栏文件菜单）。
//
// 用法：菜单打开后（v-if 挂载完成，通常在 nextTick 中）调用 positionMenu：
//   const { menuStyle, positionMenu, stopAutoUpdate } = useMenuPosition({ placement: "top" });
//   <div v-if="open" ref="menuEl" class="menu" :style="menuStyle">…</div>
//   nextTick(() => positionMenu(anchorEl, menuEl));
// 关闭菜单时调用 stopAutoUpdate() 停止位置跟踪。
import { onBeforeUnmount, reactive } from "vue";

const VIEWPORT_MARGIN = 8;
const DEFAULT_GAP = 6;
const MIN_MENU_HEIGHT = 48;

function isClipValue(value) {
    return value === "hidden" || value === "auto" || value === "scroll" || value === "clip";
}

// 向上查找最近的裁剪容器（overflow 非 visible 的祖先）。菜单定位不允许超出它，
// 避免菜单延伸到 overflow: hidden 的面板之外仍被裁剪掉。
function findClipContainer(el) {
    let node = el ? el.parentElement : null;
    while (node && node !== document.body) {
        const style = window.getComputedStyle(node);
        if (isClipValue(style.overflowY) || isClipValue(style.overflowX)) {
            return node;
        }
        node = node.parentElement;
    }
    return null;
}

/**
 * 创建菜单定位状态。
 *
 * @param {Object} [options]
 * @param {"top"|"bottom"} [options.placement="bottom"] 首选弹出方向，放不下时自动翻转
 * @param {"start"|"center"|"end"} [options.align="start"] 水平对齐方式
 * @param {number} [options.gap=6] 菜单与锚点（或鼠标落点）的间距
 * @param {number} [options.viewportMargin=8] 菜单与边界的最小间距
 * @returns {{
 *   menuStyle: Object,
 *   positionMenu: (anchorEl: HTMLElement|null, menuEl: HTMLElement, point?: {x:number,y:number}|null) => void,
 *   stopAutoUpdate: () => void,
 * }}
 */
export function useMenuPosition(options = {}) {
    const config = {
        placement: "bottom",
        align: "start",
        gap: DEFAULT_GAP,
        viewportMargin: VIEWPORT_MARGIN,
        maxHeight: 0,
        ...options,
    };

    // 菜单首帧隐藏，测量完成后再显示，避免闪现在默认位置
    const menuStyle = reactive({
        left: "0px",
        top: "0px",
        maxWidth: "",
        maxHeight: "",
        overflowY: "visible",
        visibility: "hidden",
    });

    let cleanupAutoUpdate = null;
    let retryFrame = 0;

    function computePlacement(anchorRect, point, menuRect, bounds) {
        const menuWidth = menuRect.width;
        const menuHeight = menuRect.height;
        const gap = config.gap;
        const configuredMaxHeight =
            Number.isFinite(config.maxHeight) && config.maxHeight > 0
                ? config.maxHeight
                : 0;
        const constrainedMenuHeight = configuredMaxHeight
            ? Math.min(menuHeight, configuredMaxHeight)
            : menuHeight;

        const refTop = anchorRect ? anchorRect.top : point.y;
        const refBottom = anchorRect ? anchorRect.bottom : point.y;
        const refLeft = anchorRect ? anchorRect.left : point.x;
        const refRight = anchorRect ? anchorRect.right : point.x;
        const refCenterX = anchorRect ? (anchorRect.left + anchorRect.right) / 2 : point.x;

        const spaceTop = refTop - bounds.top;
        const spaceBottom = bounds.bottom - refBottom;

        // 垂直方向：首选方向放不下则翻转；两侧都放不下时选空间更大的一侧
        const fits = (side) =>
            (side === "top" ? spaceTop : spaceBottom) >=
            constrainedMenuHeight + gap;
        let placement = config.placement;
        if (!fits(placement)) {
            const opposite = placement === "top" ? "bottom" : "top";
            placement = fits(opposite) ? opposite : spaceTop >= spaceBottom ? "top" : "bottom";
        }


        // 选中方向的空间仍不够时，限制菜单高度并交给菜单内部滚动
        let maxHeight = configuredMaxHeight;
        let renderedHeight = constrainedMenuHeight;
        if (!fits(placement)) {
            const chosenSpace = placement === "top" ? spaceTop : spaceBottom;
            const availableHeight = Math.max(
                MIN_MENU_HEIGHT,
                Math.floor(chosenSpace - gap),
            );
            maxHeight = configuredMaxHeight
                ? Math.min(configuredMaxHeight, availableHeight)
                : availableHeight;
            renderedHeight = Math.min(constrainedMenuHeight, maxHeight);
        }

        let top = placement === "top" ? refTop - gap - renderedHeight : refBottom + gap;
        top = Math.min(
            Math.max(top, bounds.top),
            Math.max(bounds.top, bounds.bottom - renderedHeight),
        );

        // 水平方向：按 align 对齐后夹紧到边界内
        let left;
        if (config.align === "end") {
            left = refRight - menuWidth;
        } else if (config.align === "center") {
            left = refCenterX - menuWidth / 2;
        } else {
            left = refLeft;
        }
        left = Math.min(Math.max(left, bounds.left), Math.max(bounds.left, bounds.right - menuWidth));

        return {
            left: Math.round(left),
            top: Math.round(top),
            maxHeight: maxHeight > 0 ? Math.round(maxHeight) : 0,
        };
    }

    function apply(anchorEl, menuEl, point) {
        const menuRect = menuEl.getBoundingClientRect();
        if (menuRect.width <= 0 && menuRect.height <= 0) {
            // 菜单尚未完成布局，等下一帧重试一次
            if (!retryFrame) {
                retryFrame = window.requestAnimationFrame(() => {
                    retryFrame = 0;
                    if (menuEl.isConnected) {
                        apply(anchorEl, menuEl, point);
                    }
                });
            }
            return false;
        }

        const viewportWidth = window.innerWidth;
        const viewportHeight = window.innerHeight;
        const margin = config.viewportMargin;

        // 可用边界 = 最近裁剪容器与视口的交集（留出 margin）
        const clipEl = findClipContainer(menuEl);
        const clipRect = clipEl ? clipEl.getBoundingClientRect() : null;
        const bounds = {
            top: Math.max(clipRect ? clipRect.top : 0, margin),
            bottom: Math.min(clipRect ? clipRect.bottom : viewportHeight, viewportHeight - margin),
            left: Math.max(clipRect ? clipRect.left : 0, margin),
            right: Math.min(clipRect ? clipRect.right : viewportWidth, viewportWidth - margin),
        };

        // 宽度上限取类样式中的 max-width 与可用宽度的较小值：先清空 inline 值，
        // 使 getComputedStyle 反映类样式（如字幕菜单的 max-width: 320px）
        menuStyle.maxWidth = "";
        const styleMaxWidth = parseFloat(window.getComputedStyle(menuEl).maxWidth);
        const maxWidth = Number.isFinite(styleMaxWidth) && styleMaxWidth > 0
            ? Math.min(styleMaxWidth, Math.floor(bounds.right - bounds.left))
            : Math.floor(bounds.right - bounds.left);

        // 内容自然高度取 scrollHeight：max-height 限制生效时 rect.height 是受限值，
        // 若用它判断是否放得下，会造成"限制→展开→再限制"的抖动循环
        const menuSize = {
            width: menuRect.width,
            height: Math.max(menuRect.height, menuEl.scrollHeight),
        };

        const anchorRect = anchorEl ? anchorEl.getBoundingClientRect() : null;
        const placement = computePlacement(anchorRect, point, menuSize, bounds);

        // 菜单 left/top 相对其 CSS 定位包含块（offsetParent）换算；
        // fixed 菜单的 offsetParent 为 null，此时按视口坐标输出。
        let offsetX = 0;
        let offsetY = 0;
        if (menuEl.offsetParent) {
            const offsetRect = menuEl.offsetParent.getBoundingClientRect();
            offsetX = offsetRect.left + menuEl.offsetParent.clientLeft;
            offsetY = offsetRect.top + menuEl.offsetParent.clientTop;
        }

        menuStyle.left = `${placement.left - offsetX}px`;
        menuStyle.top = `${placement.top - offsetY}px`;
        const hasStyleMaxWidth = Number.isFinite(styleMaxWidth) && styleMaxWidth > 0;
        // 类样式的 max-width 已足够窄时保持 inline 为空，避免覆盖组件原有的宽度约束
        menuStyle.maxWidth =
            hasStyleMaxWidth && maxWidth >= styleMaxWidth ? "" : `${maxWidth}px`;
        menuStyle.maxHeight = placement.maxHeight ? `${placement.maxHeight}px` : "";
        menuStyle.overflowY = placement.maxHeight ? "auto" : "visible";
        menuStyle.visibility = "visible";
        return true;
    }

    function startAutoUpdate(anchorEl, menuEl, point) {
        const update = () => {
            if (menuEl.isConnected) {
                apply(anchorEl, menuEl, point);
            } else {
                stopAutoUpdate();
            }
        };
        window.addEventListener("resize", update);
        document.addEventListener("scroll", update, true);
        const disposers = [
            () => window.removeEventListener("resize", update),
            () => document.removeEventListener("scroll", update, true),
        ];

        if (typeof ResizeObserver !== "undefined") {
            // 菜单内容尺寸变化（如字幕轨道异步加载）时重新定位
            const observer = new ResizeObserver(() => {
                if (menuEl.isConnected) {
                    apply(anchorEl, menuEl, point);
                }
            });
            observer.observe(menuEl);
            disposers.push(() => observer.disconnect());
        }

        cleanupAutoUpdate = () => {
            disposers.forEach((dispose) => dispose());
            cleanupAutoUpdate = null;
        };
    }

    /**
     * 定位菜单并开启自动跟随（窗口尺寸变化、页面滚动、菜单内容变化时重新计算）。
     *
     * @param {HTMLElement|null} anchorEl 触发菜单的锚点元素；右键菜单等无锚点场景传 null
     * @param {HTMLElement} menuEl 已挂载的菜单元素
     * @param {{x: number, y: number}|null} [point] 视口坐标落点，与 anchorEl 二选一
     */
    function positionMenu(anchorEl, menuEl, point = null) {
        if (!menuEl || (!anchorEl && !point)) {
            return;
        }
        if (retryFrame) {
            window.cancelAnimationFrame(retryFrame);
            retryFrame = 0;
        }
        stopAutoUpdate();
        if (apply(anchorEl, menuEl, point)) {
            startAutoUpdate(anchorEl, menuEl, point);
        }
    }

    function stopAutoUpdate() {
        if (retryFrame) {
            window.cancelAnimationFrame(retryFrame);
            retryFrame = 0;
        }
        if (cleanupAutoUpdate) {
            cleanupAutoUpdate();
        }
    }

    onBeforeUnmount(stopAutoUpdate);

    return {
        menuStyle,
        positionMenu,
        stopAutoUpdate,
    };
}
