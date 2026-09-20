import { computed, ref, watch } from "vue";

/**
 * 预览区分屏布局模型。
 *
 * 数据结构（二叉分割树）：
 *   叶子 pane：{ type: "pane", id, tabPaths: string[], activePath: string }
 *   分支 split：{ type: "split", id, direction: "row" | "column", ratio: number, children: [node, node] }
 *
 * 约定：
 * - openTabs 仍是全局扁平数组（path 唯一），保存 / 编码 / 文件系统同步 / 媒体 token
 *   等既有逻辑不受影响；本模块只维护「哪些 path 属于哪个 pane、顺序如何」。
 * - pane.tabPaths 的顺序即该 pane 的 tab 条顺序。
 * - activeTabPath（App 层）语义为「焦点 pane 的激活 tab」，由本模块统一回写，
 *   因此 App 里所有既有使用点（标题栏保存、Ctrl+S、侧栏高亮、音频队列）无需改动。
 * - 所有变更都返回新树（最多 4 个叶子，克隆成本可忽略），便于 Vue 响应式追踪。
 */

/** 分屏命中：目标 pane 外圈比例（上下左右各 1/4 为该方向的分屏带）。 */
export const PREVIEW_SPLIT_EDGE = 0.25;
/** 同时存在的 pane 上限。 */
export const PREVIEW_MAX_PANES = 4;
/** 分割比例下限 / 上限，保证两侧都留有可见空间。 */
export const PREVIEW_MIN_RATIO = 0.15;
export const PREVIEW_MAX_RATIO = 0.85;
/** 合法的落点方向。 */
export const PREVIEW_DROP_ZONES = ["top", "bottom", "left", "right", "center"];

let paneSeq = 0;
let splitSeq = 0;

function nextPaneId() {
    paneSeq += 1;
    return `pane-${paneSeq}`;
}

function nextSplitId() {
    splitSeq += 1;
    return `split-${splitSeq}`;
}

/** 保证后续新建的 pane / split id 不会与从会话恢复的 id 冲突。 */
function reserveIdsFrom(node) {
    if (!node || typeof node !== "object") return;
    const match = /^(pane|split)-(\d+)$/.exec(String(node.id || ""));
    if (match) {
        const value = Number(match[2]);
        if (Number.isFinite(value)) {
            if (match[1] === "pane") {
                paneSeq = Math.max(paneSeq, value);
            } else {
                splitSeq = Math.max(splitSeq, value);
            }
        }
    }
    for (const child of node.children || []) {
        reserveIdsFrom(child);
    }
}

/** 新建一个空 pane。 */
export function createPreviewPane() {
    return { type: "pane", id: nextPaneId(), tabPaths: [], activePath: "" };
}

/** 深度优先收集全部 pane（顺序与视觉顺序一致：左上 → 右下）。 */
export function collectPanes(node, result = []) {
    if (!node) return result;
    if (node.type === "pane") {
        result.push(node);
        return result;
    }
    collectPanes(node.children?.[0], result);
    collectPanes(node.children?.[1], result);
    return result;
}

/** 按 id 查找 pane，找不到返回 null。 */
export function findPreviewPane(node, paneId) {
    if (!node) return null;
    if (node.type === "pane") {
        return node.id === paneId ? node : null;
    }
    return (
        findPreviewPane(node.children?.[0], paneId) ||
        findPreviewPane(node.children?.[1], paneId)
    );
}

/**
 * 计算拖拽落点分区：上下左右四个 1/4 边缘带触发分屏，中间 50% × 50% 为 center。
 * @param {number} clientX
 * @param {number} clientY
 * @param {{left:number,top:number,width:number,height:number}} rect 目标 pane 的矩形
 * @returns {"top"|"bottom"|"left"|"right"|"center"}
 */
export function resolvePreviewDropZone(clientX, clientY, rect) {
    if (!rect || !rect.width || !rect.height) {
        return "center";
    }
    const x = (clientX - rect.left) / rect.width;
    const y = (clientY - rect.top) / rect.height;
    const edge = PREVIEW_SPLIT_EDGE;

    if (y < edge) return "top";
    if (y > 1 - edge) return "bottom";
    if (x < edge) return "left";
    if (x > 1 - edge) return "right";
    return "center";
}

export function clampPreviewRatio(ratio) {
    const value = Number(ratio);
    if (!Number.isFinite(value)) {
        return 0.5;
    }
    return Math.min(Math.max(value, PREVIEW_MIN_RATIO), PREVIEW_MAX_RATIO);
}

/**
 * @param {Object} options
 * @param {import("vue").Ref<Array<{path:string}>>} options.openTabs 全局扁平 tab 数组
 * @param {import("vue").Ref<string>} options.activeTabPath 焦点 pane 的激活 tab（双向同步）
 * @param {number} [options.maxPanes] pane 上限
 */
export function usePreviewLayout({
    openTabs,
    activeTabPath,
    maxPanes = PREVIEW_MAX_PANES,
}) {
    const layout = ref(createPreviewPane());
    const focusedPaneId = ref(layout.value.id);

    const panes = computed(() => collectPanes(layout.value));
    const paneCount = computed(() => panes.value.length);
    const focusedPane = computed(
        () => findPreviewPane(layout.value, focusedPaneId.value) || panes.value[0] || null,
    );
    const canSplit = computed(() => paneCount.value < maxPanes);

    // 会话恢复期间挂起 reconcile：避免「openTabs 尚为空」的中间态回收多 pane 结构。
    let reconcileSuspended = false;

    function ownerPaneOf(path) {
        return panes.value.find((pane) => pane.tabPaths.includes(path)) || null;
    }

    function paneTabs(pane) {
        if (!pane) return [];
        const byPath = new Map(openTabs.value.map((tab) => [tab.path, tab]));
        return pane.tabPaths.map((path) => byPath.get(path)).filter(Boolean);
    }

    /** 把焦点 pane 的 activePath 回写到 App 的 activeTabPath。 */
    function syncActivePath() {
        const pane = findPreviewPane(layout.value, focusedPaneId.value) || panes.value[0];
        const next = pane?.activePath || "";
        if (activeTabPath.value !== next) {
            activeTabPath.value = next;
        }
    }

    /** 归一化单个 pane：剔除已不存在的 tab，修正 activePath。 */
    function normalizePane(pane) {
        const validPaths = new Set(openTabs.value.map((tab) => tab.path));
        const tabPaths = pane.tabPaths.filter((path) => validPaths.has(path));
        let activePath = tabPaths.includes(pane.activePath) ? pane.activePath : "";
        if (!activePath && tabPaths.length) {
            activePath = tabPaths[tabPaths.length - 1];
        }
        if (tabPaths.length === pane.tabPaths.length && activePath === pane.activePath) {
            return pane;
        }
        return { ...pane, tabPaths, activePath };
    }

    /** 回收空 pane：父节点被兄弟节点替换；两块都空时保留第一块（全局至少一个 pane）。 */
    function pruneEmptyPanes(node) {
        if (!node || node.type === "pane") {
            return node;
        }
        const first = pruneEmptyPanes(node.children[0]);
        const second = pruneEmptyPanes(node.children[1]);
        const firstEmpty = first.type === "pane" && first.tabPaths.length === 0;
        const secondEmpty = second.type === "pane" && second.tabPaths.length === 0;
        if (firstEmpty && !secondEmpty) return second;
        if (secondEmpty && !firstEmpty) return first;
        if (firstEmpty && secondEmpty) return first;
        return { ...node, children: [first, second] };
    }

    function updatePaneInTree(node, paneId, updater) {
        if (!node) return node;
        if (node.type === "pane") {
            return node.id === paneId ? updater(node) : node;
        }
        return {
            ...node,
            children: [
                updatePaneInTree(node.children[0], paneId, updater),
                updatePaneInTree(node.children[1], paneId, updater),
            ],
        };
    }

    function replacePaneInTree(node, paneId, build) {
        if (!node) return node;
        if (node.type === "pane") {
            return node.id === paneId ? build(node) : node;
        }
        return {
            ...node,
            children: [
                replacePaneInTree(node.children[0], paneId, build),
                replacePaneInTree(node.children[1], paneId, build),
            ],
        };
    }

    /**
     * 以 openTabs 为唯一事实来源对齐布局：
     * 1. 剔除已关闭 / 失效的 tab；
     * 2. 未归属任何 pane 的新 tab 追加到焦点 pane（安全网，正常路径由 addTab 显式归属）；
     * 3. 修正空 pane 与焦点。
     */
    function reconcile() {
        if (reconcileSuspended) {
            return;
        }
        const mapNode = (node) =>
            node.type === "pane"
                ? normalizePane(node)
                : {
                      ...node,
                      children: [mapNode(node.children[0]), mapNode(node.children[1])],
                  };

        let next = mapNode(layout.value);
        const assigned = new Set(collectPanes(next).flatMap((pane) => pane.tabPaths));
        const orphans = openTabs.value
            .map((tab) => tab.path)
            .filter((path) => path && !assigned.has(path));
        if (orphans.length) {
            const targetId = findPreviewPane(next, focusedPaneId.value)
                ? focusedPaneId.value
                : collectPanes(next)[0]?.id;
            next = updatePaneInTree(next, targetId, (pane) => ({
                ...pane,
                tabPaths: [...pane.tabPaths, ...orphans],
                activePath: orphans[orphans.length - 1],
            }));
        }

        layout.value = pruneEmptyPanes(next);
        const remaining = collectPanes(layout.value);
        if (!remaining.some((pane) => pane.id === focusedPaneId.value)) {
            focusedPaneId.value = remaining[0]?.id || "";
        }
        syncActivePath();
    }

    function updatePane(paneId, updater) {
        layout.value = pruneEmptyPanes(updatePaneInTree(layout.value, paneId, updater));
        const remaining = collectPanes(layout.value);
        if (!remaining.some((pane) => pane.id === focusedPaneId.value)) {
            focusedPaneId.value = remaining[0]?.id || "";
        }
        syncActivePath();
    }

    function setFocusedPane(paneId) {
        if (!paneId || !findPreviewPane(layout.value, paneId)) {
            return;
        }
        if (focusedPaneId.value !== paneId) {
            focusedPaneId.value = paneId;
        }
        syncActivePath();
    }

    function setPaneActive(paneId, path) {
        const pane = findPreviewPane(layout.value, paneId);
        if (!pane || path === pane.activePath) {
            return;
        }
        updatePane(paneId, (current) => ({ ...current, activePath: path }));
    }

    /**
     * 把 tab 归属到指定 pane（默认焦点 pane）并激活。
     * 若该 tab 已属于别的 pane，则视为跨 pane 移动。
     */
    function addTab(path, { paneId = focusedPaneId.value } = {}) {
        if (!path) return;
        const owner = ownerPaneOf(path);
        const targetId = findPreviewPane(layout.value, paneId)
            ? paneId
            : owner?.id || collectPanes(layout.value)[0]?.id;
        if (!targetId) return;
        if (owner && owner.id === targetId) {
            setFocusedPane(targetId);
            setPaneActive(targetId, path);
            return;
        }
        if (owner) {
            updatePane(owner.id, (pane) => {
                const tabPaths = pane.tabPaths.filter((item) => item !== path);
                return {
                    ...pane,
                    tabPaths,
                    activePath:
                        pane.activePath === path
                            ? tabPaths[tabPaths.length - 1] || ""
                            : pane.activePath,
                };
            });
        }
        updatePane(targetId, (pane) => ({
            ...pane,
            tabPaths: pane.tabPaths.includes(path)
                ? pane.tabPaths
                : [...pane.tabPaths, path],
            activePath: path,
        }));
        setFocusedPane(targetId);
    }

    /** 同 pane 内重排（沿用拖动排序的 from / to / after 语义）。 */
    function reorderTab(paneId, fromPath, toPath, after) {
        const pane = findPreviewPane(layout.value, paneId);
        if (!pane) return false;
        const fromIndex = pane.tabPaths.indexOf(fromPath);
        if (fromIndex === -1 || fromPath === toPath) {
            return false;
        }
        const next = [...pane.tabPaths];
        next.splice(fromIndex, 1);
        const toIndex = next.indexOf(toPath);
        if (toIndex === -1) {
            return false;
        }
        next.splice(after ? toIndex + 1 : toIndex, 0, fromPath);
        updatePane(paneId, (current) => ({ ...current, tabPaths: next }));
        return true;
    }

    /** 跨 pane 移动：插到目标 pane 指定位置（默认末尾）并激活。 */
    function moveTabToPane(path, targetPaneId, { index = -1 } = {}) {
        const owner = ownerPaneOf(path);
        const target = findPreviewPane(layout.value, targetPaneId);
        if (!owner || !target || owner.id === target.id) {
            return false;
        }
        updatePane(target.id, (pane) => {
            const tabPaths = pane.tabPaths.filter((item) => item !== path);
            const insertAt =
                index >= 0 && index <= tabPaths.length ? index : tabPaths.length;
            tabPaths.splice(insertAt, 0, path);
            return { ...pane, tabPaths, activePath: path };
        });
        updatePane(owner.id, (pane) => {
            const tabPaths = pane.tabPaths.filter((item) => item !== path);
            return {
                ...pane,
                tabPaths,
                activePath:
                    pane.activePath === path
                        ? tabPaths[tabPaths.length - 1] || ""
                        : pane.activePath,
            };
        });
        setFocusedPane(target.id);
        return true;
    }

    /**
     * 拖动 tab 到目标 pane 的上 / 下 / 左 / 右 1/4 区域：分屏。
     * - 被拖 tab 落到新 pane，新 pane 成为焦点；
     * - 源 pane 只剩这一个 tab 时忽略（否则源 pane 立刻被回收，视觉上等于没分屏）；
     * - 达到 pane 上限时忽略。
     */
    function splitTabToZone({ path, targetPaneId, zone }) {
        if (!PREVIEW_DROP_ZONES.includes(zone) || zone === "center") {
            return false;
        }
        if (!canSplit.value) {
            return false;
        }
        const target = findPreviewPane(layout.value, targetPaneId);
        const owner = ownerPaneOf(path);
        if (!target || !owner || owner.tabPaths.length <= 1) {
            return false;
        }

        const movedPane = {
            ...createPreviewPane(),
            tabPaths: [path],
            activePath: path,
        };
        const nextOwner = {
            ...owner,
            tabPaths: owner.tabPaths.filter((item) => item !== path),
        };
        nextOwner.activePath = nextOwner.tabPaths[nextOwner.tabPaths.length - 1] || "";

        const direction = zone === "left" || zone === "right" ? "row" : "column";
        const children =
            zone === "left" || zone === "top"
                ? [movedPane, nextOwner]
                : [nextOwner, movedPane];

        layout.value = replacePaneInTree(layout.value, target.id, () => ({
            type: "split",
            id: nextSplitId(),
            direction,
            ratio: 0.5,
            children,
        }));
        layout.value = pruneEmptyPanes(layout.value);
        focusedPaneId.value = findPreviewPane(layout.value, movedPane.id)
            ? movedPane.id
            : collectPanes(layout.value)[0]?.id || "";
        syncActivePath();
        return true;
    }

    function setRatio(splitId, ratio) {
        const next = clampPreviewRatio(ratio);
        const mapNode = (node) => {
            if (!node || node.type === "pane") return node;
            if (node.id === splitId) {
                return { ...node, ratio: next };
            }
            return {
                ...node,
                children: [mapNode(node.children[0]), mapNode(node.children[1])],
            };
        };
        layout.value = mapNode(layout.value);
    }

    /** 另存为 / 外部重命名导致 tab path 变化时同步布局。 */
    function replaceTabPath(oldPath, newPath) {
        if (!oldPath || !newPath || oldPath === newPath) return;
        const mapPane = (pane) => {
            if (!pane.tabPaths.includes(oldPath)) return pane;
            return {
                ...pane,
                tabPaths: pane.tabPaths.map((path) => (path === oldPath ? newPath : path)),
                activePath: pane.activePath === oldPath ? newPath : pane.activePath,
            };
        };
        const mapNode = (node) =>
            node.type === "pane"
                ? mapPane(node)
                : {
                      ...node,
                      children: [mapNode(node.children[0]), mapNode(node.children[1])],
                  };
        layout.value = mapNode(layout.value);
        syncActivePath();
    }

    /** 回到单 pane（切换工作区 / 回到首页 / 关闭全部 tab 时调用）。 */
    function resetLayout() {
        layout.value = createPreviewPane();
        focusedPaneId.value = layout.value.id;
        syncActivePath();
    }

    // ---- 会话恢复期间的 reconcile 挂起 ----
    function suspendReconcile() {
        reconcileSuspended = true;
    }

    function resumeReconcile() {
        if (!reconcileSuspended) {
            return;
        }
        reconcileSuspended = false;
        reconcile();
    }

    function serialize() {
        const toPayload = (node) =>
            node.type === "pane"
                ? {
                      type: "pane",
                      id: node.id,
                      tabs: [...node.tabPaths],
                      active: node.activePath,
                  }
                : {
                      type: "split",
                      id: node.id,
                      direction: node.direction,
                      ratio: node.ratio,
                      children: [toPayload(node.children[0]), toPayload(node.children[1])],
                  };
        return {
            root: toPayload(layout.value),
            focusedPaneId: focusedPaneId.value,
        };
    }

    /** 会话恢复：先按存储结构建立骨架，再由 reconcile 过滤失效路径。 */
    function restoreLayout(payload) {
        const build = (node) => {
            if (!node || typeof node !== "object") return null;
            if (node.type === "split" && Array.isArray(node.children)) {
                const first = build(node.children[0]);
                const second = build(node.children[1]);
                if (!first || !second) {
                    return first || second || null;
                }
                return {
                    type: "split",
                    id: String(node.id || nextSplitId()),
                    direction: node.direction === "column" ? "column" : "row",
                    ratio: clampPreviewRatio(node.ratio),
                    children: [first, second],
                };
            }
            if (node.type === "pane") {
                return {
                    type: "pane",
                    id: String(node.id || nextPaneId()),
                    tabPaths: Array.isArray(node.tabs)
                        ? node.tabs.map((path) => String(path || "")).filter(Boolean)
                        : [],
                    activePath: String(node.active || ""),
                };
            }
            return null;
        };

        const root = build(payload?.root);
        if (!root) {
            resetLayout();
            return;
        }
        reserveIdsFrom(root);
        layout.value = root;
        const restored = collectPanes(layout.value);
        focusedPaneId.value = restored.some(
            (pane) => pane.id === payload?.focusedPaneId,
        )
            ? payload.focusedPaneId
            : restored[0]?.id || "";
        if (reconcileSuspended) {
            // 挂起期间只同步激活路径，失效 tab 的清理推迟到 resumeReconcile。
            syncActivePath();
            return;
        }
        reconcile();
    }

    // openTabs 变化后对齐布局（新 tab 归属焦点 pane、关闭 tab 移除、path 变更兜底）。
    // 会话恢复期间会暂时挂起：先按存储建立 pane 骨架，再逐个打开 tab，
    // 否则「openTabs 为空」的中间态会把多 pane 结构回收成单 pane。
    // 用路径字符串作 watch 源：openTabs 中任意 tab 的字段更新（加载状态、脏标记、
    // 内容版本）都会产生新数组，若直接用数组比较会频繁重建布局树并触发全树重渲染，
    // 拖拽期间的重渲染会让拖拽源元素被打断。
    watch(
        () => openTabs.value.map((tab) => tab.path).join("\u0000"),
        () => {
            if (reconcileSuspended) return;
            reconcile();
        },
    );

    // App 直接改写 activeTabPath（如点击已存在于其它 pane 的 tab）时，焦点与激活同步进布局。
    watch(activeTabPath, (path) => {
        if (!path) return;
        const owner = ownerPaneOf(path);
        if (!owner) return;
        if (owner.id !== focusedPaneId.value) {
            focusedPaneId.value = owner.id;
        }
        if (owner.activePath !== path) {
            updatePane(owner.id, (pane) => ({ ...pane, activePath: path }));
        }
    });

    reconcile();

    return {
        layout,
        focusedPaneId,
        panes,
        paneCount,
        focusedPane,
        canSplit,
        maxPanes,
        paneTabs,
        ownerPaneOf,
        setFocusedPane,
        setPaneActive,
        addTab,
        reorderTab,
        moveTabToPane,
        splitTabToZone,
        setRatio,
        replaceTabPath,
        resetLayout,
        reconcile,
        suspendReconcile,
        resumeReconcile,
        serialize,
        restoreLayout,
    };
}





