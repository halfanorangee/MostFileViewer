let worker = null;
let nextRequestId = 1;
const pendingRequests = new Map();

export function runExcelWorkerTask(type, payload) {
    if (typeof Worker === "undefined") {
        return Promise.reject(new Error("当前环境不支持 Web Worker"));
    }

    const requestId = nextRequestId;
    nextRequestId += 1;

    return new Promise((resolve, reject) => {
        getWorker().postMessage({ requestId, type, payload });
        pendingRequests.set(requestId, { resolve, reject });
    });
}

export function terminateExcelWorker() {
    if (!worker) return;
    worker.terminate();
    worker = null;
    for (const { reject } of pendingRequests.values()) {
        reject(new Error("Excel Worker 已关闭"));
    }
    pendingRequests.clear();
}

function getWorker() {
    if (worker) return worker;

    worker = new Worker(new URL("./excel.worker.js", import.meta.url), {
        type: "module",
    });
    worker.onmessage = (event) => {
        const { requestId, result, error } = event.data ?? {};
        const pending = pendingRequests.get(requestId);
        if (!pending) return;
        pendingRequests.delete(requestId);
        if (error) {
            pending.reject(new Error(error));
            return;
        }
        pending.resolve(result);
    };
    worker.onerror = (event) => {
        const message = event.message || "Excel Worker 执行失败";
        for (const { reject } of pendingRequests.values()) {
            reject(new Error(message));
        }
        pendingRequests.clear();
        worker?.terminate();
        worker = null;
    };
    return worker;
}
