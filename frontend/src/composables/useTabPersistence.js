import { App } from "../../bindings/MostFileViewer";
import {
    detectSyntaxKey,
    inferExtensionFromSyntax,
} from "./useFileTypes";

export function useTabPersistence({
    tabs,
    activePath,
    selectedFolder,
    previewTabs,
    autoSaveEnabled,
    clearAutoSave,
    scheduleAutoSave,
    updateTab,
    registerOpenPath,
    schedulePersist,
    getPathName,
    getPathExtension,
    normalizeError,
}) {
    function getEditorContent(path, fallback = "") {
        return previewTabs.value?.getCodeContent(path) ?? fallback;
    }

    function sanitizeFileName(name) {
        return String(name || "").replace(/[\\/:*?"<>|]/g, "_").trim();
    }

    function suggestedFileName(tab) {
        if (tab.virtual) {
            const ext = inferExtensionFromSyntax(tab.syntax);
            const base = sanitizeFileName(tab.name) || "未命名";
            return `${base}${ext}`;
        }
        const ext = tab.extension || ".txt";
        const base = sanitizeFileName(tab.name) || "副本";
        return `${base} (副本)${ext}`;
    }

    function defaultSaveDirectory(tab) {
        if (tab.virtual) {
            return selectedFolder.value || "";
        }
        if (!tab.path) {
            return "";
        }
        return tab.path.replace(/[\\/][^\\/]*$/, "");
    }

    async function saveAs(path, { alwaysAsCopy = false } = {}) {
        const tab = tabs.value.find((item) => item.path === path);
        if (
            !tab ||
            tab.previewType !== "code" ||
            tab.status !== "ready" ||
            tab.saving ||
            tab.encodingLoading ||
            (!tab.virtual && tab.previewOnly)
        ) {
            return;
        }

        clearAutoSave(path);

        const content = getEditorContent(path, tab.content || "");
        const encoding = tab.encoding || "utf-8";
        updateTab(path, { saving: true, saveError: "" });

        try {
            const newPath = await App.SaveFileAs(
                suggestedFileName(tab),
                content,
                encoding,
                defaultSaveDirectory(tab),
            );
            if (!newPath) {
                const current = tabs.value.find((item) => item.path === path);
                updateTab(path, {
                    saving: false,
                    saveError: "",
                    status: "ready",
                    error: "",
                    dirty: current?.dirty === true,
                });
                return;
            }

            if (!tab.virtual && alwaysAsCopy) {
                updateTab(path, {
                    saving: false,
                    saveError: `已另存为副本：${newPath}`,
                });
                return;
            }

            const saveVersion = tab.changeVersion ?? 0;
            const newExt =
                getPathExtension(newPath) || inferExtensionFromSyntax(tab.syntax);
            const newName = getPathName(newPath);
            const latestContent = getEditorContent(path, content);
            const nextTabs = tabs.value.map((item) => {
                if (item.path !== path) return item;
                return {
                    ...item,
                    path: newPath,
                    name: newName,
                    extension: newExt,
                    syntax: detectSyntaxKey(newExt, newName),
                    virtual: false,
                    previewOnly: false,
                    dirty: false,
                    saving: false,
                    saveError: "",
                    savedVersion: saveVersion,
                    contentVersion: (item.contentVersion ?? 0) + 1,
                    content: latestContent,
                };
            });
            tabs.value = nextTabs;
            activePath.value = newPath;
            await registerOpenPath(newPath);
            schedulePersist();
        } catch (error) {
            updateTab(path, {
                saving: false,
                dirty: true,
                saveError: normalizeError(error, "保存文件失败"),
            });
        }
    }

    async function save(path = activePath.value) {
        const tab = tabs.value.find((item) => item.path === path);
        if (
            !tab ||
            tab.previewType !== "code" ||
            tab.status !== "ready" ||
            tab.saving ||
            tab.encodingLoading
        ) {
            return;
        }
        if (tab.virtual) {
            return saveAs(path);
        }

        clearAutoSave(path);
        const saveVersion = tab.changeVersion ?? 0;
        const content = getEditorContent(path, tab.content);
        updateTab(path, { saving: true, saveError: "" });

        try {
            await App.SaveFile(tab.path, content, tab.encoding || "utf-8");
            const currentTab = tabs.value.find((item) => item.path === path);
            if (!currentTab) return;

            const hasNewerChanges =
                (currentTab.changeVersion ?? 0) > saveVersion;
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

            if (hasNewerChanges && autoSaveEnabled.value) {
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

    async function saveDirty() {
        const dirtyPaths = tabs.value
            .filter(
                (tab) =>
                    tab.dirty &&
                    tab.status === "ready" &&
                    tab.previewType === "code",
            )
            .map((tab) => tab.path);

        for (const path of dirtyPaths) {
            await save(path);
        }

        return dirtyPaths.every((path) => {
            const tab = tabs.value.find((item) => item.path === path);
            return !tab || (!tab.dirty && !tab.saving);
        });
    }

    return {
        save,
        saveAs,
        saveDirty,
        suggestedFileName,
        defaultSaveDirectory,
    };
}
