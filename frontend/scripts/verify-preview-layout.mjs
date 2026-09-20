// 预览区分屏布局的自检脚本（无需测试框架）：
//   node scripts/verify-preview-layout.mjs
// 覆盖落点算法、pane 归属、分屏 / 回收、path 变更与序列化恢复。
import { nextTick, ref } from "vue";
import {
    resolvePreviewDropZone,
    usePreviewLayout,
} from "../src/composables/usePreviewLayout.js";

let failed = 0;
function assert(condition, message) {
    if (condition) {
        console.log("ok   - " + message);
    } else {
        failed += 1;
        console.error("FAIL - " + message);
    }
}

// ---- 落点算法：外圈 1/4 分屏，中央移入 ----
const rect = { left: 0, top: 0, width: 100, height: 100 };
assert(resolvePreviewDropZone(50, 5, rect) === "top", "上 1/4 边缘带 → top");
assert(resolvePreviewDropZone(50, 95, rect) === "bottom", "下 1/4 边缘带 → bottom");
assert(resolvePreviewDropZone(5, 50, rect) === "left", "左 1/4 边缘带 → left");
assert(resolvePreviewDropZone(95, 50, rect) === "right", "右 1/4 边缘带 → right");
assert(resolvePreviewDropZone(50, 50, rect) === "center", "中央 50% → center");
assert(resolvePreviewDropZone(50, 50, null) === "center", "矩形缺失时回落 center");

// ---- 布局操作 ----
const openTabs = ref([]);
const activeTabPath = ref("");
const layout = usePreviewLayout({ openTabs, activeTabPath });

function openTab(path) {
    openTabs.value = [...openTabs.value, { path }];
}

for (const path of ["a", "b", "c", "d"]) {
    openTab(path);
    layout.addTab(path);
}
await nextTick();

assert(layout.paneCount.value === 1, "初始为单 pane");
assert(layout.panes.value[0].tabPaths.join(",") === "a,b,c,d", "单 pane 内 tab 顺序与添加顺序一致");
assert(activeTabPath.value === "d", "activeTabPath 跟随最后添加的 tab");

// 分屏：把 a 拖到 pane 右侧
const sourcePaneId = layout.panes.value[0].id;
assert(
    layout.splitTabToZone({ path: "a", targetPaneId: sourcePaneId, zone: "right" }) === true,
    "拖到右侧 1/4 触发分屏",
);
assert(layout.paneCount.value === 2, "分屏后共有 2 个 pane");
const [firstPane, secondPane] = layout.panes.value;
assert(firstPane.tabPaths.join(",") === "b,c,d", "源 pane 保留其余 tab");
assert(secondPane.tabPaths.join(",") === "a", "新 pane 持有被拖动的 tab");
assert(layout.focusedPaneId.value === secondPane.id, "焦点落在新建的 pane");
assert(activeTabPath.value === "a", "activeTabPath 跟随焦点 pane 的激活 tab");
assert(
    layout.layout.value.type === "split" &&
        layout.layout.value.children[1].id === secondPane.id,
    "新 pane 位于分割的右侧",
);
assert(
    layout.splitTabToZone({ path: "a", targetPaneId: secondPane.id, zone: "top" }) === false,
    "只剩一个 tab 的 pane 不再继续分屏",
);

// 跨 pane 移动
assert(layout.moveTabToPane("c", secondPane.id) === true, "跨 pane 移动成功");
assert(layout.panes.value[1].tabPaths.join(",") === "a,c", "目标 pane 追加被移动的 tab");
assert(layout.panes.value[0].tabPaths.join(",") === "b,d", "源 pane 移除被移动的 tab");
assert(layout.focusedPaneId.value === layout.panes.value[1].id, "移动后焦点切到目标 pane");

// 同 pane 重排
assert(layout.reorderTab(layout.panes.value[1].id, "c", "a", false) === true, "同 pane 重排成功");
assert(layout.panes.value[1].tabPaths.join(",") === "c,a", "重排结果正确");

// pane 数量上限：反复分屏也不会超过 4 个 pane
const limited = usePreviewLayout({
    openTabs: ref([{ path: "1" }, { path: "2" }, { path: "3" }, { path: "4" }]),
    activeTabPath: ref(""),
});
for (const path of ["1", "2", "3"]) {
    limited.addTab(path);
}
let guard = 0;
while (limited.canSplit.value && guard < 8) {
    guard += 1;
    const pane = limited.panes.value.find((item) => item.tabPaths.length > 1);
    if (!pane) break;
    limited.splitTabToZone({
        path: pane.tabPaths[pane.tabPaths.length - 1],
        targetPaneId: pane.id,
        zone: guard % 2 ? "bottom" : "right",
    });
}
assert(limited.paneCount.value <= 4, "pane 数量不超过上限 4，实际 " + limited.paneCount.value);

// path 变更（另存为 / 外部重命名）
layout.replaceTabPath("c", "c2");
openTabs.value = openTabs.value.map((tab) => (tab.path === "c" ? { path: "c2" } : tab));
await nextTick();
assert(layout.panes.value[1].tabPaths.join(",") === "c2,a", "path 变更后 tab 留在原 pane");
assert(activeTabPath.value === "c2", "path 变更后激活项同步");

// 关闭 tab
openTabs.value = openTabs.value.filter((tab) => tab.path !== "c2");
await nextTick();
assert(layout.panes.value[1].tabPaths.join(",") === "a", "关闭 tab 后从原 pane 移除");

// 空 pane 回收
openTabs.value = openTabs.value.filter((tab) => tab.path !== "a");
await nextTick();
assert(layout.paneCount.value === 1, "pane 清空后自动回收为单 pane");
assert(activeTabPath.value === "d", "回收后焦点 pane 的激活项有效");

// 序列化 / 恢复
layout.splitTabToZone({
    path: "b",
    targetPaneId: layout.panes.value[0].id,
    zone: "left",
});
await nextTick();
const snapshot = JSON.parse(JSON.stringify(layout.serialize()));
assert(snapshot.root.type === "split", "序列化输出 split 根节点");
assert(snapshot.root.children[0].tabs.join(",") === "b", "序列化保留各 pane 的 tab 顺序");

const restoredTabs = ref(openTabs.value.map((tab) => ({ path: tab.path })));
const restoredActive = ref("");
const restored = usePreviewLayout({
    openTabs: restoredTabs,
    activeTabPath: restoredActive,
});
restored.suspendReconcile();
restored.restoreLayout(snapshot);
restored.resumeReconcile();
await nextTick();
assert(
    JSON.stringify(restored.panes.value.map((pane) => pane.tabPaths)) ===
        JSON.stringify(layout.panes.value.map((pane) => pane.tabPaths)),
    "恢复后 pane 结构与 tab 归属一致",
);
assert(
    restored.panes.value.map((pane) => pane.id).join(",") ===
        layout.panes.value.map((pane) => pane.id).join(","),
    "恢复保留 pane id（焦点稳定）",
);
assert(
    restored.focusedPaneId.value === restored.panes.value[0].id,
    "恢复的焦点 pane 有效",
);

console.log(failed === 0 ? "\n全部断言通过" : `\n${failed} 条断言失败`);
process.exit(failed === 0 ? 0 : 1);

