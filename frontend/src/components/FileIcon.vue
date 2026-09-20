<template>
    <span class="file-icon" aria-hidden="true">
        <Icon v-if="iconData" :icon="iconData" />
    </span>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import defaultFileIcon from "@iconify-icons/vscode-icons/default-file.js";
import defaultFolderIcon from "@iconify-icons/vscode-icons/default-folder.js";
import defaultFolderOpenedIcon from "@iconify-icons/vscode-icons/default-folder-opened.js";
import wordIcon from "@iconify-icons/vscode-icons/file-type-word2.js";
import excelIcon from "@iconify-icons/vscode-icons/file-type-excel2.js";
import powerpointIcon from "@iconify-icons/vscode-icons/file-type-powerpoint2.js";
import pdfIcon from "@iconify-icons/vscode-icons/file-type-pdf2.js";
// vscode-icons / vscode-icons-js 均未覆盖电子书扩展名，这里内置一枚
// Material Design「book」图标（Apache-2.0）作为电子书类文件的统一图标。
const ebookIcon = Object.freeze({
    body: '<path fill="currentColor" d="M18 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 4h5v8l-2.5-1.5L6 12V4z"/>',
    width: 24,
    height: 24,
});
const EBOOK_ICON_EXTENSIONS = new Set([
    ".epub",
    ".mobi",
    ".prc",
    ".azw",
    ".azw3",
    ".fb2",
    ".fbz",
    ".cbz",
]);
import {
    DEFAULT_FILE,
    DEFAULT_FOLDER,
    DEFAULT_FOLDER_OPENED,
    getIconForFile,
    getIconForFolder,
    getIconForOpenFolder,
} from "vscode-icons-js";
import { vscodeIconLoaders } from "../generated/vscodeIconModules";

const props = defineProps({
    type: {
        type: String,
        default: "file",
    },
    extension: {
        type: String,
        default: "",
    },
    name: {
        type: String,
        default: "",
    },
    open: {
        type: Boolean,
        default: false,
    },
});

const fileIconsByExtension = {
    ".doc": wordIcon,
    ".docm": wordIcon,
    ".docx": wordIcon,
    ".dot": wordIcon,
    ".dotm": wordIcon,
    ".dotx": wordIcon,
    ".xls": excelIcon,
    ".xlsm": excelIcon,
    ".xlsx": excelIcon,
    ".xlt": excelIcon,
    ".xltm": excelIcon,
    ".xltx": excelIcon,
    ".pot": powerpointIcon,
    ".potm": powerpointIcon,
    ".potx": powerpointIcon,
    ".pps": powerpointIcon,
    ".ppsm": powerpointIcon,
    ".ppsx": powerpointIcon,
    ".ppt": powerpointIcon,
    ".pptm": powerpointIcon,
    ".pptx": powerpointIcon,
    ".pdf": pdfIcon,
};

const loadedIcon = ref(null);
const fallbackIcon = computed(() => {
    if (props.type === "folder") {
        return props.open ? defaultFolderOpenedIcon : defaultFolderIcon;
    }

    return resolveExtensionIcon() ?? defaultFileIcon;
});
const iconData = computed(() => loadedIcon.value ?? fallbackIcon.value);

watch(
    () => [props.type, props.name, props.extension, props.open],
    async () => {
        const fallback = fallbackIcon.value;
        loadedIcon.value = fallback;

        const extensionIcon = resolveExtensionIcon();
        if (extensionIcon) {
            loadedIcon.value = extensionIcon;
            return;
        }

        const moduleName = toIconModuleName(resolveIconFileName());
        const loader = vscodeIconLoaders[moduleName];
        if (!loader) {
            return;
        }

        try {
            const module = await loader();
            loadedIcon.value = module.default ?? fallback;
        } catch {
            loadedIcon.value = fallback;
        }
    },
    { immediate: true },
);

function resolveExtensionIcon() {
    if (props.type === "folder") {
        return null;
    }

    const extension = getFileExtension();
    if (EBOOK_ICON_EXTENSIONS.has(extension)) {
        return ebookIcon;
    }
    return fileIconsByExtension[extension] ?? null;
}

function getFileExtension() {
    const extension = String(props.extension || "")
        .trim()
        .toLowerCase();
    if (extension) {
        return extension.startsWith(".") ? extension : `.${extension}`;
    }

    const name = String(props.name || "")
        .trim()
        .toLowerCase();
    const extensionStart = name.lastIndexOf(".");
    return extensionStart > 0 ? name.slice(extensionStart) : "";
}

function resolveIconFileName() {
    const name = String(props.name || "").trim();

    if (props.type === "folder") {
        if (props.open) {
            return getIconForOpenFolder(name) || DEFAULT_FOLDER_OPENED;
        }
        return getIconForFolder(name) || DEFAULT_FOLDER;
    }

    return getIconForFile(name) || DEFAULT_FILE;
}

function toIconModuleName(iconFileName) {
    return String(iconFileName || DEFAULT_FILE)
        .replace(/\.svg$/i, "")
        .replace(/_/g, "-")
        .replace(/-light-/g, "-");
}
</script>

<style scoped>
.file-icon {
    width: 18px;
    height: 18px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.file-icon :deep(svg) {
    width: 100%;
    height: 100%;
}
</style>
