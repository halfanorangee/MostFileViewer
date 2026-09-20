import { nextTick } from "vue";
import { App } from "../../bindings/MostFileViewer";

export function useFileSystemSync({
    selectedFolder,
    treeData,
    openTabs,
    activeTabPath,
    isFolderPreview,
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
    replaceTabPath = () => {},
}) {
    function findNodeByPath(nodes, targetPath) {
        for (const node of nodes || []) {
            if (node.path === targetPath) {
                return node;
            }
            const found = findNodeByPath(node.children, targetPath);
            if (found) {
                return found;
            }
        }
        return null;
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

    async function handleFsChange(event) {
        const payload = event?.data;
        if (!payload?.parent || !isFolderPreview()) {
            return;
        }

        const parent = payload.parent;
        const hasExternalRename = (payload.changes || []).some(
            (change) => change?.op === "rename",
        );

        if (parent === selectedFolder.value) {
            let fresh;
            try {
                fresh = await App.LoadFolderChildren(parent);
            } catch (error) {
                return;
            }

            const oldByPath = new Map(treeData.value.map((node) => [node.path, node]));
            const merged = fresh.map((node) => {
                const old = oldByPath.get(node.path);
                if (old && old.type === "folder") {
                    return {
                        ...node,
                        loaded: old.loaded,
                        children: old.children,
                        hasChild: old.hasChild || node.hasChild,
                    };
                }
                return node;
            });
            await nextTick();
            treeData.value = merged;
        } else {
            const parentNode = findNodeByPath(treeData.value, parent);
            if (!parentNode || parentNode.type !== "folder") {
                return;
            }

            let fresh;
            try {
                fresh = await App.LoadFolderChildren(parent);
            } catch (error) {
                return;
            }

            await nextTick();
            treeData.value = parentNode.loaded
                ? updateTreeNode(treeData.value, parent, {
                      children: fresh,
                      loaded: true,
                      hasChild: fresh.length > 0,
                  })
                : updateTreeNode(treeData.value, parent, {
                      hasChild: fresh.length > 0,
                  });
        }

        if (hasExternalRename) {
            await reconcileRenamedTabs();
        }
    }

    function stripExtName(path) {
        const name = String(path || "").split(/[\\/]/).pop() || "";
        const dot = name.lastIndexOf(".");
        return dot > 0 ? name.slice(0, dot) : name;
    }

    async function reconcileRenamedTabs() {
        const newPaths = new Set();
        const pathByStem = new Map();

        function collectPaths(nodes) {
            for (const node of nodes || []) {
                if (!node?.path) continue;
                newPaths.add(node.path);
                pathByStem.set(stripExtName(node.path), node.path);
                collectPaths(node.children);
            }
        }

        collectPaths(treeData.value);
        for (const tab of openTabs.value) {
            if (!tab.path || tab.virtual || tab.previewOnly) continue;
            if (newPaths.has(tab.path)) continue;

            try {
                await App.GetFileInfo(tab.path);
                continue;
            } catch (error) {
                // The path is missing; check whether only the extension changed.
            }

            const candidate = pathByStem.get(stripExtName(tab.path));
            if (candidate && candidate !== tab.path) {
                await migrateTabToNewPath(tab, candidate);
            } else {
                updateTab(tab.path, {
                    status: "error",
                    error: "文件已被移动或重命名",
                });
            }
        }
    }

    async function migrateTabToNewPath(tab, newPath) {
        const oldPath = tab.path;
        const newExt = getPathExtension(newPath);
        const newName = getPathName(newPath);
        const sourceVersion = nextMediaSourceVersion();
        await revokeTabMedia(tab);
        await unregisterOpenPath(oldPath);

        openTabs.value = openTabs.value.map((item) =>
            item.path === oldPath
                ? {
                      ...item,
                      path: newPath,
                      name: newName,
                      extension: newExt,
                      previewType: getPreviewType(newExt),
                      media: {
                          token: "",
                          sourceVersion,
                          capability: "unknown",
                          loadState: "idle",
                          errorCode: "",
                          errorMessage: "",
                      },
                  }
                : item,
        );
        // 外部重命名后同步分屏布局中的 path，确保该 tab 留在原 pane。
        replaceTabPath(oldPath, newPath);
        if (activeTabPath.value === oldPath) {
            activeTabPath.value = newPath;
        }

        await registerOpenPath(newPath);
        try {
            const loaded = await loadTabPreview(newPath, newExt, sourceVersion);
            const currentTab = openTabs.value.find((item) => item.path === newPath);
            if (
                !currentTab ||
                (loaded.isMediaPreview &&
                    currentTab.media?.sourceVersion !== loaded.media?.sourceVersion)
            ) {
                await revokeMediaToken(loaded.media?.token);
                return;
            }
            updateTab(newPath, buildLoadedTabPatch(loaded, newExt, currentTab));
        } catch (error) {
            updateTab(newPath, {
                status: "error",
                error: normalizeError(error, "读取文件失败"),
            });
        }
    }

    function handleFileRemoved(event) {
        const path = event?.data?.path;
        if (!path) return;
        openTabs.value.forEach((tab) => {
            if (tab.path === path) {
                updateTab(path, {
                    status: "error",
                    error: "文件已被删除",
                });
            }
        });
    }

    return {
        handleFsChange,
        handleFileRemoved,
        updateTreeNode,
    };
}
