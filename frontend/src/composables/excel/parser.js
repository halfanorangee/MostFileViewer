import Papa from "papaparse";
import { normalizeTextEncoding } from "./utils";

export function isCsvFile(extension) {
    return String(extension || "").toLowerCase() === ".csv";
}

export function parseCsvWorkbook(buffer, encoding) {
    const text = decodeCsvText(buffer, encoding);
    const result = Papa.parse(text, {
        delimiter: "",
        dynamicTyping: false,
        header: false,
        skipEmptyLines: false,
    });

    if (result.errors.length > 0) {
        const error = result.errors[0];
        throw new Error(error.message || "CSV 解析失败");
    }

    const rows = result.data.map((row) =>
        Array.isArray(row)
            ? row.map((cell) => String(cell ?? ""))
            : [String(row ?? "")],
    );
    const columnCount = rows.reduce(
        (max, row) => Math.max(max, row.length),
        0,
    );

    return createGridWorkbook([
        createGridWorksheet("CSV", rows, columnCount || 1),
    ]);
}

export async function parseXlsxFallbackWorkbook(buffer, JSZip) {
    const zip = await JSZip.loadAsync(buffer);
    const workbookXml = await readZipText(zip, "xl/workbook.xml");
    if (!workbookXml) return createGridWorkbook([]);

    const workbookRelsXml = await readZipText(
        zip,
        "xl/_rels/workbook.xml.rels",
    );
    const sharedStrings = await parseSharedStrings(zip);
    const workbookDoc = parseXml(workbookXml);
    const rels = parseWorkbookRelationships(workbookRelsXml);
    const worksheets = [];

    for (const sheetNode of workbookDoc.getElementsByTagName("sheet")) {
        const name = sheetNode.getAttribute("name") || "Sheet";
        const relId = getNamespacedAttribute(sheetNode, "id");
        const target = rels.get(relId);
        if (!target) {
            worksheets.push(createGridWorksheet(name, []));
            continue;
        }

        const sheetXml = await readZipText(
            zip,
            normalizeWorkbookRelTarget(target),
        );
        worksheets.push(parseWorksheetXml(name, sheetXml, sharedStrings));
    }

    return createGridWorkbook(worksheets);
}

function decodeCsvText(buffer, encoding) {
    let text = new TextDecoder(normalizeTextEncoding(encoding), {
        fatal: false,
    }).decode(buffer);

    if (text.charCodeAt(0) === 0xfeff) {
        text = text.slice(1);
    }

    return text;
}

async function parseSharedStrings(zip) {
    const xml = await readZipText(zip, "xl/sharedStrings.xml");
    if (!xml) return [];

    const doc = parseXml(xml);
    return Array.from(doc.getElementsByTagName("si")).map((item) =>
        Array.from(item.getElementsByTagName("t"))
            .map((textNode) => textNode.textContent ?? "")
            .join(""),
    );
}

function parseWorkbookRelationships(xml) {
    const rels = new Map();
    if (!xml) return rels;

    const doc = parseXml(xml);
    for (const relNode of doc.getElementsByTagName("Relationship")) {
        rels.set(
            relNode.getAttribute("Id"),
            relNode.getAttribute("Target"),
        );
    }
    return rels;
}

function parseWorksheetXml(name, xml, sharedStrings) {
    if (!xml) return createGridWorksheet(name, []);

    const doc = parseXml(xml);
    const rows = [];
    let maxRow = 0;
    let maxCol = 0;

    for (const rowNode of doc.getElementsByTagName("row")) {
        const rowNumber = Number(rowNode.getAttribute("r")) || maxRow + 1;
        maxRow = Math.max(maxRow, rowNumber);
        const row = rows[rowNumber - 1] ?? [];
        rows[rowNumber - 1] = row;

        for (const cellNode of rowNode.getElementsByTagName("c")) {
            const address = cellNode.getAttribute("r") || "";
            const colNumber =
                cellAddressToColumnNumber(address) || row.length + 1;
            maxCol = Math.max(maxCol, colNumber);
            row[colNumber - 1] = getXmlCellText(cellNode, sharedStrings);
        }
    }

    const normalizedRows = Array.from(
        { length: maxRow },
        (_, index) => rows[index] ?? [],
    );
    return createGridWorksheet(name, normalizedRows, maxCol);
}

function getXmlCellText(cellNode, sharedStrings) {
    const type = cellNode.getAttribute("t");

    if (type === "inlineStr") {
        return Array.from(cellNode.getElementsByTagName("t"))
            .map((node) => node.textContent ?? "")
            .join("");
    }

    const value = firstChildText(cellNode, "v");
    if (type === "s") {
        return sharedStrings[Number(value)] ?? "";
    }
    if (type === "b") {
        return value === "1" ? "TRUE" : "FALSE";
    }

    return value;
}

function firstChildText(node, tagName) {
    const child = node.getElementsByTagName(tagName)[0];
    return child?.textContent ?? "";
}

function parseXml(xml) {
    return new DOMParser().parseFromString(xml, "application/xml");
}

async function readZipText(zip, path) {
    const file = zip.file(path.replace(/^\//, ""));
    return file ? await file.async("text") : "";
}

function normalizeWorkbookRelTarget(target) {
    const normalized = String(target ?? "").replace(/^\//, "");
    return normalized.startsWith("xl/") ? normalized : `xl/${normalized}`;
}

function getNamespacedAttribute(node, localName) {
    return (
        node.getAttribute(`r:${localName}`) ||
        node.getAttributeNS(
            "http://schemas.openxmlformats.org/officeDocument/2006/relationships",
            localName,
        ) ||
        ""
    );
}

function cellAddressToColumnNumber(address) {
    const letters = String(address ?? "").match(/^[A-Z]+/i)?.[0] ?? "";
    let column = 0;

    for (const letter of letters.toUpperCase()) {
        column = column * 26 + letter.charCodeAt(0) - 64;
    }

    return column;
}

export function createGridWorkbook(worksheets) {
    return {
        worksheets,
        getWorksheet(name) {
            return (
                this.worksheets.find((sheet) => sheet.name === name) ?? null
            );
        },
    };
}

export function createGridWorksheet(name, rows, columnCount) {
    const resolvedColumnCount =
        columnCount ??
        rows.reduce((max, row) => Math.max(max, row.length), 0);
    return {
        __previewGrid: true,
        name,
        rowCount: rows.length,
        columnCount: resolvedColumnCount,
        rows,
    };
}

export function getWorkbookSheetNames(workbook, fallbackWorkbook) {
    const names = [];
    for (const worksheet of workbook?.worksheets ?? []) {
        if (worksheet?.name && !names.includes(worksheet.name)) {
            names.push(worksheet.name);
        }
    }
    for (const worksheet of fallbackWorkbook?.worksheets ?? []) {
        if (worksheet?.name && !names.includes(worksheet.name)) {
            names.push(worksheet.name);
        }
    }
    return names;
}
