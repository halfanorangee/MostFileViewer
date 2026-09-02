export function useAutoSave({
    tabs,
    enabled,
    saveTab,
    debounceDelay = 2500,
    intervalDelay = 60000,
}) {
    const debounceTimers = new Map();
    let intervalTimer = null;

    function schedule(path) {
        clear(path);
        debounceTimers.set(
            path,
            setTimeout(() => {
                debounceTimers.delete(path);
                void saveTab(path);
            }, debounceDelay),
        );
    }

    function clear(path) {
        const timer = debounceTimers.get(path);
        if (!timer) return;
        clearTimeout(timer);
        debounceTimers.delete(path);
    }

    function clearAll() {
        debounceTimers.forEach((timer) => clearTimeout(timer));
        debounceTimers.clear();
    }

    function start() {
        stopInterval();
        intervalTimer = setInterval(() => {
            if (!enabled.value) return;
            tabs.value.forEach((tab) => {
                if (
                    tab.dirty &&
                    tab.status === "ready" &&
                    tab.previewType === "code" &&
                    !tab.saving
                ) {
                    void saveTab(tab.path);
                }
            });
        }, intervalDelay);
    }

    function stopInterval() {
        if (!intervalTimer) return;
        clearInterval(intervalTimer);
        intervalTimer = null;
    }

    function stop() {
        clearAll();
        stopInterval();
    }

    return { schedule, clear, clearAll, start, stop };
}
