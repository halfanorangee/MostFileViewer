import { nextTick } from "vue";

export function useTabLifecycle({
    tabs,
    activePath,
    selectedFolder,
    treeData,
    workspaceMode,
    untitledCounter,
    saveTab,
    clearAutoSave,
    encodingChangeRequests,
    revokeTabMedia,
    updateTab,
    unregisterOpenPath,
    clearLivePreviewTimer,
    schedulePersist,
    clearWorkspaceSession,
}) {
    function handleReorder({ fromPath, toPath, after }) {
        const fromIndex = tabs.value.findIndex((tab) => tab.path === fromPath);
        let toIndex = tabs.value.findIndex((tab) => tab.path === toPath);
        if (fromIndex === -1 || toIndex === -1 || fromIndex === toIndex) {
            return;
        }

        const nextTabs = [...tabs.value];
        const [moved] = nextTabs.splice(fromIndex, 1);
        toIndex = nextTabs.findIndex((tab) => tab.path === toPath);
        nextTabs.splice(after ? toIndex + 1 : toIndex, 0, moved);
        tabs.value = nextTabs;
        schedulePersist();
    }

    function handleOpenInNewTab(sourcePath, payload) {
        if (!payload) return;

        const previewPath = `preview://${sourcePath}`;
        const existingTab = tabs.value.find((tab) => tab.path === previewPath);
        if (existingTab) {
            updateTab(previewPath, {
                content: payload.content ?? "",
                extension: payload.extension || existingTab.extension,
                syntax: payload.syntax || existingTab.syntax,
            });
            activePath.value = previewPath;
            return;
        }

        tabs.value = [
            ...tabs.value,
            {
                path: previewPath,
                name: payload.name ? `${payload.name} (预览)` : "预览",
                extension: payload.extension || "",
                syntax: payload.syntax || "",
                status: "ready",
                previewType: "preview",
                previewOnly: true,
                source: null,
                content: payload.content ?? "",
                encoding: "utf-8",
                dirty: false,
                saving: false,
                saveError: "",
                encodingLoading: false,
                contentVersion: 0,
                changeVersion: 0,
                savedVersion: 0,
            },
        ];
        activePath.value = previewPath;
    }

    function handleNewTab() {
        const id = untitledCounter();
        tabs.value = [
            ...tabs.value,
            {
                path: `untitled://${id}`,
                name: id > 1 ? `未命名 ${id}` : "未命名",
                extension: ".txt",
                status: "ready",
                previewType: "code",
                virtual: true,
                previewOnly: true,
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
            },
        ];
        activePath.value = `untitled://${id}`;
        schedulePersist();
    }

    async function handleCloseTab(path) {
        const currentIndex = tabs.value.findIndex((tab) => tab.path === path);
        if (currentIndex === -1) return;

        let currentTab = tabs.value.find((tab) => tab.path === path);
        const isPreviewOnly = currentTab?.previewOnly === true;
        const isVirtual = currentTab?.virtual === true;
        if (currentTab?.dirty && !isVirtual) {
            await saveTab(path);
            currentTab = tabs.value.find((tab) => tab.path === path);
        }

        if (!isVirtual && (currentTab?.dirty || currentTab?.saving)) {
            return;
        }

        clearAutoSave(path);
        encodingChangeRequests.delete(path);
        await revokeTabMedia(currentTab);
        updateTab(path, {
            source: null,
            content: "",
            error: "",
            encodingLoading: false,
        });
        clearLivePreviewTimer(
            isPreviewOnly ? path.replace(/^preview:\/\//, "") : path,
        );
        await nextTick();

        const nextTabs = tabs.value.filter((tab) => tab.path !== path);
        tabs.value = nextTabs;
        if (!isPreviewOnly) {
            await unregisterOpenPath(path);
        }

        if (nextTabs.length === 0 && workspaceMode.value !== "folder") {
            workspaceMode.value = "empty";
            selectedFolder.value = "";
            treeData.value = [];
            activePath.value = "";
            void clearWorkspaceSession();
            return;
        }

        if (activePath.value !== path) {
            schedulePersist();
            return;
        }

        const nextActive =
            nextTabs[currentIndex] || nextTabs[currentIndex - 1] || null;
        activePath.value = nextActive ? nextActive.path : "";
        schedulePersist();
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

    async function releaseAllTabPayloads() {
        encodingChangeRequests.clear();
        if (!tabs.value.length) return;

        await Promise.all(tabs.value.map((tab) => revokeTabMedia(tab)));
        tabs.value = tabs.value.map((tab) => ({
            ...tab,
            source: null,
            content: "",
            error: "",
            encodingLoading: false,
        }));
    }

    return {
        handleReorderTab: handleReorder,
        handleOpenInNewTab,
        handleNewTab,
        handleCloseTab,
        releaseTabPayload,
        releaseAllTabPayloads,
    };
}
