<template>
    <TitleBar
        :show-sidebar-toggle="isActualFolderPreview"
        :sidebar-open="sidebarOpen"
        :can-save="canSaveActiveTab"
        :can-save-as="canSaveAsActiveTab"
        @select-folder="handleSelectFolder"
        @select-file="handleSelectFile"
        @toggle-sidebar="toggleSidebar"
        @new-window="handleNewWindow"
        @new-text-file="createBlankTab"
        @save="handleSaveTab"
        @save-as="handleSaveAsActiveTab"
    />
    <main class="page-shell">
        <section v-if="workspaceMode === 'empty'" class="hero">
            <!-- 首页保留 tab 签条（ul）：与工作区一致的样式，
                 可通过右上角 + 按钮 / 双击签条空白处新建空白 tab -->
            <div class="hero__tabbar">
                <PreviewTabs
                    :tabs="openTabs"
                    :active-tab-path="activeTabPath"
                    @new-tab="createBlankTab"
                    @change-tab="handleChangeTab"
                    @close-tab="handleCloseTab"
                />
            </div>
            <div class="hero__panel">
                <div
                    class="hero__dropzone"
                    data-file-drop-target
                    @click="handleSelectFile"
                >
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                        class="hero__dropzone-icon"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5"
                        />
                    </svg>
                    <p class="hero__dropzone-text">拖动到此处打开</p>
                </div>
                <div class="hero__actions">
                    <lay-button class="hero__select-file-btn" @click="handleSelectFile">
                        选择文件
                    </lay-button>
                    <lay-button type="primary" @click="handleSelectFolder">
                        选择文件夹
                    </lay-button>
                    <p v-if="globalError" class="hero__error">
                        {{ globalError }}
                    </p>
                </div>
            </div>
        </section>

        <section v-else class="workspace">
            <div class="workspace__body" :style="workspaceStyle">
                <WorkspaceSidebar
                    v-if="sidebarOpen"
                    :folder-name="folderName"
                    :nodes="treeData"
                    :active-path="activeTabPath"
                    :tree-refreshing="treeRefreshing"
                    @open-file="handleOpenFile"
                    @load-folder="handleLoadFolderChildren"
                    @show-in-file-manager="handleShowInFileManager"
                    @refresh="handleRefreshTree"
                />

                <div
                    v-if="sidebarOpen"
                    class="workspace__resizer"
                    @mousedown.stop.prevent="startResize"   
                ></div>

                <PreviewArea
                    ref="previewArea"
                    :tabs="openTabs"
                    :layout="previewLayout"
                    :focused-pane-id="focusedPreviewPaneId"
                    :pane-count="previewPaneCount"
                    :active-tab-path="activeTabPath"
                    :auto-save-enabled="autoSaveEnabled"
                    :audio-queue-mode="audioQueueMode"
                    :audio-has-prev="audioHasPrev"
                    :audio-has-next="audioHasNext"
                    @change-tab="handlePreviewChangeTab"
                    @close-tab="handleCloseTab"
                    @focus-pane="handleFocusPane"
                    @preview-error="handlePreviewError"
                    @preview-rendered="handlePreviewRendered"
                    @content-change="handleContentChange"
                    @encoding-change="handleEncodingChange"
                    @save-tab="handleSaveTab"
                    @save-as-tab="handleSaveAsTab"
                    @open-in-new-tab="handleOpenInNewTab"
                    @reorder-tab="handleReorderTab"
                    @move-tab="handleMoveTab"
                    @split-tab="handleSplitTab"
                    @resize-split="handleResizeSplit"
                    @new-tab="handleNewTab"
                    @auto-save-toggle="handleAutoSaveToggle"
                    @media-error="handleMediaError"
                    @media-reload="reloadMediaTab"
                    @media-open-system="openMediaWithSystem"
                    @request-prev="(path) => navigateAudioQueue(path, -1)"
                    @request-next="(path, manual) => navigateAudioQueue(path, 1, manual)"
                    @set-queue-mode="handleAudioQueueMode"
                />
            </div>
        </section>
    </main>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { Events, Window } from "@wailsio/runtime";
import TitleBar from "./components/TitleBar.vue";
import WorkspaceSidebar from "./components/WorkspaceSidebar.vue";
import PreviewArea from "./components/PreviewArea.vue";
import PreviewTabs from "./components/PreviewTabs.vue";
import { App } from "../bindings/MostFileViewer";
import { useTheme } from "./composables/useTheme";
import { getMediaType, isEbookExtension } from "./composables/useFileTypes";
import { mediaErrorMessage } from "./composables/useMediaPlayer";
import { useAutoSave } from "./composables/useAutoSave";
import { useWorkspaceSession } from "./composables/useWorkspaceSession";
import { usePreviewLoader } from "./composables/usePreviewLoader";
import { useTabPersistence } from "./composables/useTabPersistence";
import { useFileSystemSync } from "./composables/useFileSystemSync";
import { useTabLifecycle } from "./composables/useTabLifecycle";
import { usePreviewLayout } from "./composables/usePreviewLayout";

const selectedFolder = ref("");
// 当前工作区的真实类型，不再通过 treeData 的形状推断。
const workspaceMode = ref("empty"); // empty | file | folder
const treeData = ref([]);
const leftPaneWidth = ref(320);
const sidebarOpen = ref(true);
const openTabs = ref([]);
const activeTabPath = ref("");
const audioQueueMode = ref("off");
const audioQueuePlayed = new Set();
let mediaSourceVersionCounter = 0;

function nextMediaSourceVersion() {
    mediaSourceVersionCounter += 1;
    return mediaSourceVersionCounter;
}
const previewArea = ref(null);
const globalError = ref("");

// 预览区分屏布局。单 pane 时布局退化为一个 pane 节点，行为与分屏前完全一致；
// activeTabPath 由布局层回写为「焦点 pane 的激活 tab」，因此标题栏保存、
// Ctrl+S、侧栏高亮、音频队列等既有消费点无需感知分屏。
const {
    layout: previewLayout,
    focusedPaneId: focusedPreviewPaneId,
    paneCount: previewPaneCount,
    panes: previewPanes,
    addTab: addPreviewTab,
    reorderTab: reorderPreviewTab,
    moveTabToPane: movePreviewTabToPane,
    splitTabToZone: splitPreviewTabToZone,
    setPaneActive: setPreviewPaneActive,
    setFocusedPane: setFocusedPreviewPane,
    setRatio: setPreviewSplitRatio,
    replaceTabPath: replacePreviewTabPath,
    resetLayout: resetPreviewLayout,
    serialize: serializePreviewLayout,
    restoreLayout: restorePreviewLayout,
    suspendReconcile: suspendPreviewReconcile,
    resumeReconcile: resumePreviewReconcile,
} = usePreviewLayout({ openTabs, activeTabPath });

// 供 useWorkspaceSession 做会话恢复 / 持久化的布局操作集合。
const previewLayoutSession = {
    suspend: suspendPreviewReconcile,
    resume: resumePreviewReconcile,
    restore: restorePreviewLayout,
    serialize: serializePreviewLayout,
    paneIdForPath: (path) =>
        previewPanes.value.find((pane) => pane.tabPaths.includes(path))?.id || "",
    activate: (path) => {
        const owner = previewPanes.value.find((pane) => pane.tabPaths.includes(path));
        if (!owner) {
            return;
        }
        setFocusedPreviewPane(owner.id);
        setPreviewPaneActive(owner.id, path);
    },
};

/** 新 tab 归属：显式指定 pane，未指定则落到当前焦点 pane。 */
function addTabToPane(path, paneId = "") {
    addPreviewTab(path, { paneId: paneId || focusedPreviewPaneId.value });
}
const treeRefreshing = ref(false);
let removeResizeListeners = null;
let removeFilesDroppedListener = null;
let removeThemeChangeListener = null;
let removeFsChangeListener = null;
let removeFileRemovedListener = null;
// 源文件编辑时刷新对应预览 tab 的防抖定时器：源 tab path → timer id
const livePreviewTimers = new Map();
const LIVE_PREVIEW_DELAY = 150;

const { applyRemoteTheme } = useTheme();

// ============================================================================
// 多窗口去重与路径登记
// ============================================================================

/**
 * 检查路径是否已在其他窗口打开
 * @param {string} path - 文件/文件夹路径
 * @returns {Promise<boolean>} true 表示已处理（已聚焦原窗口并关闭当前窗口），false 表示未重复
 */
async function checkAndRedirectIfOpened(path) {
    try {
        const existingWindowID = await App.CheckPathOpened(path);
        if (existingWindowID && existingWindowID > 0) {
            // 已在其他窗口打开，聚焦原窗口并关闭当前窗口
            await App.FocusWindow(existingWindowID);
            await Window.Close();
            return true;
        }
    } catch (error) {
        // 检查失败时不阻断正常流程
    }
    return false;
}

/**
 * 注册当前窗口打开的路径
 */
async function registerOpenPath(path) {
    try {
        await App.RegisterOpenPath(path);
    } catch (error) {
        // silently ignore
    }
}

/**
 * 注销当前窗口不再打开的路径
 */
async function unregisterOpenPath(path) {
    try {
        await App.UnregisterOpenPath(path);
    } catch (error) {
        // silently ignore
    }
}

/**
 * 注销当前窗口所有已打开的路径（切换工作区或返回首页时调用）
 */
async function unregisterAllOpenPaths() {
    const paths = new Set();

    // 文件夹模式下注销文件夹路径
    if (isActualFolderPreview.value && selectedFolder.value) {
        paths.add(selectedFolder.value);
    }

    // 注销所有 tab 文件路径
    for (const tab of openTabs.value) {
        if (tab.path) {
            paths.add(tab.path);
        }
    }

    for (const path of paths) {
        await unregisterOpenPath(path);
    }
}

/**
 * 新建窗口
 */
async function handleNewWindow() {
    try {
        await App.NewWindow();
    } catch (error) {
        // silently ignore
    }
}

// 自动保存相关
const encodingChangeRequests = new Map();
// 自动保存开关状态，初始从 localStorage 恢复，默认为开启。
const AUTO_SAVE_STORAGE_KEY = "mfv-auto-save";
const autoSaveEnabled = ref(readAutoSaveSetting());
let saveTabForAutoSave = () => Promise.resolve();
let saveTabForLifecycle = () => Promise.resolve();
function readAutoSaveSetting() {
    try {
        const stored = localStorage.getItem(AUTO_SAVE_STORAGE_KEY);
        if (stored === "false") return false;
        return true;
    } catch (e) {
        return true;
    }
}
function persistAutoSaveSetting(enabled) {
    try {
        localStorage.setItem(AUTO_SAVE_STORAGE_KEY, enabled ? "true" : "false");
    } catch (e) {
        // localStorage 不可用时静默忽略。
    }
}
function handleAutoSaveToggle(enabled) {
    autoSaveEnabled.value = enabled;
    persistAutoSaveSetting(enabled);
    // 关闭自动保存时清理尚未触发的防抖定时器，避免关闭后又悄悄保存。
    if (!enabled) {
        clearAllAutoSaveDebounceTimers();
    }
}

const {
    schedule: scheduleAutoSave,
    clear: clearAutoSaveDebounceTimer,
    clearAll: clearAllAutoSaveDebounceTimers,
    start: startAutoSaveInterval,
    stop: stopAutoSaveTimers,
} = useAutoSave({
    tabs: openTabs,
    enabled: autoSaveEnabled,
    saveTab: (path) => saveTabForAutoSave(path),
});

const {
    restore: restoreWorkspaceSession,
    schedulePersist: schedulePersistWorkspaceSession,
    persistNow: persistWorkspaceSessionNow,
    clear: clearWorkspaceSession,
    stop: stopWorkspaceSession,
} = useWorkspaceSession({
    workspaceMode,
    selectedFolder,
    treeData,
    openTabs,
    activeTabPath,
    sidebarOpen,
    leftPaneWidth,
    getParentPath,
    getPathName,
    getPathExtension,
    registerOpenPath,
    openFileNode,
    layoutSession: previewLayoutSession,
});

const {
    loadTabPreview,
    revokeMediaToken,
    revokeTabMedia,
} = usePreviewLoader({
    getPreviewType,
    nextMediaSourceVersion,
});

const {
    save: handleSaveTab,
    saveAs: handleSaveAsTab,
    saveDirty: saveDirtyTabs,
} = useTabPersistence({
    tabs: openTabs,
    activePath: activeTabPath,
    selectedFolder,
    previewArea,
    autoSaveEnabled,
    clearAutoSave: clearAutoSaveDebounceTimer,
    scheduleAutoSave,
    updateTab,
    registerOpenPath,
    schedulePersist: schedulePersistWorkspaceSession,
    getPathName,
    getPathExtension,
    normalizeError,
    replaceTabPath: replacePreviewTabPath,
});
saveTabForAutoSave = handleSaveTab;

const {
    handleReorderTab: reorderTabsOrder,
    handleOpenInNewTab,
    handleNewTab,
    handleCloseTab,
    releaseTabPayload,
    releaseAllTabPayloads,
} = useTabLifecycle({
    tabs: openTabs,
    activePath: activeTabPath,
    selectedFolder,
    treeData,
    workspaceMode,
    untitledCounter: () => ++untitledCounter,
    saveTab: (path) => saveTabForLifecycle(path),
    clearAutoSave: clearAutoSaveDebounceTimer,
    encodingChangeRequests,
    revokeTabMedia,
    updateTab,
    unregisterOpenPath,
    clearLivePreviewTimer: (path) => clearLivePreviewTimer(path),
    schedulePersist: schedulePersistWorkspaceSession,
    clearWorkspaceSession,
    resolveFallbackActive: pickFallbackActivePath,
});
saveTabForLifecycle = handleSaveTab;

// 新建空白（未命名可编辑）tab 的统一入口：标题栏「文件 → 新建文本文件」与
// 首页 tab 签条的「+」按钮共用。首页（empty 模式）只渲染 tab 签条，首个空白
// tab 创建前需切换到 file 模式让工作区（含 tab 内容区）渲染；file 模式没有
// 文件夹树，与单文件打开行为一致地隐藏侧边栏。
function createBlankTab() {
    if (workspaceMode.value === "empty") {
        workspaceMode.value = "file";
        sidebarOpen.value = false;
    }
    handleNewTab();
}

const {
    handleFsChange,
    handleFileRemoved,
    updateTreeNode,
} = useFileSystemSync({
    selectedFolder,
    treeData,
    openTabs,
    activeTabPath,
    isFolderPreview: () => isActualFolderPreview.value,
    getPathName,
    getPathExtension,
    getPreviewType,
    nextMediaSourceVersion,
    loadTabPreview,
    buildLoadedTabPatch,
    updateTab,
    revokeTabMedia,
    revokeMediaToken,
    registerOpenPath,
    unregisterOpenPath,
    normalizeError,
    replaceTabPath: replacePreviewTabPath,
});

// 未命名（空白可编辑）tab 的自增计数器，保证多个未命名 tab 的 path 与 name 互不重复。
let untitledCounter = 0;

const workspaceStyle = computed(() => ({
    gridTemplateColumns: sidebarOpen.value
        ? `${leftPaneWidth.value}px 4px minmax(0, 1fr)`
        : "minmax(0, 1fr)",
}));

const folderName = computed(() => {
    if (!selectedFolder.value) return "";
    const trimmed = selectedFolder.value.replace(/[\\/]+$/, "");
    return trimmed.split(/[\\/]/).pop() || trimmed;
});

// 文件模式下 selectedFolder 保存文件所在目录，不能用它判断是否为文件夹工作区。
const isActualFolderPreview = computed(() => workspaceMode.value === "folder");

function findSiblingNodes(nodes, targetPath) {
    for (const node of nodes || []) {
        if (node.path === targetPath) {
            return nodes;
        }
        const found = findSiblingNodes(node.children, targetPath);
        if (found) {
            return found;
        }
    }
    return [];
}

const audioQueue = computed(() => {
    const activePath = activeTabPath.value;
    if (!activePath) {
        return [];
    }
    return findSiblingNodes(treeData.value, activePath).filter(
        (node) =>
            node?.type === "file" &&
            getPreviewType(node.extension) === "audio",
    );
});

const audioQueueIndex = computed(() =>
    audioQueue.value.findIndex((node) => node.path === activeTabPath.value),
);

const audioHasPrev = computed(
    () => audioQueueIndex.value > 0,
);

const audioHasNext = computed(() => {
    if (audioQueue.value.length < 2 || audioQueueIndex.value < 0) {
        return false;
    }
    if (audioQueueMode.value === "shuffle") {
        return audioQueue.value.some(
            (node) =>
                node.path !== activeTabPath.value &&
                !audioQueuePlayed.has(node.path),
        );
    }
    return audioQueueIndex.value < audioQueue.value.length - 1;
});

// 当前激活 tab 的元数据，用于标题栏菜单的「保存 / 另存为」可用性判断。
// 语义与 CodePreview 原状态栏的 canSave / canSaveAs 保持一致：
// - canSaveActiveTab：实文件 + dirty + ready（virtual 永远走另存为）
// - canSaveAsActiveTab：code 预览 + ready + 未在保存/切编码中（实文件和 virtual 都允许）
const activeCodeTab = computed(() => {
    const path = activeTabPath.value;
    if (!path) return null;
    return openTabs.value.find((tab) => tab.path === path) || null;
});
const canSaveActiveTab = computed(
    () =>
        !!activeCodeTab.value &&
        !activeCodeTab.value.virtual &&
        activeCodeTab.value.previewType === "code" &&
        activeCodeTab.value.status === "ready" &&
        activeCodeTab.value.dirty === true,
);
const canSaveAsActiveTab = computed(
    () =>
        !!activeCodeTab.value &&
        activeCodeTab.value.previewType === "code" &&
        activeCodeTab.value.status === "ready" &&
        activeCodeTab.value.saving !== true &&
        activeCodeTab.value.encodingLoading !== true,
);

async function handleSaveAsActiveTab() {
    // 标题栏「另存为」入口：实文件副本路径（不强制 alwaysAsCopy；virtual 走首次落地）
    await handleSaveAsTab(activeTabPath.value);
}

onMounted(() => {
    window.addEventListener("keydown", handleGlobalShortcut);
    startAutoSaveInterval();
    removeFilesDroppedListener = Events.On(
        "files-dropped",
        handleFilesDropped,
    );

    // 监听后端主题广播（多窗口同步备通道）
    // 收到事件时仅更新 ref 和 DOM，不回写 localStorage，避免循环触发
    removeThemeChangeListener = Events.On("theme-changed", (event) => {
        applyRemoteTheme(event.data);
    });

    // 监听后端文件系统变化：局部刷新文件树
    removeFsChangeListener = Events.On("fs-change", handleFsChange);
    // 监听文件被外部删除：联动 tab 状态
    removeFileRemovedListener = Events.On("file-removed", handleFileRemoved);

    void restoreWorkspaceSession();
});

onBeforeUnmount(() => {
    void Promise.all(openTabs.value.map((tab) => revokeTabMedia(tab)));
    stopResize();
    stopAutoSave();
    stopWorkspaceSession();
    window.removeEventListener("keydown", handleGlobalShortcut);
    removeFilesDroppedListener?.();
    removeThemeChangeListener?.();
    removeFsChangeListener?.();
    removeFileRemovedListener?.();
});

async function handleSelectFile() {
    globalError.value = "";

    try {
        const filePath = await App.SelectFile();
        if (!filePath) {
            return;
        }

        // 去重检查：如果已在其他窗口打开，直接关闭当前窗口
        if (await checkAndRedirectIfOpened(filePath)) {
            return;
        }

        // 如果已有工作区，直接在预览区域添加新tab
        if (workspaceMode.value !== "empty") {
            await addFileToWorkspace(filePath);
        } else {
            // 没有工作区时，打开单个文件（不显示侧边栏）
            await openFileInWorkspace(filePath);
        }
    } catch (error) {
        // silently ignore
    }
}

async function openFileInWorkspace(filePath) {
    const fileName = getPathName(filePath);
    const fileNode = {
        name: fileName,
        path: filePath,
        type: "file",
        extension: getPathExtension(fileName),
    };

    if (!(await saveDirtyTabs())) {
        return;
    }

    await replaceWorkspace({
        mode: "file",
        rootPath: getParentPath(filePath),
        tree: [fileNode],
        sidebarOpen: false,
        openNode: fileNode,
    });
}

async function replaceWorkspace({
    mode,
    rootPath,
    tree,
    sidebarOpen: nextSidebarOpen,
    openNode = null,
}) {
    // 注销旧工作区的已打开路径
    await unregisterAllOpenPaths();

    workspaceMode.value = mode;
    selectedFolder.value = rootPath;
    treeData.value = tree;
    sidebarOpen.value = nextSidebarOpen;
    clearAllAutoSaveDebounceTimers();
    clearAllLivePreviewTimers();
    await releaseAllTabPayloads();
    await nextTick();
    openTabs.value = [];
    activeTabPath.value = "";
    // 新工作区回到单 pane 布局。
    resetPreviewLayout();

    if (openNode) {
        await openFileNode(openNode);
    } else {
        await registerOpenPath(rootPath);
    }
    schedulePersistWorkspaceSession();
}

// 在已有工作区中直接添加新tab（不清除现有tabs）
async function addFileToWorkspace(filePath) {
    const fileName = getPathName(filePath);
    const fileNode = {
        name: fileName,
        path: filePath,
        type: "file",
        extension: getPathExtension(fileName),
    };

    // 检查文件是否已在tabs中
    const existingTab = openTabs.value.find((tab) => tab.path === filePath);
    if (existingTab) {
        activeTabPath.value = existingTab.path;
        return;
    }

    // 直接打开新tab
    await openFileNode(fileNode);
    schedulePersistWorkspaceSession();
}

function getParentPath(path) {
    const match = String(path || "").match(/^(.*)[\\/][^\\/]+$/);
    return match?.[1] || path;
}

function getPathName(path) {
    return (
        String(path || "")
            .split(/[\\/]/)
            .pop() || path
    );
}

function getPathExtension(path) {
    const name = getPathName(path).toLowerCase();
    const extensionStart = name.lastIndexOf(".");
    return extensionStart > 0 ? name.slice(extensionStart) : "";
}

// 另存为对话框的建议文件名：
// - virtual：使用 tab 上显示的名字（编辑时由 deriveUntitledName 派生为内容前 10 字符或「未命名」）
//   加上 syntax 推断的后缀；过滤掉文件名非法字符，避免对话框兜底
// - 实文件副本："<原名> (副本).<ext>"
async function handleFilesDropped(event) {
    globalError.value = "";

    const items = event.data;
    if (!Array.isArray(items) || items.length === 0) {
        return;
    }

    // 仅处理第一个拖入项
    const item = items[0];
    if (!item?.path) {
        return;
    }

    // 去重检查
    if (await checkAndRedirectIfOpened(item.path)) {
        return;
    }

    if (item.isDir) {
        await handleOpenFolder(item.path);
    } else {
        // 如果已有工作区，直接添加新tab
        if (workspaceMode.value !== "empty") {
            await addFileToWorkspace(item.path);
        } else {
            await openFileInWorkspace(item.path);
        }
    }
}

async function handleOpenFolder(folderPath) {
    try {
        if (!(await saveDirtyTabs())) {
            return;
        }

        const tree = await App.LoadFolderTree(folderPath);
        await replaceWorkspace({
            mode: "folder",
            rootPath: folderPath,
            tree,
            sidebarOpen: true,
        });
    } catch (error) {
        // silently ignore
    }
}

async function handleLoadFolderChildren(node) {
    if (!node || node.type !== "folder" || node.loaded) {
        return;
    }

    try {
        const children = await App.LoadFolderChildren(node.path);
        treeData.value = updateTreeNode(treeData.value, node.path, {
            children,
            loaded: true,
            hasChild: children.length > 0,
        });
    } catch (error) {
        treeData.value = updateTreeNode(treeData.value, node.path, {
            loaded: true,
            hasChild: false,
        });
    }
}

// 刷新文件列表：按当前根目录重新扫描整棵树。已打开的 tab 不受影响；
// 同一根目录下后端不会重置 watcher / 白名单，已展开节点靠 path key 保留展开状态。
async function handleRefreshTree() {
    if (treeRefreshing.value || !isActualFolderPreview.value) {
        return;
    }

    treeRefreshing.value = true;
    try {
        treeData.value = await App.LoadFolderTree(selectedFolder.value);
    } catch (error) {
        // silently ignore，与打开文件夹失败的处理保持一致
    } finally {
        treeRefreshing.value = false;
    }
}

async function openFileNode(node, paneId = "") {
    const initialPreviewType = getPreviewType(node.extension);
    const sourceVersion =
        initialPreviewType === "audio" || initialPreviewType === "video"
            ? nextMediaSourceVersion()
            : 0;
    const tab = {
        path: node.path,
        name: node.name,
        extension: node.extension,
        status: "loading",
        previewType: initialPreviewType,
        source: null,
        content: "",
        encoding: "utf-8",
        dirty: false,
        saving: false,
        saveError: "",
        encodingLoading: false,
        contentVersion: 0,
        changeVersion: 0,
        savedVersion: 0,
        media: sourceVersion
            ? {
                  token: "",
                  sourceVersion,
                  capability: "unknown",
                  loadState: "idle",
                  errorCode: "",
                  errorMessage: "",
              }
            : undefined,
    };

    openTabs.value = [...openTabs.value, tab];
    // 归属到指定 pane（未指定则为当前焦点 pane）；随后才设置激活路径，
    // 使「新打开的 tab 出现在焦点区块」这一语义在分屏下同样成立。
    addTabToPane(tab.path, paneId);
    activeTabPath.value = tab.path;

    try {
        await registerOpenPath(node.path);
        const loaded = await loadTabPreview(node.path, node.extension, sourceVersion);
        const { isMediaPreview } = loaded;
        const currentTab = openTabs.value.find((item) => item.path === node.path);
        if (
            !currentTab ||
            (isMediaPreview &&
                currentTab.media?.sourceVersion !== loaded.media?.sourceVersion)
        ) {
            await revokeMediaToken(loaded.media?.token);
            return;
        }
        updateTab(
            node.path,
            buildLoadedTabPatch(loaded, node.extension, currentTab),
        );

        // 成功打开文件后登记该 tab 路径
        schedulePersistWorkspaceSession();
    } catch (error) {
        updateTab(node.path, {
            status: "error",
            error: normalizeError(error, "读取文件失败"),
        });
        schedulePersistWorkspaceSession();
    }
}

function buildLoadedTabPatch(loaded, fallbackExtension, currentTab) {
    const { content, previewType, isCodePreview, isMediaPreview } = loaded;
    return {
        extension: content.extension || fallbackExtension,
        previewType,
        size: Number(content.size || 0),
        source: loaded.source,
        media: isMediaPreview ? loaded.media : undefined,
        content: isCodePreview ? content.content || "" : "",
        encoding: content.encoding || "utf-8",
        dirty: false,
        saving: false,
        saveError: "",
        encodingLoading: false,
        contentVersion: isCodePreview
            ? (currentTab?.contentVersion ?? 0) + 1
            : 0,
        changeVersion: 0,
        savedVersion: 0,
        status: "ready",
        error: "",
    };
}

async function handleSelectFolder() {
    globalError.value = "";

    try {
        const folder = await App.SelectFolder();
        if (!folder) {
            return;
        }

        // 去重检查
        if (await checkAndRedirectIfOpened(folder)) {
            return;
        }

        if (!(await saveDirtyTabs())) {
            return;
        }

        const tree = await App.LoadFolderTree(folder);
        await replaceWorkspace({
            mode: "folder",
            rootPath: folder,
            tree,
            sidebarOpen: true,
        });
    } catch (error) {
        // silently ignore
    }
}

async function handleOpenFile(node) {
    globalError.value = "";

    const existingTab = openTabs.value.find((tab) => tab.path === node.path);
    if (existingTab) {
        activeTabPath.value = existingTab.path;
        return;
    }

    await openFileNode(node);
}

async function handleShowInFileManager(node) {
    if (!node?.path) {
        return;
    }

    try {
        await App.ShowInFileManager(node.path);
    } catch (error) {
        // silently ignore
    }
}

function handleChangeTab(path) {
    activeTabPath.value = path;
    schedulePersistWorkspaceSession();
}

// ---- 分屏布局交互（由 PreviewArea 上抛）----

/** pane 内切换激活 tab：先聚焦该 pane，再设置其激活项。 */
function handlePreviewChangeTab(paneId, path) {
    setFocusedPreviewPane(paneId);
    setPreviewPaneActive(paneId, path);
    handleChangeTab(path);
}

/** 点击 / 操作某个 pane 即聚焦它，决定后续新打开 tab 的归属。 */
function handleFocusPane(paneId) {
    setFocusedPreviewPane(paneId);
}

/**
 * 关闭 tab 后决定新的激活项：优先「同 pane 内关闭位置的后一个 / 前一个 tab」，
 * 避免分屏时焦点跳到其它区块；找不到同 pane 邻居时回退到全局相邻项。
 * 注意：调用时 openTabs 已移除该 tab，但布局要到下一次 reconcile 才同步，
 * 因此这里仍能从 pane.tabPaths 里读到被关闭 tab 的位置。
 */
function pickFallbackActivePath({ closedPath, nextTabs, currentIndex }) {
    const pane = previewPanes.value.find((item) => item.tabPaths.includes(closedPath));
    if (pane) {
        const index = pane.tabPaths.indexOf(closedPath);
        const candidates = [
            pane.tabPaths[index + 1],
            pane.tabPaths[index - 1],
            ...pane.tabPaths,
        ];
        const nextPath = candidates.find(
            (path) =>
                path &&
                path !== closedPath &&
                nextTabs.some((tab) => tab.path === path),
        );
        if (nextPath) {
            return nextPath;
        }
    }
    return nextTabs[currentIndex]?.path || nextTabs[currentIndex - 1]?.path || "";
}

/**
 * 同一 pane 内拖动排序：布局顺序（决定 tab 条与持久化）与 openTabs 顺序同步维护，
 * 后者与视觉顺序保持一致，便于复用其它「按数组顺序」处理 tab 的逻辑。
 */
function handleReorderTab(payload) {
    const { paneId, fromPath, toPath, after } = payload || {};
    if (!fromPath || !toPath || fromPath === toPath) {
        return;
    }
    // paneId 缺失时回退到焦点 pane：否则只会重排 openTabs，
    // 而 tab 条顺序取自布局，视觉上会看不到任何变化。
    const targetPaneId =
        paneId || focusedPreviewPaneId.value || previewPanes.value[0]?.id || "";
    if (targetPaneId) {
        reorderPreviewTab(targetPaneId, fromPath, toPath, after);
    }
    reorderTabsOrder({ fromPath, toPath, after });
    schedulePersistWorkspaceSession();
}

/** 跨 pane 移动 tab：插入到目标 pane 的指定位置（缺省追加到末尾）。 */
function handleMoveTab(payload) {
    const { fromPath, toPaneId, beforePath, after } = payload || {};
    if (!fromPath || !toPaneId) {
        return;
    }
    const target = previewPanes.value.find((pane) => pane.id === toPaneId);
    let index = -1;
    if (target && beforePath) {
        const at = target.tabPaths.indexOf(beforePath);
        if (at !== -1) {
            index = after ? at + 1 : at;
        }
    }
    if (movePreviewTabToPane(fromPath, toPaneId, { index })) {
        schedulePersistWorkspaceSession();
    }
}

/** 拖动 tab 到目标 pane 的上 / 下 / 左 / 右 1/4 区域：分屏。 */
function handleSplitTab(payload) {
    const { fromPath, targetPaneId, zone } = payload || {};
    if (!fromPath || !targetPaneId || !zone) {
        return;
    }
    if (splitPreviewTabToZone({ path: fromPath, targetPaneId, zone })) {
        schedulePersistWorkspaceSession();
    }
}

/** 拖动分隔条调整两个区块的比例。 */
function handleResizeSplit(payload) {
    const { splitId, ratio } = payload || {};
    if (!splitId) {
        return;
    }
    setPreviewSplitRatio(splitId, ratio);
    schedulePersistWorkspaceSession();
}

function handleAudioQueueMode(mode) {
    audioQueueMode.value = ["off", "sequential", "shuffle"].includes(mode)
        ? mode
        : "off";
    audioQueuePlayed.clear();
    if (activeTabPath.value) {
        audioQueuePlayed.add(activeTabPath.value);
    }
}

async function navigateAudioQueue(path, direction, manual = false) {
    const queue = audioQueue.value;
    const currentIndex = queue.findIndex((node) => node.path === path);
    if (currentIndex < 0 || queue.length < 2) {
        return;
    }
    if (!manual && audioQueueMode.value === "off") {
        return;
    }

    let nextNode = null;
    if (direction > 0 && audioQueueMode.value === "shuffle" && !manual) {
        const remaining = queue.filter(
            (node) =>
                node.path !== path && !audioQueuePlayed.has(node.path),
        );
        if (!remaining.length) {
            return;
        }
        nextNode = remaining[Math.floor(Math.random() * remaining.length)];
    } else {
        const nextIndex = currentIndex + direction;
        if (nextIndex < 0 || nextIndex >= queue.length) {
            return;
        }
        nextNode = queue[nextIndex];
    }

    audioQueuePlayed.add(path);
    audioQueuePlayed.add(nextNode.path);
    await handleOpenFile(nextNode);
}

function handleMediaError(path, payload) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab?.media) {
        return;
    }
    const token = tab.media.token;
    updateTab(path, {
        media: {
            ...tab.media,
            token: "",
            loadState: "failed",
            errorCode: payload?.code || "unknown",
            errorMessage:
                payload?.message || mediaErrorMessage(payload?.code || "unknown"),
        },
    });
    void revokeMediaToken(token);
}

async function openMediaWithSystem(path) {
    try {
        await App.OpenMediaWithSystem(path);
    } catch (error) {
        const tab = openTabs.value.find((item) => item.path === path);
        if (tab?.media) {
            updateTab(path, {
                media: {
                    ...tab.media,
                    notice: normalizeError(error, "无法启动系统播放器"),
                },
            });
        }
    }
}

async function reloadMediaTab(path) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab || !["audio", "video"].includes(tab.previewType)) {
        return;
    }
    const sourceVersion = nextMediaSourceVersion();
    await revokeTabMedia(tab);
    updateTab(path, {
        source: null,
        media: {
            ...(tab.media || {}),
            token: "",
            sourceVersion,
            loadState: "loading",
            errorCode: "",
            errorMessage: "",
            notice: "",
        },
    });

    try {
        const loaded = await loadTabPreview(path, tab.extension, sourceVersion);
        const { content, previewType } = loaded;
        const latest = openTabs.value.find((item) => item.path === path);
        if (!latest?.media || latest.media.sourceVersion !== sourceVersion) {
            await revokeMediaToken(loaded.media?.token);
            return;
        }
        updateTab(path, {
            extension: content.extension || tab.extension,
            previewType,
            size: Number(content.size || 0),
            source: loaded.source || null,
            media: loaded.media || latest.media,
        });
    } catch (error) {
        const latest = openTabs.value.find((item) => item.path === path);
        if (latest?.media?.sourceVersion === sourceVersion) {
            updateTab(path, {
                media: {
                    ...latest.media,
                    loadState: "failed",
                    errorCode: "unknown",
                    errorMessage: normalizeError(error, "重新加载媒体失败"),
                },
            });
        }
    }
}

function handlePreviewError(path, error) {
    const message = normalizeError(error, "预览失败");
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab) {
        return;
    }
    if (tab.status === "error" && tab.error === message) {
        return;
    }

    updateTab(path, {
        status: "error",
        error: message,
    });
}

function handlePreviewRendered(path) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab || tab.previewType !== "ppt" || !tab.source) {
        return;
    }

    updateTab(path, { source: null });
}

function handleContentChange(path) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab || tab.previewType !== "code" || tab.status !== "ready") {
        return;
    }

    const updates = {
        dirty: true,
        saveError: "",
        changeVersion: (tab.changeVersion ?? 0) + 1,
    };

    // 未命名（virtual）tab：以编辑器内首段内容的前 10 个字符作为标签名，
    // 内容清空后恢复为「未命名」/「未命名 N」。仅 virtual 走此规则，普通文件保留原名。
    if (tab.virtual) {
        const latest =
            previewArea.value?.getCodeContent?.(path) ?? tab.content ?? "";
        updates.name = deriveUntitledName(latest);
    }

    updateTab(path, updates);

    scheduleLivePreviewSync(path);
    if (!tab.virtual && autoSaveEnabled.value) {
        scheduleAutoSave(path);
    }
}

// 派生未命名 tab 的标签名：去掉首尾空白与换行，按字符（code point）切前 10 个；
// 内容为空时返回默认名（带计数器后缀）。
function deriveUntitledName(content) {
    const flat = String(content || "").replace(/[\r\n]+/g, " ").trim();
    if (!flat) {
        return untitledCounter > 1 ? `未命名 ${untitledCounter}` : "未命名";
    }
    const sliced = [...flat].slice(0, 10).join("");
    return sliced || (untitledCounter > 1 ? `未命名 ${untitledCounter}` : "未命名");
}

// 源文件编辑时，防抖刷新其对应的只读预览 tab，实现「编辑即预览」。
function scheduleLivePreviewSync(sourcePath) {
    const previewPath = `preview://${sourcePath}`;
    if (!openTabs.value.some((tab) => tab.path === previewPath)) {
        return;
    }

    const existingTimer = livePreviewTimers.get(sourcePath);
    if (existingTimer) {
        clearTimeout(existingTimer);
    }

    const timer = setTimeout(() => {
        livePreviewTimers.delete(sourcePath);
        const latest = previewArea.value?.getCodeContent?.(sourcePath);
        if (latest === undefined || latest === null) {
            return;
        }
        updateTab(previewPath, { content: latest });
    }, LIVE_PREVIEW_DELAY);

    livePreviewTimers.set(sourcePath, timer);
}

async function handleEncodingChange(path, encoding) {
    let tab = openTabs.value.find((item) => item.path === path);
    if (
        !tab ||
        tab.previewType !== "code" ||
        tab.status !== "ready" ||
        tab.virtual
    ) {
        return;
    }
    if (tab.encoding === encoding || tab.encodingLoading) {
        return;
    }
    if (tab.dirty) {
        await handleSaveTab(path);
        tab = openTabs.value.find((item) => item.path === path);
        if (!tab || tab.dirty || tab.saving || tab.status !== "ready") {
            return;
        }
    }

    const requestToken = Symbol("encoding-change");
    encodingChangeRequests.set(path, requestToken);
    clearAutoSaveDebounceTimer(path);
    updateTab(path, { encodingLoading: true, saveError: "" });
    try {
        const content = await App.ReadFileWithEncoding(tab.path, encoding);
        if (encodingChangeRequests.get(path) !== requestToken) {
            return;
        }
        const currentTab = openTabs.value.find((item) => item.path === path);
        if (!currentTab) {
            return;
        }
        updateTab(path, {
            content: content.content || "",
            encoding: content.encoding || encoding,
            dirty: false,
            saving: false,
            saveError: "",
            encodingLoading: false,
            contentVersion: (currentTab.contentVersion ?? 0) + 1,
            changeVersion: 0,
            savedVersion: 0,
        });
    } catch (error) {
        if (encodingChangeRequests.get(path) !== requestToken) {
            return;
        }
        updateTab(path, {
            encodingLoading: false,
            saveError: normalizeError(error, "切换编码失败"),
        });
    } finally {
        if (encodingChangeRequests.get(path) === requestToken) {
            encodingChangeRequests.delete(path);
        }
    }
}

function handleGlobalShortcut(event) {
    const ctrl = event.ctrlKey || event.metaKey;
    if (!ctrl) {
        return;
    }
    if (event.key.toLowerCase() !== "s") {
        return;
    }
    event.preventDefault();
    if (event.shiftKey) {
        // Ctrl+Shift+S：始终走另存为（virtual 落地 / 实文件副本）
        void handleSaveAsTab(activeTabPath.value, { alwaysAsCopy: true });
    } else {
        // Ctrl+S：virtual → 另存为；实文件 → 原地保存
        void handleSaveTab();
    }
}

function toggleSidebar() {
    // 单个文件模式下不允许打开侧边栏
    if (!isActualFolderPreview.value) {
        sidebarOpen.value = false;
        schedulePersistWorkspaceSession();
        return;
    }
    sidebarOpen.value = !sidebarOpen.value;
    if (!sidebarOpen.value) {
        stopResize();
    }
    schedulePersistWorkspaceSession();
}

function updateTab(path, patch) {
    openTabs.value = openTabs.value.map((tab) =>
        tab.path === path ? { ...tab, ...patch } : tab,
    );
}

// 在 getPreviewType 函数中添加音频和视频支持
function getPreviewType(extension) {
  const normalized = (extension || "").toLowerCase()
  if (normalized === ".docx") {
    return "word"
  }
  if (normalized === ".csv") {
    return "csv"
  }
  if ([".xlsx", ".xlsm", ".xltx", ".xltm"].includes(normalized)) {
    return "excel"
  }
  if ([".pptx", ".pptm", ".ppsx", ".ppsm"].includes(normalized)) {
    return "ppt"
  }
  if (normalized === ".pdf") {
    return "pdf"
  }
  if (isEbookExtension(normalized)) {
    return "ebook"
  }
  if (isImageExtension(normalized)) {
    return "image"
  }
  const mediaType = getMediaType(normalized)
  if (mediaType) {
    return mediaType
  }
  if (normalized === ".xls") {
    return "unsupported"
  }
  return "code"
}

function isImageExtension(extension) {
    return [
        ".png",
        ".jpg",
        ".jpeg",
        ".gif",
        ".webp",
        ".bmp",
        ".svg",
        ".ico",
        ".avif",
    ].includes(extension);
}

function normalizeError(error, fallback) {
    const message = String(error ?? "").trim();
    return message || fallback;
}

function startResize(event) {
    if (window.innerWidth < 768) {
        return;
    }

    stopResize();

    const startX = event.clientX;
    const startWidth = leftPaneWidth.value;

    const onMouseMove = (moveEvent) => {
        if (moveEvent.buttons === 0) {
            stopResize();
            return;
        }

        const nextWidth = startWidth + moveEvent.clientX - startX;
        const maxWidth = Math.min(window.innerWidth * 0.5, 560);
        leftPaneWidth.value = Math.min(Math.max(nextWidth, 240), maxWidth);
    };

    const onMouseUp = () => {
        stopResize();
        schedulePersistWorkspaceSession();
    };

    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    document.addEventListener("mouseleave", onMouseUp);
    removeResizeListeners = () => {
        document.removeEventListener("mousemove", onMouseMove);
        document.removeEventListener("mouseup", onMouseUp);
        document.removeEventListener("mouseleave", onMouseUp);
        removeResizeListeners = null;
    };
}

function stopResize() {
    removeResizeListeners?.();
}

function clearLivePreviewTimer(sourcePath) {
    const timer = livePreviewTimers.get(sourcePath);
    if (!timer) {
        return;
    }
    clearTimeout(timer);
    livePreviewTimers.delete(sourcePath);
}

function clearAllLivePreviewTimers() {
    livePreviewTimers.forEach((timer) => clearTimeout(timer));
    livePreviewTimers.clear();
}

function stopAutoSave() {
    stopAutoSaveTimers();
    clearAllLivePreviewTimers();
}
</script>

