<template>
    <TitleBar
        @select-folder="handleSelectFolder"
        @select-file="handleSelectFile"
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
                <aside class="workspace__sidebar">
                    <div class="pane-container">
                        <div class="pane-card__header">
                            <span>{{ folderName || "文件树" }}</span>
                        </div>
                        <FileTree
                            :nodes="treeData"
                            :active-path="activeTabPath"
                            @open-file="handleOpenFile"
                        />
                    </div>
                </aside>

                <div
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
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import TitleBar from "./components/TitleBar.vue";
import FileTree from "./components/FileTree.vue";
import PreviewTabs from "./components/PreviewTabs.vue";
import { App } from "../bindings/AnyFileViewer";

const selectedFolder = ref("");
const treeData = ref([]);
const leftPaneWidth = ref(320);
const openTabs = ref([]);
const activeTabPath = ref("");
const previewTabs = ref(null);
const globalError = ref("");
const dragOver = ref(false);
let removeResizeListeners = null;

const workspaceStyle = computed(() => ({
    gridTemplateColumns: `${leftPaneWidth.value}px 8px minmax(0, 1fr)`,
}));

const folderName = computed(() => {
    if (!selectedFolder.value) return "";
    const trimmed = selectedFolder.value.replace(/[\\/]+$/, "");
    return trimmed.split(/[\\/]/).pop() || trimmed;
});

onMounted(() => {
    window.addEventListener("keydown", handleGlobalShortcut);
});

onBeforeUnmount(() => {
    stopResize();
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
    const fileName = getPathName(filePath);
    const fileNode = {
        name: fileName,
        path: filePath,
        type: "file",
        extension: getPathExtension(fileName),
    };

    selectedFolder.value = getParentPath(filePath);
    treeData.value = [fileNode];
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
        const tree = await App.LoadFolderTree(folderPath);
        selectedFolder.value = folderPath;
        treeData.value = tree;
        openTabs.value = [];
        activeTabPath.value = "";
    } catch (error) {
        // silently ignore
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
    };

    openTabs.value = [...openTabs.value, tab];
    activeTabPath.value = tab.path;

    try {
        const content = await App.ReadFile(node.path);
        const previewType = getPreviewType(content.extension || node.extension);
        updateTab(node.path, {
            extension: content.extension || node.extension,
            previewType,
            source:
                previewType === "code"
                    ? null
                    : base64ToArrayBuffer(content.base64),
            content:
                previewType === "code"
                    ? base64ToText(content.base64, content.encoding)
                    : "",
            encoding: content.encoding || "utf-8",
            dirty: false,
            saving: false,
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

        const tree = await App.LoadFolderTree(folder);
        selectedFolder.value = folder;
        treeData.value = tree;
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

function handleCloseTab(path) {
    const currentIndex = openTabs.value.findIndex((tab) => tab.path === path);
    if (currentIndex === -1) {
        return;
    }

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
    updateTab(path, {
        status: "error",
        error: normalizeError(error, "预览失败"),
    });
}

function handleContentChange(path) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab || tab.previewType !== "code" || tab.dirty) {
        return;
    }

    updateTab(path, {
        dirty: true,
    });
}

async function handleSaveTab(path = activeTabPath.value) {
    const tab = openTabs.value.find((item) => item.path === path);
    if (!tab || tab.previewType !== "code" || tab.status !== "ready") {
        return;
    }

    updateTab(path, { saving: true });
    try {
        const content = previewTabs.value?.getCodeContent(path) ?? tab.content;
        await App.SaveFile(tab.path, content);
        updateTab(path, {
            content,
            dirty: false,
            saving: false,
            error: "",
        });
    } catch (error) {
        updateTab(path, {
            saving: false,
            status: "error",
            error: normalizeError(error, "保存文件失败"),
        });
    }
}

function handleGlobalShortcut(event) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        void handleSaveTab();
    }
}

function updateTab(path, patch) {
    openTabs.value = openTabs.value.map((tab) =>
        tab.path === path ? { ...tab, ...patch } : tab,
    );
}

function getPreviewType(extension) {
    const normalized = (extension || "").toLowerCase();
    if (normalized === ".docx") {
        return "word";
    }
    if ([".xlsx", ".xlsm", ".xltx", ".xltm", ".csv"].includes(normalized)) {
        return "excel";
    }
    if (normalized === ".xls") {
        return "unsupported";
    }
    return "code";
}

function base64ToArrayBuffer(base64) {
    const binary = window.atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
        bytes[index] = binary.charCodeAt(index);
    }
    return bytes.buffer;
}

function base64ToText(base64, encoding = "utf-8") {
    const bytes = new Uint8Array(base64ToArrayBuffer(base64));
    return new TextDecoder(normalizeTextEncoding(encoding), {
        fatal: false,
    }).decode(bytes);
}

function normalizeTextEncoding(encoding) {
    const normalized = String(encoding || "").toLowerCase();
    if (["gbk", "gb2312", "gb18030"].includes(normalized)) {
        return "gb18030";
    }
    if (normalized === "utf-16le" || normalized === "utf-16be") {
        return normalized;
    }
    return "utf-8";
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
