import { computed, ref, watch } from "vue";
import { columnLabel } from "./utils";
import {
  getColumnStyle,
  getBaseCellStyle,
  getRowHeight,
  getColumnWidth,
} from "./styles";
import {
  getWorksheetCache,
  getWorksheetDimensions,
  hasWorksheetDimensions,
} from "./worksheet";
import {
  ROW_HEADER_WIDTH,
  COLUMN_HEADER_HEIGHT,
  DEFAULT_COLUMN_WIDTH,
  DEFAULT_ROW_HEIGHT,
  VIRTUAL_OVERSCAN_ROWS,
  VIRTUAL_OVERSCAN_COLS,
} from "./constants";
import { runExcelWorkerTask } from "./workerClient";

/**
 * Core hook for virtual scrolling computed properties.
 * Works with external reactive state: activeWorksheet, zoom, viewport, scrollPosition,
 * customColumnWidths, customRowHeights, worksheetCache, sizeKeyFn.
 */
export function useVirtualScroll(state) {
  const {
    activeWorksheet,
    zoom,
    viewport,
    scrollPosition,
    customColumnWidths,
    customRowHeights,
    worksheetCache,
    sizeKeyFn,
  } = state;

  const isGridWorksheet = computed(() =>
    Boolean(activeWorksheet.value?.__previewGrid),
  );
  const workerAxisMetrics = ref(null);
  let axisMetricsRequest = 0;

  const renderBounds = computed(() => {
    const ws = activeWorksheet.value;
    if (!ws) return { rows: 0, cols: 0, totalRows: 0, totalCols: 0 };

    const dimensions = getWorksheetDimensions(ws);
    const totalRows = dimensions.rows;
    const totalCols = dimensions.cols;
    return { rows: totalRows, cols: totalCols, totalRows, totalCols };
  });

  const hasRenderableData = computed(() => {
    const bounds = renderBounds.value;
    return bounds.rows > 0 && bounds.cols > 0;
  });

  const axisMetrics = computed(() => {
    if (workerAxisMetrics.value) {
      return workerAxisMetrics.value;
    }

    // Local fallback when worker is unavailable – compute proper offsets
    // so virtual scrolling works without the worker.
    const bounds = renderBounds.value;
    const rowOffsets = new Array(bounds.rows + 1);
    const colOffsets = new Array(bounds.cols + 1);
    rowOffsets[0] = COLUMN_HEADER_HEIGHT * zoom.value;
    colOffsets[0] = ROW_HEADER_WIDTH;

    for (let r = 1; r <= bounds.rows; r += 1) {
      rowOffsets[r] = rowOffsets[r - 1] + DEFAULT_ROW_HEIGHT * zoom.value;
    }
    for (let c = 1; c <= bounds.cols; c += 1) {
      colOffsets[c] = colOffsets[c - 1] + DEFAULT_COLUMN_WIDTH * zoom.value;
    }

    return {
      rowOffsets,
      colOffsets,
      totalHeight: rowOffsets[bounds.rows] ?? rowOffsets[0],
      totalWidth: colOffsets[bounds.cols] ?? colOffsets[0],
    };
  });

  watch(
    [
      activeWorksheet,
      renderBounds,
      zoom,
      () => customColumnWidths.value,
      () => customRowHeights.value,
    ],
    () => {
      const ws = activeWorksheet.value;
      const bounds = renderBounds.value;
      axisMetricsRequest += 1;
      const requestId = axisMetricsRequest;

      if (!ws || !bounds.rows || !bounds.cols) {
        workerAxisMetrics.value = null;
        return;
      }

      const worksheetSizes = extractWorksheetSizes(ws, zoom.value);

      runExcelWorkerTask("axisMetrics", {
        rows: bounds.rows,
        cols: bounds.cols,
        zoom: zoom.value,
        rowHeaderWidth: ROW_HEADER_WIDTH,
        columnHeaderHeight: COLUMN_HEADER_HEIGHT,
        defaultRowHeight: DEFAULT_ROW_HEIGHT,
        defaultColumnWidth: DEFAULT_COLUMN_WIDTH,
        rowHeights: worksheetSizes.rowHeights,
        columnWidths: worksheetSizes.columnWidths,
        customRowHeights: extractCustomSizes(customRowHeights.value),
        customColumnWidths: extractCustomSizes(customColumnWidths.value),
      })
        .then((metrics) => {
          if (requestId === axisMetricsRequest) {
            workerAxisMetrics.value = metrics;
          }
        })
        .catch(() => {
          if (requestId === axisMetricsRequest) {
            workerAxisMetrics.value = null;
          }
        });
    },
    { immediate: true },
  );

  const virtualRange = computed(() => {
    const bounds = renderBounds.value;
    const metrics = axisMetrics.value;
    const { left, top } = scrollPosition.value;
    const { width, height } = viewport.value;

    if (!bounds.rows || !bounds.cols) {
      return {
        rowStart: 1,
        rowEnd: 0,
        colStart: 1,
        colEnd: 0,
      };
    }

    const rowStart = Math.max(
      1,
      findOffsetIndex(metrics.rowOffsets, top) - VIRTUAL_OVERSCAN_ROWS,
    );
    const rowEnd = Math.min(
      bounds.rows,
      findOffsetIndex(metrics.rowOffsets, top + height) + VIRTUAL_OVERSCAN_ROWS,
    );
    const colStart = Math.max(
      1,
      findOffsetIndex(metrics.colOffsets, left) - VIRTUAL_OVERSCAN_COLS,
    );
    const colEnd = Math.min(
      bounds.cols,
      findOffsetIndex(metrics.colOffsets, left + width) + VIRTUAL_OVERSCAN_COLS,
    );

    return { rowStart, rowEnd, colStart, colEnd };
  });

  const virtualSheetStyle = computed(() => ({
    width: `${axisMetrics.value.totalWidth}px`,
    height: `${axisMetrics.value.totalHeight}px`,
    fontSize: `${10 * zoom.value}px`,
  }));

  const virtualColHeaders = computed(() => {
    const ws = activeWorksheet.value;
    if (!ws) return [];

    const { colStart, colEnd } = virtualRange.value;
    const headers = [];
    for (let c = colStart; c <= colEnd; c += 1) {
      headers.push({
        index: c,
        label: columnLabel(c),
        style: {
          ...getColumnStyle(
            isGridWorksheet.value ? null : ws.getColumn(c),
            c,
            zoom.value,
            (index, column) =>
              getColumnWidth(index, column, customColumnWidths, sizeKeyFn),
          ),
          left: `${axisMetrics.value.colOffsets[c - 1]}px`,
          top: `${scrollPosition.value.top}px`,
          height: `${COLUMN_HEADER_HEIGHT * zoom.value}px`,
          lineHeight: `${COLUMN_HEADER_HEIGHT * zoom.value - 1}px`,
        },
      });
    }
    return headers;
  });

  const virtualRows = computed(() => {
    const ws = activeWorksheet.value;
    if (!ws) return [];

    const { rowStart, rowEnd, colStart, colEnd } = virtualRange.value;
    const scrollLeft = scrollPosition.value.left;
    const rows = [];
    for (let r = rowStart; r <= rowEnd; r += 1) {
      rows.push(
        buildRenderableRow(
          ws,
          r,
          colStart,
          colEnd,
          isGridWorksheet.value,
          zoom.value,
          axisMetrics.value,
          scrollLeft,
          customColumnWidths,
          customRowHeights,
          worksheetCache,
          sizeKeyFn,
        ),
      );
    }
    return rows;
  });

  return {
    isGridWorksheet,
    renderBounds,
    hasRenderableData,
    axisMetrics,
    virtualRange,
    virtualSheetStyle,
    virtualColHeaders,
    virtualRows,
  };
}

function findOffsetIndex(offsets, value) {
  let low = 0;
  let high = offsets.length - 1;

  while (low < high) {
    const mid = Math.floor((low + high) / 2);
    if (offsets[mid] <= value) {
      low = mid + 1;
    } else {
      high = mid;
    }
  }

  return Math.max(1, low);
}

function extractCustomSizes(sizes) {
  const result = {};
  for (const [key, value] of Object.entries(sizes ?? {})) {
    const index = Number(String(key).split(":").pop());
    if (Number.isFinite(index) && index > 0) {
      result[index] = value;
    }
  }
  return result;
}

function extractWorksheetSizes(worksheet, currentZoom) {
  if (worksheet.__previewGrid) {
    return { rowHeights: {}, columnWidths: {} };
  }

  const rowHeights = {};
  for (const [index, row] of Object.entries(worksheet._rows ?? {})) {
    if (!row?.height) continue;
    const rowNumber = Number(index) + 1;
    if (Number.isFinite(rowNumber) && rowNumber > 0) {
      rowHeights[rowNumber] = getRowHeight(
        rowNumber,
        row,
        currentZoom,
        emptySizesRef,
        emptySizeKey,
      );
    }
  }

  const columnWidths = {};
  worksheet.columns?.forEach((column, index) => {
    if (!column?.width) return;
    const columnNumber = index + 1;
    columnWidths[columnNumber] = getColumnWidth(
      columnNumber,
      column,
      emptySizesRef,
      emptySizeKey,
    );
  });

  return { rowHeights, columnWidths };
}

const emptySizesRef = { value: {} };

function emptySizeKey() {
  return "";
}

function buildRenderableRow(
  worksheet,
  rowNumber,
  colStart,
  colEnd,
  isGrid,
  zoom,
  metrics,
  scrollLeft,
  customColumnWidths,
  customRowHeights,
  worksheetCache,
  sizeKeyFn,
) {
  if (isGrid) {
    return buildGridRenderableRow(
      worksheet,
      rowNumber,
      colStart,
      colEnd,
      zoom,
      metrics,
      scrollLeft,
      customColumnWidths,
      customRowHeights,
      sizeKeyFn,
    );
  }

  const worksheetRow = worksheet.getRow(rowNumber);
  const mergeMap = getWorksheetCache(worksheet, worksheetCache).mergeMap;
  const cells = [];

  for (let c = colStart; c <= colEnd; c += 1) {
    const cell = worksheet.getCell(rowNumber, c);
    const merge = mergeMap.get(`${rowNumber}:${c}`);
    cells.push(
      withVirtualCellPosition(
        createRenderableCell(
          cell,
          worksheet.getColumn(c),
          c,
          merge,
          zoom,
          customColumnWidths,
          sizeKeyFn,
        ),
        rowNumber,
        c,
        metrics,
      ),
    );
  }

  return {
    number: rowNumber,
    headerStyle: getVirtualRowHeaderStyle(
      worksheetRow,
      rowNumber,
      zoom,
      metrics,
      scrollLeft,
      customRowHeights,
      sizeKeyFn,
    ),
    cells,
  };
}

function buildGridRenderableRow(
  worksheet,
  rowNumber,
  colStart,
  colEnd,
  zoom,
  metrics,
  scrollLeft,
  customColumnWidths,
  customRowHeights,
  sizeKeyFn,
) {
  const values = worksheet.rows[rowNumber - 1] ?? [];
  const cells = [];

  for (let c = colStart; c <= colEnd; c += 1) {
    const text = values[c - 1] ?? "";
    cells.push(
      withVirtualCellPosition(
        {
          address: `${columnLabel(c)}${rowNumber}`,
          text,
          richText: [],
          hidden: false,
          rowspan: 1,
          colspan: 1,
          style: {
            ...getBaseCellStyle(null, c, zoom, (index, column) =>
              getColumnWidth(index, column, customColumnWidths, sizeKeyFn),
            ),
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          },
        },
        rowNumber,
        c,
        metrics,
      ),
    );
  }

  return {
    number: rowNumber,
    headerStyle: getVirtualRowHeaderStyle(
      null,
      rowNumber,
      zoom,
      metrics,
      scrollLeft,
      customRowHeights,
      sizeKeyFn,
    ),
    cells,
  };
}

function withVirtualCellPosition(cell, rowNumber, columnNumber, metrics) {
  const top = metrics.rowOffsets[rowNumber - 1];
  const left = metrics.colOffsets[columnNumber - 1];
  const width = getSpanSize(
    metrics.colOffsets,
    columnNumber,
    cell.colspan ?? 1,
  );
  const height = getSpanSize(metrics.rowOffsets, rowNumber, cell.rowspan ?? 1);

  return {
    ...cell,
    rowNumber,
    columnNumber,
    style: {
      ...cell.style,
      top: `${top}px`,
      left: `${left}px`,
      width: `${width}px`,
      minWidth: `${width}px`,
      maxWidth: `${width}px`,
      height: `${height}px`,
    },
  };
}

function getSpanSize(offsets, startIndex, span) {
  const endIndex = Math.min(
    offsets.length - 1,
    startIndex + Math.max(1, span) - 1,
  );
  return offsets[endIndex] - offsets[startIndex - 1];
}

function getVirtualRowHeaderStyle(
  row,
  index,
  zoom,
  metrics,
  scrollLeft,
  customRowHeights,
  sizeKeyFn,
) {
  const height =
    getRowHeight(index, row, zoom, customRowHeights, sizeKeyFn) * zoom;
  return {
    top: `${metrics.rowOffsets[index - 1]}px`,
    left: `${scrollLeft}px`,
    width: `${ROW_HEADER_WIDTH}px`,
    height: `${height}px`,
    lineHeight: `${height - 1}px`,
  };
}

// Re-export for use in the main component
import { getCellStyle, getRichText, formatCellValue } from "./styles";

function createRenderableCell(
  cell,
  column,
  columnIndex,
  merge,
  zoom,
  customColumnWidths,
  sizeKeyFn,
) {
  return {
    address: cell.address,
    text: formatCellValue(cell.value, cell),
    richText: getRichText(cell.value, zoom),
    hidden: merge?.hidden ?? false,
    rowspan: merge?.rowspan ?? 1,
    colspan: merge?.colspan ?? 1,
    style: {
      ...getBaseCellStyle(column, columnIndex, zoom, (index, column) =>
        getColumnWidth(index, column, customColumnWidths, sizeKeyFn),
      ),
      ...getCellStyle(cell, zoom),
    },
  };
}
