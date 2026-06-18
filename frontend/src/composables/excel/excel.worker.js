self.onmessage = (event) => {
    const { requestId, type, payload } = event.data ?? {};
    try {
        const result = runTask(type, payload);
        self.postMessage({ requestId, result });
    } catch (error) {
        self.postMessage({
            requestId,
            error: String(error?.message ?? error ?? "Excel Worker 执行失败"),
        });
    }
};

function runTask(type, payload) {
    switch (type) {
        case "gridDimensions":
            return getGridDimensions(payload.rows, payload.columnCount);
        case "axisMetrics":
            return buildAxisMetrics(payload);
        case "gridSearch":
            return searchGrid(payload.rows, payload.query);
        case "textsSearch":
            return searchTexts(payload.items, payload.query);
        default:
            throw new Error(`未知 Excel Worker 任务: ${type}`);
    }
}

function getGridDimensions(rows, columnCount) {
    return {
        rows: Array.isArray(rows) ? rows.length : 0,
        cols: columnCount || getMaxColumnCount(rows),
    };
}

function getMaxColumnCount(rows) {
    let max = 0;
    for (const row of rows ?? []) {
        max = Math.max(max, Array.isArray(row) ? row.length : 0);
    }
    return max;
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
        totalRows,
        totalCols,
        extraEmptyRowHeight = 30,
        extraEmptyColumnWidth = 100,
    } = payload;
    const rowOffsets = new Array(rows + 1);
    const colOffsets = new Array(cols + 1);
    rowOffsets[0] = columnHeaderHeight * zoom;
    colOffsets[0] = rowHeaderWidth;

    const actualRows = totalRows ?? rows;
    const actualCols = totalCols ?? cols;

    for (let r = 1; r <= actualRows; r += 1) {
        rowOffsets[r] = rowOffsets[r - 1] + (customRowHeights?.[r] || rowHeights?.[r] || defaultRowHeight) * zoom;
    }
    // 额外的空白行
    for (let r = actualRows + 1; r <= rows; r += 1) {
        rowOffsets[r] = rowOffsets[r - 1] + extraEmptyRowHeight * zoom;
    }

    for (let c = 1; c <= actualCols; c += 1) {
        colOffsets[c] = colOffsets[c - 1] + (customColumnWidths?.[c] || columnWidths?.[c] || defaultColumnWidth) * zoom;
    }
    // 额外的空白列
    for (let c = actualCols + 1; c <= cols; c += 1) {
        colOffsets[c] = colOffsets[c - 1] + extraEmptyColumnWidth * zoom;
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
        String(item.text ?? "").toLowerCase().includes(normalizedQuery),
    );
}
