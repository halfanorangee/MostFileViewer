<template>
    <TitleBar
        :show-sidebar-toggle="isActualFolderPreview"
        :sidebar-open="sidebarOpen"
        @select-folder="handleSelectFolder"
        @select-file="handleSelectFile"
        @toggle-sidebar="toggleSidebar"
        @new-window="handleNewWindow"
    />
    <main class="page-shell">
        <section v-if="!selectedFolder" class="hero">
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
                        <div class="pane-card__header">
                            <span>{{ folderName || "文件树" }}</span>
                        </div>
                        <FileTree
                            :nodes="treeData"
                            :active-path="activeTabPath"
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
                            @change-tab="handleChangeTab"
                            @close-tab="handleCloseTab"
                            @preview-error="handlePreviewError"
                            @preview-rendered="handlePreviewRendered"
                            @content-change="handleContentChange"
                            @encoding-change="handleEncodingChange"
                            @save-tab="handleSaveTab"
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

const selectedFolder = ref("");
const treeData = ref([]);
const leftPaneWidth = ref(320);
const sidebarOpen = ref(true);
const openTabs = ref([]);
const activeTabPath = ref("");
const previewTabs = ref(null);
const globalError = ref("");
let removeResizeListeners = null;
let removeFilesDroppedListener = null;
let removeThemeChangeListener = null;
let restoringSession = false;
let persistSessionTimer = null;

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
const autoSaveDebounceTimers = new Map();
const encodingChangeRequests = new Map();
let autoSaveIntervalTimer = null;
const AUTO_SAVE_DEBOUNCE_DELAY = 2500; // 2.5秒防抖
const AUTO_SAVE_INTERVAL_DELAY = 60000; // 60秒定时保存

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

// 判断是否是真正的文件夹预览（而不是单个文件预览）
const isActualFolderPreview = computed(() => {
    if (!selectedFolder.value) return false;
    // 如果树数据只有一个文件节点，说明是单个文件预览
    if (treeData.value.length === 1 && treeData.value[0].type === "file") {
        return false;
    }
    // 否则是真正的文件夹预览
    return true;
});

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

    void restoreWorkspaceSession();
});

onBeforeUnmount(() => {
    stopResize();
    stopAutoSave();
    if (persistSessionTimer) {
        clearTimeout(persistSessionTimer);
        persistSessionTimer = null;
    }
    window.removeEventListener("keydown", handleGlobalShortcut);
    removeFilesDroppedListener?.();
    removeThemeChangeListener?.();
});

async function restoreWorkspaceSession() {
    restoringSession = true;
    try {
        const session = await App.ConsumeRestoreSession();
        if (!session?.hasSession) {
            restoringSession = false;
            return;
        }

        if (Number.isFinite(session.leftPaneWidth) && session.leftPaneWidth > 0) {
            leftPaneWidth.value = session.leftPaneWidth;
        }

        if (session.mode === "folder" && session.rootPath) {
            const tree = await App.LoadFolderTree(session.rootPath);
            selectedFolder.value = session.rootPath;
            treeData.value = tree;
            sidebarOpen.value = session.sidebarOpen !== false;
            openTabs.value = [];
            activeTabPath.value = "";
            await registerOpenPath(session.rootPath);
        } else if (session.openTabs?.length) {
            const firstPath = session.openTabs[0].path;
            selectedFolder.value = getParentPath(firstPath);
            treeData.value = [
                {
                    name: getPathName(firstPath),
                    path: firstPath,
                    type: "file",
                    extension: getPathExtension(firstPath),
                },
            ];
            sidebarOpen.value = false;
            openTabs.value = [];
            activeTabPath.value = "";
        }

        for (const tab of session.openTabs || []) {
            if (!tab?.path) {
                continue;
            }
            await openFileNode({
                name: getPathName(tab.path),
                path: tab.path,
                type: "file",
                extension: getPathExtension(tab.path),
            });
        }

        if (session.activePath && openTabs.value.some((tab) => tab.path === session.activePath)) {
            activeTabPath.value = session.activePath;
        }

        restoringSession = false;
        if (!selectedFolder.value && openTabs.value.length === 0) {
            await clearWorkspaceSession();
            return;
        }
        await persistWorkspaceSessionNow();
    } catch (error) {
        restoringSession = false;
        await clearWorkspaceSession();
    } finally {
        restoringSession = false;
    }
}

function schedulePersistWorkspaceSession() {
    if (restoringSession) {
        return;
    }
    if (persistSessionTimer) {
        clearTimeout(persistSessionTimer);
    }
    persistSessionTimer = setTimeout(() => {
        persistSessionTimer = null;
        void persistWorkspaceSessionNow();
    }, 250);
}

async function persistWorkspaceSessionNow() {
    if (restoringSession) {
        return;
    }

    const tabs = openTabs.value
        .filter((tab) => tab?.path && tab.status !== "error")
        .map((tab) => ({ path: tab.path }));

    if (!selectedFolder.value && tabs.length === 0) {
        await clearWorkspaceSession();
        return;
    }

    const session = {
        mode: isActualFolderPreview.value ? "folder" : "file",
        rootPath: selectedFolder.value || "",
        openTabs: tabs,
        activePath: activeTabPath.value || "",
        sidebarOpen: sidebarOpen.value,
        leftPaneWidth: leftPaneWidth.value,
    };

    try {
        await App.SaveWindowSession(session);
    } catch (error) {
        // Session persistence is best-effort.
    }
}

async function clearWorkspaceSession() {
    try {
        await App.ClearWindowSession();
    } catch (error) {
        // Session persistence is best-effort.
    }
}

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
        if (selectedFolder.value) {
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
    if (!(await saveDirtyTabs())) {
        return;
    }

    // 注销旧工作区的已打开路径
    await unregisterAllOpenPaths();

    const fileName = getPathName(filePath);
    const fileNode = {
        name: fileName,
        path: filePath,
        type: "file",
        extension: getPathExtension(fileName),
    };

    selectedFolder.value = getParentPath(filePath);
    treeData.value = [fileNode];
    sidebarOpen.value = false; // 选择单个文件时关闭侧边栏
    clearAllAutoSaveDebounceTimers();
    releaseAllTabPayloads();
    await nextTick();
    openTabs.value = [];
    activeTabPath.value = "";
    await openFileNode(fileNode);
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

function findNodeByPath(nodes, targetPath) {
    for (const node of nodes) {
        if (node.path === targetPath) {
            return node;
        }
        if (node.children && node.children.length > 0) {
            const found = findNodeByPath(node.children, targetPath);
            if (found) {
                return found;
            }
        }
    }
    return null;
}

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
        if (selectedFolder.value) {
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

        // 注销旧工作区的已打开路径
        await unregisterAllOpenPaths();

        const tree = await App.LoadFolderTree(folderPath);
        selectedFolder.value = folderPath;
        treeData.value = tree;
        sidebarOpen.value = true; // 选择文件夹时显示侧边栏
        clearAllAutoSaveDebounceTimers();
        releaseAllTabPayloads();
        await nextTick();
        openTabs.value = [];
        activeTabPath.value = "";

        // 注册文件夹路径
        await registerOpenPath(folderPath);
        schedulePersistWorkspaceSession();
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

async function openFileNode(node) {
    const tab = {
        path: node.path,
        name: node.name,
        extension: node.extension,
        status: "loading",
        previewType: getPreviewType(node.extension),
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
    };

    openTabs.value = [...openTabs.value, tab];
    activeTabPath.value = tab.path;

    try {
        await registerOpenPath(node.path);
        const content = await App.ReadFile(node.path);
        const previewType = getPreviewType(content.extension || node.extension);
        const isCodePreview = previewType === "code";
        const source = isCodePreview
            ? null
            : await loadBinarySource(content, previewType);
        updateTab(node.path, {
            extension: content.extension || node.extension,
            previewType,
            source,
            content: isCodePreview ? content.content || "" : "",
            encoding: content.encoding || "utf-8",
            dirty: false,
            saving: false,
            saveError: "",
            encodingLoading: false,
            contentVersion: isCodePreview ? (tab.contentVersion ?? 0) + 1 : 0,
            changeVersion: 0,
            savedVersion: 0,
            status: "ready",
        });

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

        // 注销旧工作区的已打开路径
        await unregisterAllOpenPaths();

        const tree = await App.LoadFolderTree(folder);
        selectedFolder.value = folder;
        treeData.value = tree;
        sidebarOpen.value = true; // 选择文件夹时显示侧边栏
        clearAllAutoSaveDebounceTimers();
        releaseAllTabPayloads();
        await nextTick();
        openTabs.value = [];
        activeTabPath.value = "";

        // 注册文件夹路径
        await registerOpenPath(folder);
        schedulePersistWorkspaceSession();
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

async function handleCloseTab(path) {
    const currentIndex = openTabs.value.findIndex((tab) => tab.path === path);
    if (currentIndex === -1) {
        return;
    }

    let currentTab = openTabs.value.find((tab) => tab.path === path);
    if (currentTab?.dirty) {
        await handleSaveTab(path);
        currentTab = openTabs.value.find((tab) => tab.path === path);
    }

    if (currentTab?.dirty || currentTab?.saving) {
        return;
    }

    clearAutoSaveDebounceTimer(path);
    encodingChangeRequests.delete(path);
    releaseTabPayload(path);
    await nextTick();

    const nextTabs = openTabs.value.filter((tab) => tab.path !== path);
    openTabs.value = nextTabs;

    // 注销已关闭 tab 的文件路径
    await unregisterOpenPath(path);

    // 单文件模式下关闭最后一个 tab 后返回首页
    if (nextTabs.length === 0 && !isActualFolderPreview.value) {
        selectedFolder.value = "";
        treeData.value = [];
        activeTabPath.value = "";
        void clearWorkspaceSession();
        return;
    }

    if (activeTabPath.value !== path) {
        schedulePersistWorkspaceSession();
        return;
    }

    const nextActive =
        nextTabs[currentIndex] || nextTabs[currentIndex - 1] || null;
    activeTabPath.value = nextActive ? nextActive.path : "";
    schedulePersistWorkspaceSession();
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

    updateTab(path, {
        dirty: true,
        saveError: "",
        changeVersion: (tab.changeVersion ?? 0) + 1,
    });

    scheduleAutoSave(path);
}

async function handleEncodingChange(path, encoding) {
    let tab = openTabs.value.find((item) => item.path === path);
    if (!tab || tab.previewType !== "code" || tab.status !== "ready") {
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

async function handleSaveTab(path = activeTabPath.value) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (
        !tab ||
        tab.previewType !== "code" ||
        tab.status !== "ready" ||
        tab.saving ||
        tab.encodingLoading
    ) {
        return;
    }

    clearAutoSaveDebounceTimer(path);

    const saveVersion = tab.changeVersion ?? 0;
    const content = previewTabs.value?.getCodeContent(path) ?? tab.content;

    updateTab(path, { saving: true, saveError: "" });
    try {
        await App.SaveFile(tab.path, content, tab.encoding || "utf-8");

        const currentTab = openTabs.value.find((item) => item.path === path);
        if (!currentTab) {
            return;
        }

        const hasNewerChanges = (currentTab.changeVersion ?? 0) > saveVersion;
        updateTab(path, {
            ...(hasNewerChanges ? {} : { content }),
            dirty: hasNewerChanges,
            saving: false,
            savedVersion: hasNewerChanges
                ? (currentTab.savedVersion ?? 0)
                : saveVersion,
            error: "",
            saveError: "",
        });

        if (hasNewerChanges) {
            scheduleAutoSave(path);
        }
    } catch (error) {
        updateTab(path, {
            saving: false,
            dirty: true,
            saveError: normalizeError(error, "保存文件失败"),
        });
    }
}

async function saveDirtyTabs() {
    const dirtyPaths = openTabs.value
        .filter(
            (tab) =>
                tab.dirty &&
                tab.status === "ready" &&
                tab.previewType === "code",
        )
        .map((tab) => tab.path);

    for (const path of dirtyPaths) {
        await handleSaveTab(path);
    }

    return dirtyPaths.every((path) => {
        const tab = openTabs.value.find((item) => item.path === path);
        return !tab || (!tab.dirty && !tab.saving);
    });
}

function handleGlobalShortcut(event) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
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

function releaseTabPayload(path) {
    encodingChangeRequests.delete(path);
    updateTab(path, {
        source: null,
        content: "",
        error: "",
        encodingLoading: false,
    });
}

function releaseAllTabPayloads() {
    encodingChangeRequests.clear();
    if (!openTabs.value.length) {
        return;
    }

    openTabs.value = openTabs.value.map((tab) => ({
        ...tab,
        source: null,
        content: "",
        error: "",
        encodingLoading: false,
    }));
}

function updateTab(path, patch) {
    openTabs.value = openTabs.value.map((tab) =>
        tab.path === path ? { ...tab, ...patch } : tab,
    );
}

function updateTreeNode(nodes, path, patch) {
    return nodes.map((node) => {
        if (node.path === path) {
            return { ...node, ...patch };
        }
        if (!node.children?.length) {
            return node;
        }
        return {
            ...node,
            children: updateTreeNode(node.children, path, patch),
        };
    });
}

function getPreviewType(extension) {
    const normalized = (extension || "").toLowerCase();
    if (normalized === ".docx") {
        return "word";
    }
    if (normalized === ".csv") {
        return "csv";
    }
    if ([".xlsx", ".xlsm", ".xltx", ".xltm"].includes(normalized)) {
        return "excel";
    }
    if ([".pptx", ".pptm", ".ppsx", ".ppsm"].includes(normalized)) {
        return "ppt";
    }
    if (normalized === ".pdf") {
        return "pdf";
    }
    if (isImageExtension(normalized)) {
        return "image";
    }
    if (normalized === ".xls") {
        return "unsupported";
    }
    return "code";
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

async function loadBinarySource(content, previewType) {
    if (!content) {
        return null;
    }
    if (["word", "excel", "ppt", "image", "pdf"].includes(previewType)) {
        return readFileInChunks(content.path, Number(content.size || 0));
    }
    return base64ToArrayBuffer(content.base64);
}

async function readFileInChunks(path, totalSize) {
    if (!path || !Number.isFinite(totalSize) || totalSize <= 0) {
        return new ArrayBuffer(0);
    }

    const chunkSize = 2 * 1024 * 1024;
    const bytes = new Uint8Array(totalSize);
    let offset = 0;

    while (offset < totalSize) {
        const nextSize = Math.min(chunkSize, totalSize - offset);
        const chunk = await App.ReadFileChunk(path, offset, nextSize);
        const actualSize = Number(chunk?.size || 0);
        if (actualSize <= 0) {
            break;
        }

        writeBase64Chunk(bytes, offset, chunk.base64, actualSize);
        offset += actualSize;
    }

    return bytes.buffer;
}

function writeBase64Chunk(target, offset, base64, expectedSize) {
    const binary = window.atob(base64 || "");
    const size = Math.min(binary.length, expectedSize, target.length - offset);
    for (let index = 0; index < size; index += 1) {
        target[offset + index] = binary.charCodeAt(index);
    }
}

function base64ToArrayBuffer(base64) {
    if (!base64) {
        return null;
    }
    const binary = window.atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
        bytes[index] = binary.charCodeAt(index);
    }
    return bytes.buffer;
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

function startAutoSaveInterval() {
    // 清理旧的定时器
    if (autoSaveIntervalTimer) {
        clearInterval(autoSaveIntervalTimer);
    }

    // 启动定时保存：每60秒检查所有脏tabs并保存
    autoSaveIntervalTimer = setInterval(() => {
        openTabs.value.forEach((tab) => {
            // 只保存真正修改过的tabs（优化1：智能节流）
            if (
                tab.dirty &&
                tab.status === "ready" &&
                tab.previewType === "code" &&
                !tab.saving
            ) {
                void handleSaveTab(tab.path);
            }
        });
    }, AUTO_SAVE_INTERVAL_DELAY);
}

function scheduleAutoSave(path) {
    clearAutoSaveDebounceTimer(path);
    autoSaveDebounceTimers.set(
        path,
        setTimeout(() => {
            autoSaveDebounceTimers.delete(path);
            void handleSaveTab(path);
        }, AUTO_SAVE_DEBOUNCE_DELAY),
    );
}

function clearAutoSaveDebounceTimer(path) {
    const timer = autoSaveDebounceTimers.get(path);
    if (!timer) {
        return;
    }

    clearTimeout(timer);
    autoSaveDebounceTimers.delete(path);
}

function clearAllAutoSaveDebounceTimers() {
    autoSaveDebounceTimers.forEach((timer) => clearTimeout(timer));
    autoSaveDebounceTimers.clear();
}

function stopAutoSave() {
    // 清理防抖定时器
    clearAllAutoSaveDebounceTimers();

    // 清理定时保存定时器
    if (autoSaveIntervalTimer) {
        clearInterval(autoSaveIntervalTimer);
        autoSaveIntervalTimer = null;
    }
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
</style>
