import { App } from "../../bindings/MostFileViewer";

export function useWorkspaceSession({
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
}) {
    let restoring = false;
    let persistTimer = null;

    function createFileNode(path) {
        return {
            name: getPathName(path),
            path,
            type: "file",
            extension: getPathExtension(path),
        };
    }

    async function restore() {
        restoring = true;
        try {
            const session = await App.ConsumeRestoreSession();
            if (!session?.hasSession) {
                return;
            }

            if (Number.isFinite(session.leftPaneWidth) && session.leftPaneWidth > 0) {
                leftPaneWidth.value = session.leftPaneWidth;
            }

            if (session.mode === "folder" && session.rootPath) {
                const tree = await App.LoadFolderTree(session.rootPath);
                workspaceMode.value = "folder";
                selectedFolder.value = session.rootPath;
                treeData.value = tree;
                sidebarOpen.value = session.sidebarOpen !== false;
                openTabs.value = [];
                activeTabPath.value = "";
                await registerOpenPath(session.rootPath);
            } else if (session.openTabs?.length) {
                const firstPath = session.openTabs[0].path;
                workspaceMode.value = "file";
                selectedFolder.value = getParentPath(firstPath);
                treeData.value = [createFileNode(firstPath)];
                sidebarOpen.value = false;
                openTabs.value = [];
                activeTabPath.value = "";
            }

            for (const tab of session.openTabs || []) {
                if (tab?.path) {
                    await openFileNode(createFileNode(tab.path));
                }
            }

            if (
                session.activePath &&
                openTabs.value.some((tab) => tab.path === session.activePath)
            ) {
                activeTabPath.value = session.activePath;
            }

            if (workspaceMode.value === "empty" && openTabs.value.length === 0) {
                await clear();
                return;
            }
            await persistNow();
        } catch (error) {
            await clear();
        } finally {
            restoring = false;
        }
    }

    function schedulePersist() {
        if (restoring) {
            return;
        }
        if (persistTimer) {
            clearTimeout(persistTimer);
        }
        persistTimer = setTimeout(() => {
            persistTimer = null;
            void persistNow();
        }, 250);
    }

    async function persistNow() {
        if (restoring) {
            return;
        }

        const tabs = openTabs.value
            .filter(
                (tab) => tab?.path && tab.status !== "error" && !tab.previewOnly,
            )
            .map((tab) => ({ path: tab.path }));

        if (workspaceMode.value === "empty" && tabs.length === 0) {
            await clear();
            return;
        }

        try {
            await App.SaveWindowSession({
                mode: workspaceMode.value,
                rootPath: selectedFolder.value || "",
                openTabs: tabs,
                activePath: activeTabPath.value || "",
                sidebarOpen: sidebarOpen.value,
                leftPaneWidth: leftPaneWidth.value,
            });
        } catch (error) {
            // Session persistence is best-effort.
        }
    }

    async function clear() {
        try {
            await App.ClearWindowSession();
        } catch (error) {
            // Session persistence is best-effort.
        }
    }

    function stop() {
        if (!persistTimer) {
            return;
        }
        clearTimeout(persistTimer);
        persistTimer = null;
    }

    return {
        restore,
        schedulePersist,
        persistNow,
        clear,
        stop,
    };
}
