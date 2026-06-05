import ExcelWorker from "./excel.worker.js?worker";

let worker = null;
let nextRequestId = 1;
const pendingRequests = new Map();
let workerUnavailable = false;

export function runExcelWorkerTask(type, payload) {
  if (workerUnavailable || typeof Worker === "undefined") {
    return runTaskLocally(type, payload);
  }

  const requestId = nextRequestId;
  nextRequestId += 1;

  return new Promise((resolve, reject) => {
    try {
      const currentWorker = getWorker();
      currentWorker.postMessage({ requestId, type, payload });
      pendingRequests.set(requestId, { resolve, reject });
    } catch (error) {
      workerUnavailable = true;
      rejectPendingRequests(error);
      runTaskLocally(type, payload).then(resolve).catch(reject);
    }
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

  worker = new ExcelWorker({
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
    workerUnavailable = true;
    const message = event.message || "Excel Worker 执行失败";
    rejectPendingRequests(new Error(message));
    worker?.terminate();
    worker = null;
  };
  return worker;
}

function rejectPendingRequests(error) {
  for (const { reject } of pendingRequests.values()) {
    reject(error instanceof Error ? error : new Error(String(error)));
  }
  pendingRequests.clear();
}

function runTaskLocally(type, payload) {
  try {
    return Promise.resolve(runTask(type, payload));
  } catch (error) {
    return Promise.reject(
      error instanceof Error ? error : new Error(String(error)),
    );
  }
}

function runTask(type, payload) {
  switch (type) {
    case "axisMetrics":
      return buildAxisMetrics(payload);
    case "gridSearch":
      return searchGrid(payload?.rows, payload?.query);
    case "textsSearch":
      return searchTexts(payload?.items, payload?.query);
    default:
      throw new Error(`未知 Excel Worker 任务: ${type}`);
  }
}

function buildAxisMetrics(payload) {
  const {
    rows,
    cols,
    zoom,
    rowHeaderWidth,
    columnHeaderHeight,
    defaultRowHeight,
    defaultColumnWidth,
    customRowHeights,
    customColumnWidths,
    rowHeights,
    columnWidths,
  } = payload ?? {};
  const rowOffsets = new Array(rows + 1);
  const colOffsets = new Array(cols + 1);
  rowOffsets[0] = columnHeaderHeight * zoom;
  colOffsets[0] = rowHeaderWidth;

  for (let r = 1; r <= rows; r += 1) {
    rowOffsets[r] =
      rowOffsets[r - 1] +
      (customRowHeights?.[r] || rowHeights?.[r] || defaultRowHeight) * zoom;
  }

  for (let c = 1; c <= cols; c += 1) {
    colOffsets[c] =
      colOffsets[c - 1] +
      (customColumnWidths?.[c] || columnWidths?.[c] || defaultColumnWidth) *
        zoom;
  }

  return {
    rowOffsets,
    colOffsets,
    totalHeight: rowOffsets[rows] ?? columnHeaderHeight * zoom,
    totalWidth: colOffsets[cols] ?? rowHeaderWidth,
  };
}

function searchGrid(rows, query) {
  const normalizedQuery = String(query ?? "").toLowerCase();
  if (!normalizedQuery) return [];

  const matches = [];
  rows?.forEach((row, rowIndex) => {
    row?.forEach((value, colIndex) => {
      const text = String(value ?? "");
      if (text.toLowerCase().includes(normalizedQuery)) {
        matches.push({ row: rowIndex + 1, col: colIndex + 1, text });
      }
    });
  });
  return matches;
}

function searchTexts(items, query) {
  const normalizedQuery = String(query ?? "").toLowerCase();
  if (!normalizedQuery) return [];
  return (items ?? []).filter((item) =>
    String(item.text ?? "")
      .toLowerCase()
      .includes(normalizedQuery),
  );
}
