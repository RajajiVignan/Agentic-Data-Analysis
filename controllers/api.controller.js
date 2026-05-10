
const { exec } = require("child_process");
const { promisify } = require("util");
const execPromise = promisify(exec);
const { 
  parseRows, parseJsonRows, profileRows, buildKpis, buildTrend, buildSegments, buildSql, makeNarrative, makeDashboardTitle, makeRecommendations 
} = require("../utils/data-processor");

const fs = require("node:fs/promises");
const path = require("node:path");
const crypto = require("node:crypto");

let datasets = new Map();
let connections = new Map();
let UPLOAD_DIR = "";
let openai = null;

function setDependencies(deps) {
  datasets = deps.datasets;
  connections = deps.connections;
  UPLOAD_DIR = deps.UPLOAD_DIR;
  openai = deps.openai;
}

async function handleUpload(req, res) {
  const contentType = req.headers["content-type"] || "";
  const body = await readBody(req);
  const upload = parseUpload(body, contentType);

  if (!upload) {
    return sendJson(res, 400, { error: "Upload a CSV or JSON file using multipart/form-data." });
  }

  const id = crypto.randomUUID();
  const safeName = upload.filename.replace(/[^a-z0-9._-]/gi, "_");
  const filePath = path.join(UPLOAD_DIR, `${id}-${safeName}`);
  await fs.writeFile(filePath, upload.content);

  const text = upload.content.toString("utf8");
  let rows;
  try {
    rows = parseRows(text, upload.filename);
  } catch (error) {
    return sendJson(res, 400, { error: error.message });
  }
  if (rows.length < 2) {
    return sendJson(res, 400, { error: "The file needs a header row and at least one data row." });
  }

  const profile = profileRows(rows);
  const dataset = {
    id,
    filename: upload.filename,
    filePath,
    profile,
    rows: rows.slice(1).map((row) => Object.fromEntries(profile.columns.map((col, idx) => [col.name, row[idx] || ""]))),
  };
  datasets.set(id, dataset);

  sendJson(res, 201, {
    datasetId: id,
    filename: upload.filename,
    profile,
  });
}

async function handleAnalyze(req, res) {
  const body = await readJson(req);
  const dataset = datasets.get(body.datasetId);

  if (!dataset) {
    return sendJson(res, 404, { error: "Upload a dataset before running analysis." });
  }

  try {
    const answer = await runAgentAnalysis(dataset, body.prompt || "");
    sendJson(res, 200, answer);
  } catch (error) {
    console.error("Analysis error:", error);
    sendJson(res, 500, { error: "AI Agent failed to analyze data: " + error.message });
  }
}

async function handleConnectSource(req, res) {
  const body = await readJson(req);
  const source = body.source || "Warehouse";
  const id = crypto.randomUUID();
  const filename = `${source.toLowerCase().replace(/[^a-z0-9]+/g, "_")}_sample`;
  const rows = [
    ["month", "segment", "revenue", "customers", "churn_risk"],
    ["2026-01", "Enterprise", "124000", "42", "3.2"],
    ["2026-01", "Mid-market", "86000", "118", "5.1"],
    ["2026-01", "SMB", "39000", "314", "8.4"],
    ["2026-02", "Enterprise", "138500", "45", "2.8"],
    ["2026-02", "Mid-market", "91000", "124", "4.9"],
    ["2026-02", "SMB", "41000", "329", "8.9"],
    ["2026-03", "Enterprise", "151200", "49", "2.6"],
    ["2026-03", "Mid-market", "97000", "131", "4.6"],
    ["2026-03", "SMB", "40500", "337", "9.3"],
  ];
  const profile = profileRows(rows);
  const dataset = {
    id,
    filename,
    filePath: null,
    profile,
    rows: rows.slice(1).map((row) => Object.fromEntries(profile.columns.map((col, idx) => [col.name, row[idx] || ""]))),
  };
  datasets.set(id, dataset);
  connections.set(id, { source, connectedAt: new Date().toISOString() });

  sendJson(res, 201, {
    datasetId: id,
    filename,
    source,
    profile,
  });
}

async function runAgentAnalysis(dataset, prompt) {
  const columns = dataset.profile.columns;
  const numericColumns = columns.filter((column) => column.type === "number");
  const dateColumn = columns.find((column) => column.type === "date");
  const categoryColumn = columns.find((column) => column.type === "text");

  let metricColumn = null;
  let sql = "";
  let narrative = "";
  let aiPythonCode = null;
  let aiSegments = null;

  if (process.env.NVIDIA_API_KEY && openai) {
    try {
      const plotPath = path.join(UPLOAD_DIR, "plots", dataset.id + "_plot.png");
      const systemPrompt = "You are a BI Analyst Agent. \\n" +
        "Dataset: " + dataset.filename + "\\n" +
        "Columns: " + JSON.stringify(columns) + "\\n" +
        "Dataset Rows: " + dataset.profile.rowCount + "\\n" +
        "Sample Data: " + JSON.stringify(dataset.rows.slice(0, 10)) + "\\n" +
        "\\n" +
        "Your goal is to analyze the dataset based on the user's prompt.\\n" +
        "You must return a JSON response with:\\n" +
        "1. \\\"metricColumn\\\": The name of the most relevant numeric column for the prompt.\\n" +
        "2. \\\"sql\\\": A valid SQL query to extract the trend (group by month).\\n" +
        "3. \\\"narrative\\\": A brief, professional business insight based on the user prompt.\\n" +
        "4. \\\"python_code\\\": A complete Python script using pandas and matplotlib/seaborn to create a visualization. \\n" +
        "   - The script must read data from: " + dataset.filePath + "\\n" +
        "   - It must save the resulting plot as: " + plotPath + "\\n" +
        "   - Ensure the script handles CSV/JSON format correctly based on filename extension.\\n" +
        "5. \\\"segmentData\\\": An array of objects [{label: \\\"segment name\\\", value: number}] for the top 5 segments.";

      const completion = await openai.chat.completions.create({
        model: "meta/llama-3.1-70b-instruct",
        messages: [
          { role: "system", content: systemPrompt },
          { role: "user", content: prompt || "Summarize this dataset." },
        ],
        response_format: { type: "json_object" },
      });

      const aiResult = JSON.parse(completion.choices[0].message.content);
      metricColumn = columns.find(col => col.name === aiResult.metricColumn);
      sql = aiResult.sql;
      narrative = aiResult.narrative;
      aiPythonCode = aiResult.python_code;
      aiSegments = aiResult.segmentData;
    } catch (e) {
      console.error("AI call failed, falling back to deterministic logic:", e);
    }
  }

  if (!metricColumn) {
    metricColumn = pickMetricColumn(prompt, numericColumns);
  }
  if (!sql) {
    sql = buildSql(dataset.filename, dateColumn, metricColumn, categoryColumn);
  }
  if (!narrative) {
    narrative = makeNarrative(prompt, metricColumn, dateColumn, categoryColumn, buildKpis(dataset.rows, metricColumn, categoryColumn));
  }

  const kpis = buildKpis(dataset.rows, metricColumn, categoryColumn);
  const segments = aiSegments || buildSegments(dataset.rows, categoryColumn, metricColumn);

  let plotUrl = null;
  if (aiPythonCode) {
    try {
      const pyFile = path.join(UPLOAD_DIR, "plots", dataset.id + ".py");
      await fs.mkdir(path.join(UPLOAD_DIR, "plots"), { recursive: true });
      await fs.writeFile(pyFile, aiPythonCode);
      
      await execPromise(`python3 ${pyFile}`);
      
      const plotFileName = dataset.id + "_plot.png";
      plotUrl = "/plots/" + plotFileName;
    } catch (err) {
      console.error("Python execution failed:", err);
      plotUrl = null;
    }
  }

  return {
    question: prompt,
    dataset: {
      id: dataset.id,
      filename: dataset.filename,
      rowCount: dataset.profile.rowCount,
      columns,
    },
    notebook: [
      {
        title: "Data understanding",
        body: "Profiled " + dataset.profile.rowCount + " rows across " + columns.length + " columns. The agent selected " + (metricColumn ? metricColumn.name : "row count") + " as the main measure.",
      },
      {
        title: "Generated SQL",
        code: sql,
      },
      {
        title: "Answer",
        body: narrative,
      },
    ],
    dashboard: {
      title: makeDashboardTitle(prompt),
      kpis,
      plotUrl, 
      segments,
      recommendations: makeRecommendations(metricColumn, categoryColumn, kpis),
    },
  };
}

function pickMetricColumn(prompt, numericColumns) {
  if (!numericColumns.length) return null;
  const lowerPrompt = prompt.toLowerCase();
  return (
    numericColumns.find((column) => lowerPrompt.includes(column.name.toLowerCase())) ||
    numericColumns.find((column) => /revenue|sales|amount|price|total|arr|mrr/i.test(column.name)) ||
    numericColumns[0]
  );
}

async function serveStatic(req, res) {
  const ROOT = __dirname + '/..'; 
  let url = req.url === "/" ? "/index.html" : req.url;
  
  // Handle plots directory specifically
  if (url.startsWith("/plots/")) {
    const plotPath = path.join(ROOT, "uploads", "plots", url.replace("/plots/", ""));
    try {
      const content = await fs.readFile(plotPath);
      res.writeHead(200, { "Content-Type": "image/png" });
      res.end(content);
      return;
    } catch (e) {
      return sendJson(res, 404, { error: "Plot not found" });
    }
  }

  const filePath = path.normalize(path.join(ROOT, decodeURIComponent(url)));
  if (!filePath.startsWith(ROOT)) return sendJson(res, 403, { error: "Forbidden" });

  try {
    const content = await fs.readFile(filePath);
    res.writeHead(200, { "Content-Type": "application/octet-stream" });
    res.end(content);
  } catch {
    sendJson(res, 404, { error: "Not found" });
  }
}

async function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => resolve(Buffer.concat(chunks)));
    req.on("error", reject);
  });
}

async function readJson(req) {
  const body = await readBody(req);
  return JSON.parse(body.toString("utf8") || "{}");
}

function sendJson(res, status, data) {
  res.writeHead(status, { "Content-Type": "application/json; charset=utf-8" });
  res.end(JSON.stringify(data));
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

module.exports = {
  setDependencies, handleUpload, handleAnalyze, handleConnectSource, serveStatic, sendJson, readJson
};
