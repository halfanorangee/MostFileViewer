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
        @save="handleSaveTab"
        @save-as="handleSaveAsActiveTab"
    />
    <main class="page-shell">
        <section v-if="workspaceMode === 'empty'" class="hero">
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
                <aside v-if="sidebarOpen" class="workspace__sidebar">
                    <div class="pane-container">
                        <div
                            class="pane-card__header"
                            :class="{ 'pane-card__header--search': treeSearchActive }"
                        >
                            <template v-if="treeSearchActive">
                                <svg
                                    xmlns="http://www.w3.org/2000/svg"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                    stroke-width="1.5"
                                    stroke="currentColor"
                                    class="pane-card__search-icon pane-card__search-icon--input"
                                >
                                    <path
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                        d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
                                    />
                                </svg>
                                <input
                                    ref="treeSearchInput"
                                    v-model="treeSearchQuery"
                                    type="text"
                                    class="pane-card__search-input"
                                    placeholder="过滤文件…"
                                    @keydown.esc="closeTreeSearch"
                                />
                                <button
                                    type="button"
                                    class="pane-card__search-btn pane-card__search-btn--close"
                                    title="关闭搜索"
                                    @click="closeTreeSearch"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke-width="1.5"
                                        stroke="currentColor"
                                        class="pane-card__search-icon"
                                    >
                                        <path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            d="M6 18 18 6M6 6l12 12"
                                        />
                                    </svg>
                                </button>
                            </template>
                            <template v-else>
                                <span class="pane-card__title">{{ folderName || "文件树" }}</span>
                                <div class="pane-card__header-actions">
                                    <button
                                        type="button"
                                        class="pane-card__search-btn"
                                        :class="{
                                            'pane-card__search-btn--busy':
                                                treeRefreshing,
                                        }"
                                        title="刷新"
                                        aria-label="刷新文件列表"
                                        :disabled="treeRefreshing"
                                        @click="handleRefreshTree"
                                    >
                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke-width="1.5"
                                            stroke="currentColor"
                                            class="pane-card__search-icon"
                                            :class="{
                                                'pane-card__search-icon--spinning':
                                                    treeRefreshing,
                                            }"
                                        >
                                            <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
                                            />
                                        </svg>
                                    </button>
                                    <button
                                        type="button"
                                        class="pane-card__search-btn"
                                        title="搜索"
                                        @click="handleTreeSearch"
                                    >
                                        <svg
                                            xmlns="http://www.w3.org/2000/svg"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke-width="1.5"
                                            stroke="currentColor"
                                            class="pane-card__search-icon"
                                        >
                                            <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
                                            />
                                        </svg>
                                    </button>
                                </div>
                            </template>
                        </div>
                        <FileTree
                            :nodes="displayTreeData"
                            :active-path="activeTabPath"
                            :search-active="isTreeFiltering"
                            @open-file="handleOpenFile"
                            @load-folder="handleLoadFolderChildren"
                            @show-in-file-manager="handleShowInFileManager"
                        />
                    </div>
                </aside>

                <div
                    v-if="sidebarOpen"
                    class="workspace__resizer"
                    @mousedown.stop.prevent="startResize"
                ></div>

                <section class="workspace__preview">
                    <div class="pane-container pane-card--preview">
                        <PreviewTabs
                            ref="previewTabs"
                            :tabs="openTabs"
                            :active-tab-path="activeTabPath"
                            :auto-save-enabled="autoSaveEnabled"
                            :audio-queue-mode="audioQueueMode"
                            :audio-has-prev="audioHasPrev"
                            :audio-has-next="audioHasNext"
                            @change-tab="handleChangeTab"
                            @close-tab="handleCloseTab"
                            @preview-error="handlePreviewError"
                            @preview-rendered="handlePreviewRendered"
                            @content-change="handleContentChange"
                            @encoding-change="handleEncodingChange"
                            @save-tab="handleSaveTab"
                            @save-as-tab="handleSaveAsTab"
                            @open-in-new-tab="handleOpenInNewTab"
                            @reorder-tab="handleReorderTab"
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
            </div>
        </section>
    </main>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { Events, Window } from "@wailsio/runtime";
import TitleBar from "./components/TitleBar.vue";
import FileTree from "./components/FileTree.vue";
import PreviewTabs from "./components/PreviewTabs.vue";
import { App } from "../bindings/MostFileViewer";
import { useTheme } from "./composables/useTheme";
import { getMediaType } from "./composables/useFileTypes";
import { mediaErrorMessage } from "./composables/useMediaPlayer";
import { useAutoSave } from "./composables/useAutoSave";
import { useWorkspaceSession } from "./composables/useWorkspaceSession";
import { usePreviewLoader } from "./composables/usePreviewLoader";
import { useTabPersistence } from "./composables/useTabPersistence";
import { useFileSystemSync } from "./composables/useFileSystemSync";
import { useTabLifecycle } from "./composables/useTabLifecycle";

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
const previewTabs = ref(null);
const globalError = ref("");
const treeSearchActive = ref(false);
const treeSearchQuery = ref("");
const treeSearchInput = ref(null);
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
    previewTabs,
    autoSaveEnabled,
    clearAutoSave: clearAutoSaveDebounceTimer,
    scheduleAutoSave,
    updateTab,
    registerOpenPath,
    schedulePersist: schedulePersistWorkspaceSession,
    getPathName,
    getPathExtension,
    normalizeError,
});
saveTabForAutoSave = handleSaveTab;

const {
    handleReorderTab,
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
});
saveTabForLifecycle = handleSaveTab;

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
});

// 未命名（空白可编辑）tab 的自增计数器，保证多个未命名 tab 的 path 与 name 互不重复。
let untitledCounter = 0;

const workspaceStyle = computed(() => ({
    gridTemplateColumns: sidebarOpen.value
        ? `${leftPaneWidth.value}px 8px minmax(0, 1fr)`
        : "minmax(0, 1fr)",
}));

const folderName = computed(() => {
    if (!selectedFolder.value) return "";
    const trimmed = selectedFolder.value.replace(/[\\/]+$/, "");
    return trimmed.split(/[\\/]/).pop() || trimmed;
});

// 是否处于有效的搜索过滤状态（激活且有非空关键字）
const isTreeFiltering = computed(
    () => treeSearchActive.value && treeSearchQuery.value.trim() !== "",
);

// 根据搜索关键字过滤文件树：保留名称匹配的文件，以及包含匹配项的文件夹（并强制展开）。
// 注意：文件树为懒加载，未展开的文件夹其子节点尚未加载，过滤仅作用于已加载的节点。
const displayTreeData = computed(() => {
    if (!isTreeFiltering.value) {
        return treeData.value;
    }
    const keyword = treeSearchQuery.value.trim().toLowerCase();
    return filterTreeNodes(treeData.value, keyword);
});

function filterTreeNodes(nodes, keyword) {
    const result = [];
    for (const node of nodes) {
        const nameMatched = (node.name || "").toLowerCase().includes(keyword);
        if (node.type === "folder") {
            const filteredChildren = filterTreeNodes(
                node.children || [],
                keyword,
            );
            if (nameMatched || filteredChildren.length > 0) {
                result.push({
                    ...node,
                    children: filteredChildren,
                    // 过滤时强制展开以显示命中的后代节点
                    forceExpanded: filteredChildren.length > 0,
                });
            }
        } else if (nameMatched) {
            result.push({ ...node });
        }
    }
    return result;
}

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

async function openFileNode(node) {
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

function handleTreeSearch() {
    treeSearchActive.value = true;
    nextTick(() => {
        treeSearchInput.value?.focus();
    });
}

function closeTreeSearch() {
    treeSearchActive.value = false;
    treeSearchQuery.value = "";
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
            previewTabs.value?.getCodeContent?.(path) ?? tab.content ?? "";
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
        const latest = previewTabs.value?.getCodeContent?.(sourcePath);
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

<style scoped>
.pane-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background-color: var(--bg-surface);
}

.pane-card__header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 2px 12px;
    font-size: 16px;
    border-bottom: 1px solid var(--border-subtle);
    font-weight: 600;
}

.pane-card__title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.pane-card__header-actions {
    display: flex;
    align-items: center;
    flex-shrink: 0;
}

.pane-card__search-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    border: none;
    background: transparent;
    border-radius: 4px;
    color: var(--text-secondary, inherit);
    cursor: pointer;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease, background-color 0.15s ease;
}

.workspace__sidebar:hover .pane-card__search-btn,
.pane-card__search-btn--busy {
    opacity: 1;
    visibility: visible;
}

.pane-card__search-btn:hover {
    background-color: var(--bg-hover, rgba(0, 0, 0, 0.08));
}

.pane-card__search-icon {
    width: 16px;
    height: 16px;
}

.pane-card__header--search {
    gap: 6px;
}

.pane-card__search-icon--input {
    flex-shrink: 0;
    color: var(--text-secondary, inherit);
}

.pane-card__search-input {
    flex: 1;
    min-width: 0;
    border: none;
    outline: none;
    background: transparent;
    font-size: 14px;
    font-weight: 400;
    color: var(--text-primary, inherit);
}

.pane-card__search-input::placeholder {
    color: var(--text-muted, #999);
}

.pane-card__search-btn:disabled {
    cursor: default;
}

.pane-card__search-btn:disabled:hover {
    background-color: transparent;
}

.pane-card__search-icon--spinning {
    animation: pane-card-icon-spin 0.8s linear infinite;
}

@keyframes pane-card-icon-spin {
    to {
        transform: rotate(360deg);
    }
}

.pane-card__search-btn--close {
    opacity: 1;
    visibility: visible;
    flex-shrink: 0;
}
</style>
