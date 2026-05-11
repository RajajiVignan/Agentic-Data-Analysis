
const crypto = require("node:crypto");
const fs = require("node:fs/promises");
const path = require("node:path");

function parseRows(text, filename) {
  if (/\.json$/i.test(filename)) return parseJsonRows(text);
  return parseCsv(text);
}

function parseJsonRows(text) {
  const parsed = JSON.parse(text);
  const records = Array.isArray(parsed) ? parsed : parsed.data || parsed.rows || parsed.records;
  if (!Array.isArray(records) || !records.length || typeof records[0] !== "object") {
    throw new Error("JSON uploads must be an array of objects or contain data, rows, or records.");
  }
  const headers = [...new Set(records.flatMap((record) => Object.keys(record)))];
  const rows = records.map((record) => headers.map((header) => stringifyCell(record[header])));
  return [headers, ...rows];
}

function stringifyCell(value) {
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function cleanCell(value) {
  return stringifyCell(value).trim().replace(/\s+/g, " ");
}

function escapeCsvCell(value) {
  const text = cleanCell(value);
  if (/[",\n\r]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`;
  }
  return text;
}

function objectRowsToCsv(datasetsList) {
  const allColumns = [
    "source_dataset",
    ...new Set(datasetsList.flatMap((dataset) => dataset.profile.columns.map((column) => column.name))),
  ];

  const lines = [allColumns.map(escapeCsvCell).join(",")];

  datasetsList.forEach((dataset) => {
    dataset.rows.forEach((row) => {
      const values = allColumns.map((column) => {
        if (column === "source_dataset") return dataset.filename;
        return row[column] ?? "";
      });
      lines.push(values.map(escapeCsvCell).join(","));
    });
  });

  return lines.join("\n") + "\n";
}

function profileRows(rows) {
  const headers = rows[0].map((header, index) => header.trim() || `column_${index + 1}`);
  const dataRows = rows.slice(1);
  const columns = headers.map((name, index) => {
    const values = dataRows.map((row) => row[index]).filter((value) => value !== undefined && value !== "");
    return {
      name,
      type: inferType(values),
      nonEmpty: values.length,
      sample: values.slice(0, 3),
    };
  });
  return {
    rowCount: dataRows.length,
    columns,
  };
}

function inferType(values) {
  if (!values.length) return "empty";
  const numeric = values.filter((value) => Number.isFinite(Number(value))).length;
  const dates = values.filter((value) => Number.isFinite(new Date(value).getTime()) && /[-/]/.test(value)).length;
  if (numeric / values.length > 0.8) return "number";
  if (dates / values.length > 0.8) return "date";
  return "text";
}

function parseCsv(text) {
  const rows = [];
  let row = [];
  let value = "";
  let quoted = false;

  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    const next = text[index + 1];
    if (char === '"' && quoted && next === '"') {
      value += '"';
      index += 1;
    } else if (char === '"') {
      quoted = !quoted;
    } else if (char === "," && !quoted) {
      row.push(value.trim());
      value = "";
    } else if ((char === "\n" || char === "\r") && !quoted) {
      if (char === "\r" && next === "\n") index += 1;
      row.push(value.trim());
      if (row.some((cell) => cell.length)) rows.push(row);
      row = [];
      value = "";
    } else {
      value += char;
    }
  }

  row.push(value.trim());
  if (row.some((cell) => cell.length)) rows.push(row);
  return rows;
}

function parseUpload(body, contentType) {
  const boundary = contentType.match(/boundary=(.+)$/)?.[1];
  if (!boundary) return null;

  const parts = body.toString("binary").split(`--${boundary}`);
  for (const part of parts) {
    if (!part.includes('name="file"')) continue;
    const filename = part.match(/filename="([^"]+)"/)?.[1];
    const start = part.indexOf("\r\n\r\n");
    if (!filename || start === -1) return null;
    const raw = part.slice(start + 4).replace(/\r\n--$/, "").replace(/\r\n$/, "");
    return {
      filename,
      content: Buffer.from(raw, "binary"),
    };
  }

  return null;
}

function formatNumber(value) {
  if (!Number.isFinite(value)) return "0";
  return new Intl.NumberFormat("en-US", {
    notation: Math.abs(value) >= 1000000 ? "compact" : "standard",
    maximumFractionDigits: 1,
  }).format(value);
}

function quoteId(value) {
  return `"${String(value).replace(/"/g, '""')}"`;
}

function buildKpis(rows, metricColumn, categoryColumn) {
  const values = metricColumn ? rows.map((row) => Number(row[metricColumn.name])).filter(Number.isFinite) : [];
  const total = values.reduce((sum, value) => sum + value, 0);
  const average = values.length ? total / values.length : rows.length;
  const categories = categoryColumn ? new Set(rows.map((row) => row[categoryColumn.name]).filter(Boolean)).size : 0;

  return [
    { label: metricColumn ? `Total ${metricColumn.name}` : "Rows analyzed", value: formatNumber(metricColumn ? total : rows.length), change: "Dataset result" },
    { label: metricColumn ? `Avg ${metricColumn.name}` : "Fields profiled", value: formatNumber(average), change: "Per row" },
    { label: categoryColumn ? `${categoryColumn.name} groups` : "Columns", value: formatNumber(categories || Object.keys(rows[0] || {}).length), change: "Available split" },
  ];
}

function buildTrend(rows, dateColumn, metricColumn) {
  if (!dateColumn || !metricColumn) return [];
  const grouped = new Map();
  rows.forEach((row) => {
    const date = new Date(row[dateColumn.name]);
    const value = Number(row[metricColumn.name]);
    if (!Number.isFinite(date.getTime()) || !Number.isFinite(value)) return;
    const label = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
    grouped.set(label, (grouped.get(label) || 0) + value);
  });
  return [...grouped.entries()].sort(([a], [b]) => a.localeCompare(b)).slice(-8).map(([label, value]) => ({ label, value }));
}

function buildSegments(rows, categoryColumn, metricColumn) {
  if (!categoryColumn) return [];
  const grouped = new Map();
  rows.forEach((row) => {
    const label = row[categoryColumn.name] || "Unknown";
    const value = metricColumn ? Number(row[metricColumn.name]) : 1;
    grouped.set(label, (grouped.get(label) || 0) + (Number.isFinite(value) ? value : 0));
  });
  return [...grouped.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([label, value]) => ({ label, value }));
}

function buildSql(filename, dateColumn, metricColumn, categoryColumn) {
  const metric = metricColumn ? `sum(${quoteId(metricColumn.name)}) as total_${safeSqlName(metricColumn.name)}` : "count(*) as rows";
  const dateExpr = dateColumn ? `date_trunc('month', ${quoteId(dateColumn.name)}) as month` : "'all' as period";
  const category = categoryColumn ? `,\\n  ${quoteId(categoryColumn.name)}` : "";
  const groupBy = categoryColumn ? "1, 2" : "1";
  return `select\\n  ${dateExpr}${category},\\n  ${metric}\\nfrom ${quoteId(filename)}\\ngroup by ${groupBy}\\norder by 1;`;
}

function makeNarrative(prompt, metricColumn, dateColumn, categoryColumn, kpis) {
  const focus = prompt.trim() ? `For "${prompt.trim()}", ` : "";
  const metric = metricColumn ? metricColumn.name : "record volume";
  const time = dateColumn ? ` over ${dateColumn.name}` : "";
  const split = categoryColumn ? ` by ${categoryColumn.name}` : "";
  return `${focus}the agent analyzed ${metric}${time}${split}. The dashboard highlights ${kpis[0].label.toLowerCase()} of ${kpis[0].value} and surfaces the highest-impact segments for follow-up.`;
}

function makeDashboardTitle(prompt) {
  if (/churn/i.test(prompt)) return "Churn Risk Dashboard";
  if (/revenue|sales|arr|mrr/i.test(prompt)) return "Revenue Command Center";
  if (/customer|account/i.test(prompt)) return "Customer Analytics Board";
  return "Dataset Insights Board";
}

function makeRecommendations(metricColumn, categoryColumn, kpis) {
  const metric = metricColumn?.name || "records";
  const category = categoryColumn?.name || "available segments";
  return [
    `Review the top ${category} groups contributing to ${metric}.`,
    `Add business definitions for ${metric} so future prompts stay consistent.`,
    `Publish this board after validating ${kpis[0].label.toLowerCase()} with the data owner.`,
  ];
}

function safeSqlName(value) {
  return String(value).toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "") || "metric";
}

module.exports = {
  parseRows, parseJsonRows, stringifyCell, cleanCell, escapeCsvCell, objectRowsToCsv, profileRows, inferType, parseCsv, parseUpload, formatNumber, quoteId, safeSqlName, buildKpis, buildTrend, buildSegments, buildSql, makeNarrative, makeDashboardTitle, makeRecommendations
}
