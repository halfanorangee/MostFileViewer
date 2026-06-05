<template>
    <div class="excel-viewer">
        <div class="excel-viewer__toolbar">
            <select v-model="activeSheet" class="excel-viewer__sheet-select">
                <option
                    v-for="(sheet, index) in sheets"
                    :key="index"
                    :value="sheet.name"
                >
                    {{ sheet.name }}
                </option>
            </select>
            <span v-if="renderNotice" class="excel-viewer__notice">
                {{ renderNotice }}
            </span>
            <div class="excel-viewer__search">
                <input
                    v-model="searchQuery"
                    class="excel-viewer__search-input"
                    type="search"
                    placeholder="搜索当前工作表"
                    :disabled="!hasRenderableData"
                    @keydown.enter.prevent="handleSearchEnter"
                />
                <span class="excel-viewer__search-count">
                    {{ searchStatusText }}
                </span>
                <button
                    class="excel-viewer__search-button"
                    type="button"
                    :disabled="!searchMatches.length"
                    @click="goToPreviousSearchMatch"
                >
                    上一个
                </button>
                <button
                    class="excel-viewer__search-button"
                    type="button"
                    :disabled="!searchMatches.length"
                    @click="goToNextSearchMatch"
                >
                    下一个
                </button>
            </div>
            <span class="excel-viewer__zoom-label">
                {{ Math.round(zoom * 100) }}%
            </span>
        </div>

        <div v-if="!parsed" class="excel-viewer__loading">
            <p>{{ loadingText }}</p>
        </div>

        <div v-else-if="parseError" class="excel-viewer__error">
            <p>{{ parseError }}</p>
        </div>

        <div
            v-else
            ref="tableWrapperRef"
            class="excel-viewer__table-wrapper"
            @scroll="handleTableScroll"
            @wheel.ctrl.prevent="handleZoom"
        >
            <div v-if="!hasRenderableData" class="excel-viewer__empty">
                <p>当前工作表没有可预览的单元格内容。</p>
            </div>
            <div
                v-else
                class="excel-viewer__virtual-sheet"
                :style="virtualSheetStyle"
            >
                <div class="excel-viewer__corner" :style="cornerStyle"></div>
                <div
                    v-for="col in virtualColHeaders"
                    :key="col.index"
                    class="excel-viewer__col-header"
                    :style="col.style"
                >
                    {{ col.label }}
                    <span
                        class="excel-viewer__col-resizer"
                        @mousedown.stop.prevent="
                            startColumnResize(col.index, $event)
                        "
                        @dblclick.stop.prevent="resetColumnWidth(col.index)"
                    ></span>
                </div>
                <div
                    v-for="row in virtualRows"
                    :key="row.number"
                    class="excel-viewer__row-header"
                    :style="row.headerStyle"
                >
                    {{ row.number }}
                    <span
                        class="excel-viewer__row-resizer"
                        @mousedown.stop.prevent="
                            startRowResize(row.number, $event)
                        "
                        @dblclick.stop.prevent="resetRowHeight(row.number)"
                    ></span>
                </div>
                <template v-for="row in virtualRows" :key="`row:${row.number}`">
                    <div
                        v-for="cell in row.cells"
                        v-show="!cell.hidden"
                        :key="cell.address"
                        class="excel-viewer__cell"
                        :class="{
                            'excel-viewer__cell--search-match':
                                isSearchMatch(cell),
                            'excel-viewer__cell--search-active':
                                isActiveSearchMatch(cell),
                        }"
                        :style="cell.style"
                    >
                        <template v-if="cell.richText.length">
                            <span
                                v-for="(part, index) in cell.richText"
                                :key="index"
                                :style="part.style"
                                >{{ part.text }}</span
                            >
                        </template>
                        <template v-else>{{ cell.text }}</template>
                    </div>
                </template>
            </div>
        </div>
    </div>
</template>

<script setup>
import {
    computed,
    nextTick,
    onBeforeUnmount,
    onMounted,
    ref,
    shallowRef,
    watch,
} from "vue";

import { toArrayBuffer } from "../composables/excel/utils";
import {
    formatCellValue,
    getColumnWidth,
    getRowHeight,
} from "../composables/excel/styles";
import {
    isCsvFile,
    parseCsvWorkbook,
    parseXlsxFallbackWorkbook,
    getWorkbookSheetNames,
} from "../composables/excel/parser";
import { hasWorksheetDimensions } from "../composables/excel/worksheet";
import { useVirtualScroll } from "../composables/excel/virtualScroll";
import { createResizeHandlers } from "../composables/excel/resize";
import {
    ROW_HEADER_WIDTH,
    COLUMN_HEADER_HEIGHT,
} from "../composables/excel/constants";
import { runExcelWorkerTask } from "../composables/excel/workerClient";

// ── Props & Emits ──────────────────────────────────────────

const props = defineProps({
    src: { type: [ArrayBuffer, Object], default: null },
    extension: { type: String, default: "" },
    encoding: { type: String, default: "utf-8" },
});

const emit = defineEmits(["error"]);

// ── Reactive state ─────────────────────────────────────────

const parsed = ref(false);
const parseError = ref("");
const sheets = ref([]);
const activeSheet = ref("");
const zoom = ref(1);
const customColumnWidths = ref({});
const customRowHeights = ref({});
const searchQuery = ref("");
const searchMatches = ref([]);
const activeMatchIndex = ref(-1);
const renderNotice = ref("");

const tableWrapperRef = ref(null);
const viewport = shallowRef({ width: 0, height: 0 });
const scrollPosition = shallowRef({ left: 0, top: 0 });

let workbookData = null;
let xlsxFallbackData = null;
const worksheetCache = new WeakMap();
let resizeObserver = null;
let observedWrapper = null;
let searchDebounceTimer = null;
let searchSequence = 0;

// ── Sheet key helper ───────────────────────────────────────

function sizeKey(index) {
    return `${encodeURIComponent(activeSheet.value)}:${index}`;
}

// ── Active worksheet ───────────────────────────────────────

const activeWorksheet = computed(() => {
    if (!activeSheet.value) return null;

    const excelWorksheet =
        workbookData?.getWorksheet(activeSheet.value) ?? null;
    if (excelWorksheet && hasWorksheetDimensions(excelWorksheet)) {
        return excelWorksheet;
    }

    return (
        xlsxFallbackData?.getWorksheet(activeSheet.value) ??
        excelWorksheet ??
        null
    );
});

const previewKind = computed(() =>
    isCsvFile(props.extension) ? "csv" : "excel",
);

const loadingText = computed(() =>
    previewKind.value === "csv"
        ? "正在解析 CSV 文件..."
        : "正在解析 Excel 文件...",
);

// ── Virtual scroll ─────────────────────────────────────────

const {
    hasRenderableData,
    virtualSheetStyle,
    virtualColHeaders,
    virtualRows,
    axisMetrics,
} = useVirtualScroll({
    activeWorksheet,
    zoom,
    viewport,
    scrollPosition,
    customColumnWidths,
    customRowHeights,
    worksheetCache,
    sizeKeyFn: sizeKey,
});

const cornerStyle = computed(() => ({
    width: `${ROW_HEADER_WIDTH}px`,
    height: `${COLUMN_HEADER_HEIGHT * zoom.value}px`,
}));

const normalizedSearchQuery = computed(() =>
    searchQuery.value.trim().toLowerCase(),
);

const searchMatchKeys = computed(
    () =>
        new Set(
            searchMatches.value.map((match) => cellKey(match.row, match.col)),
        ),
);

const activeSearchMatchKey = computed(() => {
    const match = searchMatches.value[activeMatchIndex.value];
    return match ? cellKey(match.row, match.col) : "";
});

const searchStatusText = computed(() => {
    if (!normalizedSearchQuery.value) return "";
    if (!searchMatches.value.length) return "无结果";
    return `${activeMatchIndex.value + 1}/${searchMatches.value.length}`;
});

// ── Resize / Zoom handlers ─────────────────────────────────

const {
    handleZoom,
    startColumnResize,
    startRowResize,
    resetColumnWidth,
    resetRowHeight,
    cleanup: cleanupResize,
} = createResizeHandlers({
    zoom,
    customColumnWidths,
    customRowHeights,
    sizeKeyFn: sizeKey,
    getColumnWidthFn: (index) =>
        getColumnWidth(
            index,
            activeWorksheet.value?.getColumn?.(index) ?? null,
            customColumnWidths,
            sizeKey,
        ),
    getRowHeightFn: (index) =>
        getRowHeight(
            index,
            activeWorksheet.value?.getRow?.(index) ?? null,
            zoom.value,
            customRowHeights,
            sizeKey,
        ),
    updateViewport,
});

// ── Viewport ───────────────────────────────────────────────

let updateViewportPending = false;
let updateViewportRafId = null;

function updateViewport() {
    const wrapper = tableWrapperRef.value;
    if (!wrapper) return;

    if (resizeObserver && observedWrapper !== wrapper) {
        if (observedWrapper) resizeObserver.unobserve(observedWrapper);
        resizeObserver.observe(wrapper);
        observedWrapper = wrapper;
    }

    const nextWidth = wrapper.clientWidth;
    const nextHeight = wrapper.clientHeight;
    const nextScrollLeft = wrapper.scrollLeft;
    const nextScrollTop = wrapper.scrollTop;

    if (
        viewport.value.width !== nextWidth ||
        viewport.value.height !== nextHeight
    ) {
        viewport.value = { width: nextWidth, height: nextHeight };
    }
    if (
        scrollPosition.value.left !== nextScrollLeft ||
        scrollPosition.value.top !== nextScrollTop
    ) {
        scrollPosition.value = { left: nextScrollLeft, top: nextScrollTop };
    }
}

function handleTableScroll(event) {
    const nextLeft = event.currentTarget.scrollLeft;
    const nextTop = event.currentTarget.scrollTop;

    if (
        scrollPosition.value.left !== nextLeft ||
        scrollPosition.value.top !== nextTop
    ) {
        scrollPosition.value = { left: nextLeft, top: nextTop };
    }
}

// ── Search ─────────────────────────────────────────────────

function cellKey(row, col) {
    return `${row}:${col}`;
}

function scheduleSearch() {
    if (searchDebounceTimer !== null) {
        clearTimeout(searchDebounceTimer);
        searchDebounceTimer = null;
    }

    if (!normalizedSearchQuery.value) {
        searchSequence += 1;
        searchMatches.value = [];
        activeMatchIndex.value = -1;
        return;
    }

    searchDebounceTimer = setTimeout(() => {
        searchDebounceTimer = null;
        void rebuildSearchMatches();
    }, 120);
}

async function rebuildSearchMatches() {
    const worksheet = activeWorksheet.value;
    const query = normalizedSearchQuery.value;
    const currentSearch = ++searchSequence;

    if (!worksheet || !query) {
        searchMatches.value = [];
        activeMatchIndex.value = -1;
        return;
    }

    let matches = [];
    try {
        matches = await (worksheet.__previewGrid
            ? collectGridSearchMatches(worksheet, query)
            : collectExcelSearchMatches(worksheet, query));
    } catch {
        matches = [];
    }

    if (currentSearch !== searchSequence) {
        return;
    }

    searchMatches.value = matches;
    activeMatchIndex.value = matches.length ? 0 : -1;
    if (matches.length) {
        nextTick(() => scrollToSearchMatch(matches[0]));
    }
}

async function collectGridSearchMatches(worksheet, query) {
    return runExcelWorkerTask("gridSearch", {
        rows: worksheet.rows,
        query,
    });
}

async function collectExcelSearchMatches(worksheet, query) {
    const items = [];
    worksheet.eachRow?.({ includeEmpty: false }, (row, rowNumber) => {
        row.eachCell?.({ includeEmpty: false }, (cell, colNumber) => {
            const text = String(formatCellValue(cell.value, cell) ?? "");
            items.push({ row: rowNumber, col: colNumber, text });
        });
    });
    return runExcelWorkerTask("textsSearch", { items, query });
}

function handleSearchEnter(event) {
    if (event.shiftKey) {
        goToPreviousSearchMatch();
        return;
    }
    goToNextSearchMatch();
}

function goToNextSearchMatch() {
    if (!searchMatches.value.length) return;
    const nextIndex =
        activeMatchIndex.value < 0
            ? 0
            : (activeMatchIndex.value + 1) % searchMatches.value.length;
    setActiveSearchMatch(nextIndex);
}

function goToPreviousSearchMatch() {
    if (!searchMatches.value.length) return;
    const nextIndex =
        activeMatchIndex.value < 0
            ? searchMatches.value.length - 1
            : (activeMatchIndex.value - 1 + searchMatches.value.length) %
              searchMatches.value.length;
    setActiveSearchMatch(nextIndex);
}

function setActiveSearchMatch(index) {
    activeMatchIndex.value = index;
    nextTick(() => scrollToSearchMatch(searchMatches.value[index]));
}

function scrollToSearchMatch(match) {
    const wrapper = tableWrapperRef.value;
    if (!wrapper || !match) return;

    const metrics = axisMetrics.value;
    const cellTop = metrics.rowOffsets[match.row - 1] ?? 0;
    const cellBottom = metrics.rowOffsets[match.row] ?? cellTop;
    const cellLeft = metrics.colOffsets[match.col - 1] ?? 0;
    const cellRight = metrics.colOffsets[match.col] ?? cellLeft;
    const headerHeight = COLUMN_HEADER_HEIGHT * zoom.value;
    const rowHeaderWidth = ROW_HEADER_WIDTH;
    const bodyHeight = Math.max(0, wrapper.clientHeight - headerHeight);
    const bodyWidth = Math.max(0, wrapper.clientWidth - rowHeaderWidth);

    const top = clampScrollPosition(
        cellTop - headerHeight - (bodyHeight - (cellBottom - cellTop)) / 2,
        Math.max(0, metrics.totalHeight - wrapper.clientHeight),
    );
    const left = clampScrollPosition(
        cellLeft - rowHeaderWidth - (bodyWidth - (cellRight - cellLeft)) / 2,
        Math.max(0, metrics.totalWidth - wrapper.clientWidth),
    );

    wrapper.scrollTop = top;
    wrapper.scrollLeft = left;
    scrollPosition.value = { left, top };
}

function clampScrollPosition(value, max) {
    return Math.min(Math.max(0, value), max);
}

function isSearchMatch(cell) {
    return searchMatchKeys.value.has(
        cellKey(cell.rowNumber, cell.columnNumber),
    );
}

function isActiveSearchMatch(cell) {
    return (
        activeSearchMatchKey.value ===
        cellKey(cell.rowNumber, cell.columnNumber)
    );
}

const MAX_CSV_PREVIEW_BYTES = 8 * 1024 * 1024;
const CSV_ROW_SOFT_LIMIT = 20000;
const CSV_COLUMN_SOFT_LIMIT = 200;

function buildCsvNotice(buffer, worksheet) {
    const notices = [];

    if (buffer.byteLength > MAX_CSV_PREVIEW_BYTES * 0.5) {
        notices.push(
            `当前 CSV 约 ${(buffer.byteLength / 1024 / 1024).toFixed(1)} MB，首次打开可能稍慢。`,
        );
    }

    if (worksheet.rowCount > CSV_ROW_SOFT_LIMIT) {
        notices.push(
            `行数较多（${worksheet.rowCount} 行），滚动和搜索可能变慢。`,
        );
    }

    if (worksheet.columnCount > CSV_COLUMN_SOFT_LIMIT) {
        notices.push(
            `列数较多（${worksheet.columnCount} 列），表格渲染可能变慢。`,
        );
    }

    return notices.join(" ");
}

function ensureCsvPreviewSize(buffer) {
    if (buffer.byteLength <= MAX_CSV_PREVIEW_BYTES) {
        return;
    }

    throw new Error(
        `CSV 文件过大，暂不直接预览（当前 ${(buffer.byteLength / 1024 / 1024).toFixed(1)} MB，限制 ${(MAX_CSV_PREVIEW_BYTES / 1024 / 1024).toFixed(0)} MB）。请拆分文件后再打开。`,
    );
}

// ── Parsing ────────────────────────────────────────────────

async function parseExcel(src) {
    parsed.value = false;
    parseError.value = "";
    renderNotice.value = "";
    sheets.value = [];
    workbookData = null;
    xlsxFallbackData = null;
    customColumnWidths.value = {};
    customRowHeights.value = {};
    searchQuery.value = "";
    searchMatches.value = [];
    activeMatchIndex.value = -1;

    try {
        const buffer = toArrayBuffer(src);

        if (isCsvFile(props.extension)) {
            ensureCsvPreviewSize(buffer);
            workbookData = parseCsvWorkbook(buffer, props.encoding);
            sheets.value = workbookData.worksheets.map((ws) => ({
                name: ws.name,
            }));
            renderNotice.value = buildCsvNotice(
                buffer,
                workbookData.worksheets[0] ?? null,
            );
        } else {
            try {
                const { default: ExcelJS } = await import("exceljs");
                const workbook = new ExcelJS.Workbook();
                await workbook.xlsx.load(buffer);
                workbookData = workbook;
                sheets.value = getWorkbookSheetNames(workbook, null).map(
                    (name) => ({ name }),
                );
            } catch (error) {
                const { default: JSZip } = await import("jszip");
                xlsxFallbackData = await parseXlsxFallbackWorkbook(
                    buffer,
                    JSZip,
                );
                workbookData = xlsxFallbackData;
                sheets.value = getWorkbookSheetNames(
                    null,
                    xlsxFallbackData,
                ).map((name) => ({ name }));
            }
        }

        if (sheets.value.length > 0) {
            activeSheet.value = sheets.value[0].name;
        }

        parsed.value = true;
        await nextTick();
        updateViewport();
    } catch (err) {
        renderNotice.value = "";
        parseError.value = String(
            err?.message ??
                err ??
                (previewKind.value === "csv"
                    ? "CSV 解析失败"
                    : "Excel 解析失败"),
        );
        emit("error", parseError.value);
        parsed.value = true;
        await nextTick();
        updateViewport();
    }
}

// ── Watchers & Lifecycle ───────────────────────────────────

watch(
    () => [props.src, props.extension, props.encoding],
    async ([newSrc]) => {
        if (!newSrc) return;
        await parseExcel(newSrc);
    },
    { immediate: true },
);

watch(activeSheet, () => {
    nextTick(updateViewport);
    scheduleSearch();
});

watch(normalizedSearchQuery, () => {
    scheduleSearch();
});

onMounted(() => {
    updateViewport();
    if (typeof ResizeObserver !== "undefined") {
        // Batch ResizeObserver callbacks through requestAnimationFrame
        // to prevent layout thrashing and recursive update loops.
        resizeObserver = new ResizeObserver(() => {
            if (updateViewportPending) return;
            updateViewportPending = true;
            updateViewportRafId = requestAnimationFrame(() => {
                updateViewportPending = false;
                updateViewportRafId = null;
                updateViewport();
            });
        });
        if (tableWrapperRef.value) {
            resizeObserver.observe(tableWrapperRef.value);
        }
    } else {
        window.addEventListener("resize", updateViewport);
    }
});

onBeforeUnmount(() => {
    cleanupResize();
    resizeObserver?.disconnect();
    if (updateViewportRafId !== null) {
        cancelAnimationFrame(updateViewportRafId);
        updateViewportRafId = null;
    }
    if (searchDebounceTimer !== null) {
        clearTimeout(searchDebounceTimer);
        searchDebounceTimer = null;
    }
    searchSequence += 1;
    updateViewportPending = false;
    observedWrapper = null;
    window.removeEventListener("resize", updateViewport);
});
</script>

<style scoped>
.excel-viewer {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
}

.excel-viewer__toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
    padding: 8px 12px;
    border-bottom: 1px solid #e6edf7;
    background: #f9fafc;
    flex-shrink: 0;
}

.excel-viewer__sheet-select {
    padding: 4px 8px;
    border: 1px solid #dbe3f0;
    border-radius: 4px;
    font-size: 13px;
    background: #fff;
    color: #1f2937;
    outline: none;
    cursor: pointer;
}

.excel-viewer__sheet-select:focus {
    border-color: #2563eb;
}

.excel-viewer__notice {
    max-width: min(48vw, 420px);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
    color: #9a3412;
}

.excel-viewer__search {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
}

.excel-viewer__search-input {
    width: min(240px, 32vw);
    height: 28px;
    padding: 0 8px;
    border: 1px solid #d7deea;
    border-radius: 4px;
    background: #fff;
    color: #1f2937;
    font-size: 13px;
    outline: none;
}

.excel-viewer__search-input:focus {
    border-color: #2563eb;
    box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.12);
}

.excel-viewer__search-input:disabled,
.excel-viewer__search-button:disabled {
    cursor: not-allowed;
    opacity: 0.55;
}

.excel-viewer__search-count {
    min-width: 46px;
    color: #64748b;
    font-size: 12px;
    text-align: center;
}

.excel-viewer__search-button {
    height: 28px;
    padding: 0 8px;
    border: 1px solid #d7deea;
    border-radius: 4px;
    background: #fff;
    color: #334155;
    font-size: 13px;
    cursor: pointer;
}

.excel-viewer__search-button:not(:disabled):hover {
    background: #f3f7ff;
    border-color: #c5d2e6;
}

.excel-viewer__zoom-label {
    font-size: 12px;
    color: #5b6b83;
    margin-left: auto;
}

.excel-viewer__loading,
.excel-viewer__error,
.excel-viewer__empty {
    display: grid;
    flex: 1;
    place-items: center;
    padding: 24px;
    text-align: center;
}

.excel-viewer__error {
    color: #c53030;
}

.excel-viewer__empty {
    color: #5b6b83;
}

.excel-viewer__table-wrapper {
    flex: 1;
    min-height: 0;
    overflow: auto;
    position: relative;
}

.excel-viewer__virtual-sheet {
    position: relative;
    min-width: 100%;
    min-height: 100%;
    white-space: nowrap;
}

.excel-viewer__corner {
    position: sticky;
    top: 0;
    left: 0;
    z-index: 4;
    background: #eef2f7;
    border: 1px solid #d7deea;
}

.excel-viewer__col-header,
.excel-viewer__row-header {
    position: absolute;
    box-sizing: border-box;
    user-select: none;
    background: #eef2f7;
    border: 1px solid #d7deea;
    padding: 0 8px;
    font-weight: 600;
    color: #374151;
    text-align: center;
    overflow: hidden;
    z-index: 2;
}

.excel-viewer__col-header {
    z-index: 3;
}

.excel-viewer__row-header {
    z-index: 2;
}

.excel-viewer__col-header::after,
.excel-viewer__row-header::after {
    content: "";
    position: absolute;
    background: transparent;
}

.excel-viewer__col-resizer {
    position: absolute;
    top: 0;
    right: -4px;
    width: 8px;
    height: 100%;
    cursor: col-resize;
    z-index: 4;
}

.excel-viewer__row-resizer {
    position: absolute;
    left: 0;
    bottom: -4px;
    width: 100%;
    height: 8px;
    cursor: row-resize;
    z-index: 4;
}

.excel-viewer__cell {
    position: absolute;
    box-sizing: border-box;
    border: 1px solid #d7deea;
    padding: 4px 8px;
    color: #1f2937;
    line-height: 1.4;
    overflow: hidden;
    text-overflow: ellipsis;
}

.excel-viewer__cell--search-match {
    box-shadow: inset 0 0 0 2px #f59e0b;
}

.excel-viewer__cell--search-active {
    z-index: 1;
    box-shadow:
        inset 0 0 0 2px #2563eb,
        0 0 0 2px rgba(37, 99, 235, 0.18);
}

@media (max-width: 768px) {
    .excel-viewer__search {
        order: 3;
        width: 100%;
    }

    .excel-viewer__search-input {
        flex: 1;
        width: auto;
    }

    .excel-viewer__zoom-label {
        margin-left: 0;
    }
}
</style>
