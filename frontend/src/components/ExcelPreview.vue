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
            <span class="excel-viewer__zoom-label">
                {{ Math.round(zoom * 100) }}%
            </span>
        </div>

        <div v-if="!parsed" class="excel-viewer__loading">
            <p>正在解析 Excel 文件...</p>
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
                <div class="excel-viewer__corner"></div>
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
import { getColumnWidth, getRowHeight } from "../composables/excel/styles";
import {
    isCsvFile,
    parseCsvWorkbook,
    parseXlsxFallbackWorkbook,
    getWorkbookSheetNames,
} from "../composables/excel/parser";
import { hasWorksheetDimensions } from "../composables/excel/worksheet";
import { useVirtualScroll } from "../composables/excel/virtualScroll";
import { createResizeHandlers } from "../composables/excel/resize";

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

const tableWrapperRef = ref(null);
const viewport = shallowRef({ width: 0, height: 0 });
const scrollPosition = shallowRef({ left: 0, top: 0 });

let workbookData = null;
let xlsxFallbackData = null;
const worksheetCache = new WeakMap();
let resizeObserver = null;
let observedWrapper = null;

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

// ── Virtual scroll ─────────────────────────────────────────

const { hasRenderableData, virtualSheetStyle, virtualColHeaders, virtualRows } =
    useVirtualScroll({
        activeWorksheet,
        zoom,
        viewport,
        scrollPosition,
        customColumnWidths,
        customRowHeights,
        worksheetCache,
        sizeKeyFn: sizeKey,
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

// ── Parsing ────────────────────────────────────────────────

const renderNotice = computed(() => "");

async function parseExcel(src) {
    parsed.value = false;
    parseError.value = "";
    sheets.value = [];
    workbookData = null;
    xlsxFallbackData = null;
    customColumnWidths.value = {};
    customRowHeights.value = {};

    try {
        const buffer = toArrayBuffer(src);

        if (isCsvFile(props.extension)) {
            workbookData = parseCsvWorkbook(buffer, props.encoding);
            sheets.value = workbookData.worksheets.map((ws) => ({
                name: ws.name,
            }));
        } else {
            const [{ default: ExcelJS }, { default: JSZip }] =
                await Promise.all([import("exceljs"), import("jszip")]);
            const workbook = new ExcelJS.Workbook();
            await workbook.xlsx.load(buffer);
            workbookData = workbook;
            xlsxFallbackData = await parseXlsxFallbackWorkbook(buffer, JSZip);
            sheets.value = getWorkbookSheetNames(
                workbook,
                xlsxFallbackData,
            ).map((name) => ({ name }));
        }

        if (sheets.value.length > 0) {
            activeSheet.value = sheets.value[0].name;
        }

        parsed.value = true;
        await nextTick();
        updateViewport();
    } catch (err) {
        parseError.value = String(err?.message ?? err ?? "Excel 解析失败");
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
    width: 40px;
    height: 28px;
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
</style>
