<template>
    <TitleBar
        :show-sidebar-toggle="Boolean(selectedFolder)"
        :sidebar-open="sidebarOpen"
        @select-folder="handleSelectFolder"
        @select-file="handleSelectFile"
        @toggle-sidebar="toggleSidebar"
    />
    <main class="page-shell">
        <section v-if="!selectedFolder" class="hero">
            <div class="hero__panel">
                <div
                    class="hero__dropzone"
                    :class="{ 'hero__dropzone--active': dragOver }"
                    @dragover.prevent="handleDragOver"
                    @dragleave="handleDragLeave"
                    @drop.prevent="handleDrop"
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
                    <p class="hero__dropzone-text">拖动文件到此处打开</p>
                </div>
                <div class="hero__actions">
                    <lay-button type="primary" @click="handleSelectFile">
                        选择文件
                    </lay-button>
                    <lay-button @click="handleSelectFolder">
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
import TitleBar from "./components/TitleBar.vue";
import FileTree from "./components/FileTree.vue";
import PreviewTabs from "./components/PreviewTabs.vue";
import { App } from "../bindings/AnyFileViewer";

const selectedFolder = ref("");
const treeData = ref([]);
const leftPaneWidth = ref(320);
const sidebarOpen = ref(true);
const openTabs = ref([]);
const activeTabPath = ref("");
const previewTabs = ref(null);
const globalError = ref("");
const dragOver = ref(false);
let removeResizeListeners = null;

// 自动保存相关
const autoSaveDebounceTimers = new Map();
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

onMounted(() => {
    window.addEventListener("keydown", handleGlobalShortcut);
    startAutoSaveInterval();
});

onBeforeUnmount(() => {
    stopResize();
    stopAutoSave();
    window.removeEventListener("keydown", handleGlobalShortcut);
});

async function handleSelectFile() {
    globalError.value = "";

    try {
        const filePath = await App.SelectFile();
        if (!filePath) {
            return;
        }
        await openFileInWorkspace(filePath);
    } catch (error) {
        // silently ignore
    }
}

async function openFileInWorkspace(filePath) {
    if (!(await saveDirtyTabs())) {
        return;
    }

    const fileName = getPathName(filePath);
    const fileNode = {
        name: fileName,
        path: filePath,
        type: "file",
        extension: getPathExtension(fileName),
    };

    selectedFolder.value = getParentPath(filePath);
    treeData.value = [fileNode];
    clearAllAutoSaveDebounceTimers();
    releaseAllTabPayloads();
    await nextTick();
    openTabs.value = [];
    activeTabPath.value = "";
    await openFileNode(fileNode);
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

function handleDragOver(event) {
    dragOver.value = true;
}

function handleDragLeave() {
    dragOver.value = false;
}

async function handleDrop(event) {
    dragOver.value = false;
    globalError.value = "";

    const items = event.dataTransfer.items;
    if (!items || items.length === 0) {
        return;
    }

    // Use the first item
    const entry = items[0].webkitGetAsEntry?.();
    if (!entry) {
        // Fallback: try files
        const file = event.dataTransfer.files[0];
        if (file && file.path) {
            await openFileInWorkspace(file.path);
        }
        return;
    }

    if (entry.isDirectory) {
        // Resolve the directory path
        const dirPath = await resolveEntryPath(entry);
        if (dirPath) {
            await handleOpenFolder(dirPath);
        }
    } else {
        const filePath = await resolveEntryPath(entry);
        if (filePath) {
            await openFileInWorkspace(filePath);
        }
    }
}

function resolveEntryPath(entry) {
    return new Promise((resolve) => {
        entry.file(
            (file) => resolve(file.path || null),
            () => resolve(null),
        );
    });
}

async function handleOpenFolder(folderPath) {
    try {
        if (!(await saveDirtyTabs())) {
            return;
        }

        const tree = await App.LoadFolderTree(folderPath);
        selectedFolder.value = folderPath;
        treeData.value = tree;
        clearAllAutoSaveDebounceTimers();
        releaseAllTabPayloads();
        await nextTick();
        openTabs.value = [];
        activeTabPath.value = "";
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
        changeVersion: 0,
        savedVersion: 0,
    };

    openTabs.value = [...openTabs.value, tab];
    activeTabPath.value = tab.path;

    try {
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
            changeVersion: 0,
            savedVersion: 0,
            status: "ready",
        });
    } catch (error) {
        updateTab(node.path, {
            status: "error",
            error: normalizeError(error, "读取文件失败"),
        });
    }
}

async function handleSelectFolder() {
    globalError.value = "";

    try {
        const folder = await App.SelectFolder();
        if (!folder) {
            return;
        }

        if (!(await saveDirtyTabs())) {
            return;
        }

        const tree = await App.LoadFolderTree(folder);
        selectedFolder.value = folder;
        treeData.value = tree;
        clearAllAutoSaveDebounceTimers();
        releaseAllTabPayloads();
        await nextTick();
        openTabs.value = [];
        activeTabPath.value = "";
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

function handleChangeTab(path) {
    activeTabPath.value = path;
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
    releaseTabPayload(path);
    await nextTick();

    const nextTabs = openTabs.value.filter((tab) => tab.path !== path);
    openTabs.value = nextTabs;

    if (activeTabPath.value !== path) {
        return;
    }

    const nextActive =
        nextTabs[currentIndex] || nextTabs[currentIndex - 1] || null;
    activeTabPath.value = nextActive ? nextActive.path : "";
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

async function handleSaveTab(path = activeTabPath.value) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (
        !tab ||
        tab.previewType !== "code" ||
        tab.status !== "ready" ||
        tab.saving
    ) {
        return;
    }

    clearAutoSaveDebounceTimer(path);

    const saveVersion = tab.changeVersion ?? 0;
    const content = previewTabs.value?.getCodeContent(path) ?? tab.content;

    updateTab(path, { saving: true, saveError: "" });
    try {
        await App.SaveFile(tab.path, content);

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
    sidebarOpen.value = !sidebarOpen.value;
    if (!sidebarOpen.value) {
        stopResize();
    }
}

function releaseTabPayload(path) {
    updateTab(path, {
        source: null,
        content: "",
        error: "",
    });
}

function releaseAllTabPayloads() {
    if (!openTabs.value.length) {
        return;
    }

    openTabs.value = openTabs.value.map((tab) => ({
        ...tab,
        source: null,
        content: "",
        error: "",
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
    if (["word", "excel", "ppt", "image"].includes(previewType)) {
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
    background-color: #fff;
}
</style>
