import { cellAddressToColumnNumber } from "./utils";

export function hasWorksheetDimensions(worksheet) {
    const dimensions = getWorksheetDimensions(worksheet);
    return dimensions.rows > 0 && dimensions.cols > 0;
}

export function getWorksheetDimensions(worksheet) {
    if (worksheet.__previewGrid) {
        return {
            rows: worksheet.rowCount || 0,
            cols: worksheet.columnCount || 0,
        };
    }

    const dimensions = worksheet.dimensions?.model;
    const bottom = dimensions?.bottom ?? dimensions?.br?.nativeRow ?? 0;
    const right = dimensions?.right ?? dimensions?.br?.nativeCol ?? 0;
    const scanned = scanWorksheetDimensions(worksheet);

    return {
        rows: Math.max(
            worksheet.rowCount || 0,
            worksheet.actualRowCount || 0,
            bottom,
            scanned.rows,
        ),
        cols: Math.max(
            worksheet.columnCount || 0,
            worksheet.actualColumnCount || 0,
            right,
            scanned.cols,
        ),
    };
}

export function getRenderableDimensions(worksheet) {
    const base = getWorksheetDimensions(worksheet);
    // 增加 1 行空白行和 1 列空白列用于边框渲染
    return {
        rows: base.rows + 1,
        cols: base.cols + 1,
        totalRows: base.rows,
        totalCols: base.cols,
    };
}

function scanWorksheetDimensions(worksheet) {
    let rows = 0;
    let cols = 0;

    worksheet.eachRow?.({ includeEmpty: false }, (row, rowNumber) => {
        rows = Math.max(rows, rowNumber);
        row.eachCell?.({ includeEmpty: false }, (_cell, colNumber) => {
            cols = Math.max(cols, colNumber);
        });

        if (Array.isArray(row.values)) {
            cols = Math.max(cols, row.values.length - 1);
        }
    });

    for (const [index, row] of Object.entries(worksheet._rows ?? {})) {
        if (!row) continue;
        const rowNumber = Number(index) + 1;
        rows = Math.max(rows, rowNumber);

        row.eachCell?.({ includeEmpty: false }, (_cell, colNumber) => {
            cols = Math.max(cols, colNumber);
        });

        if (Array.isArray(row.values)) {
            cols = Math.max(cols, row.values.length - 1);
        }
    }

    for (const modelRow of worksheet.model?.rows ?? []) {
        rows = Math.max(rows, modelRow.number ?? 0);
        for (const modelCell of modelRow.cells ?? []) {
            const colNumber = cellAddressToColumnNumber(modelCell.address);
            cols = Math.max(cols, colNumber);
        }
    }

    return { rows, cols };
}

export function getWorksheetCache(worksheet, cacheMap) {
    let cache = cacheMap.get(worksheet);
    if (!cache) {
        cache = {
            mergeMap: buildMergeMap(worksheet),
        };
        cacheMap.set(worksheet, cache);
    }
    return cache;
}

function buildMergeMap(worksheet) {
    const mergeMap = new Map();
    const merges = Object.values(worksheet._merges ?? {});

    for (const merge of merges) {
        const range = merge.model ?? merge;
        const top = range.top ?? range.tl?.nativeRow;
        const left = range.left ?? range.tl?.nativeCol;
        const bottom = range.bottom ?? range.br?.nativeRow;
        const right = range.right ?? range.br?.nativeCol;

        if (!top || !left || !bottom || !right) continue;

        for (let r = top; r <= bottom; r++) {
            for (let c = left; c <= right; c++) {
                mergeMap.set(`${r}:${c}`, {
                    hidden: r !== top || c !== left,
                    rowspan:
                        r === top && c === left ? bottom - top + 1 : 1,
                    colspan:
                        r === top && c === left ? right - left + 1 : 1,
                });
            }
        }
    }

    return mergeMap;
}
