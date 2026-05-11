
const { exec } = require("child_process");
const { promisify } = require("util");
const execPromise = promisify(exec);
const { 
  parseRows, parseJsonRows, profileRows, buildKpis, buildTrend, buildSegments, buildSql, makeNarrative, makeDashboardTitle, makeRecommendations, parseUpload, objectRowsToCsv
} = require("../utils/data-processor");

const fs = require("node:fs/promises");
const path = require("node:path");
const crypto = require("node:crypto");

function rowToObject(columns, row) {
  const obj = {};
  for (let i = 0; i < columns.length; i++) {
    obj[columns[i].name] = row[i] || "";
  }
  return obj;
}

let datasets = new Map();
let connections = new Map();
let UPLOAD_DIR = "";
let openai = null;
let mimeTypes = {};

function setDependencies(deps) {
  datasets = deps.datasets;
  connections = deps.connections;
  UPLOAD_DIR = deps.UPLOAD_DIR;
  openai = deps.openai;
  mimeTypes = deps.mimeTypes || {};
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
    rows: rows.slice(1).map((row) => rowToObject(profile.columns, row)),
  };
  datasets.set(id, dataset);

  // Run automated health check
  let healthCheck = null;
  try {
    healthCheck = await runAgentAnalysis([dataset], "Perform a comprehensive health check on this dataset. Look for anomalies, outliers, missing values, surprising correlations, and any data quality issues. Provide insights in a structured format.");
  } catch (error) {
    console.error("Health check failed:", error);
    // Optionally, we can set healthCheck to an error object
    healthCheck = { error: "Health check failed: " + error.message };
  }
  // Update dataset with healthCheck
  dataset.healthCheck = healthCheck;
  datasets.set(id, dataset);

  sendJson(res, 201, {
    datasetId: id,
    filename: upload.filename,
    profile,
    healthCheck, // Include in response
  });
}

async function handleAnalyze(req, res) {
  const body = await readJson(req);
  const datasetIds = Array.isArray(body.datasetIds) ? body.datasetIds : (body.datasetId ? [body.datasetId] : []);
  
  if (datasetIds.length === 0) {
    return sendJson(res, 400, { error: "No dataset IDs provided for analysis." });
  }

  const datasetsToAnalyze = datasetIds.map(id => datasets.get(id)).filter(Boolean);

  if (datasetsToAnalyze.length === 0) {
    return sendJson(res, 404, { error: "One or more requested datasets were not found." });
  }

  try {
    const answer = await runAgentAnalysis(datasetsToAnalyze, body.prompt || "");
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
    rows: rows.slice(1).map((row) => rowToObject(profile.columns, row)),
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

async function handleExportCsv(req, res) {
  const url = new URL(req.url, "http://localhost");
  const ids = (url.searchParams.get("datasetIds") || url.searchParams.get("datasetId") || "")
    .split(",")
    .map((id) => id.trim())
    .filter(Boolean);

  if (ids.length === 0) {
    return sendJson(res, 400, { error: "Provide datasetIds to export." });
  }

  const datasetsToExport = ids.map((id) => datasets.get(id)).filter(Boolean);
  if (datasetsToExport.length === 0) {
    return sendJson(res, 404, { error: "No matching datasets found for export." });
  }

  const csv = objectRowsToCsv(datasetsToExport);
  const filename = datasetsToExport.length === 1
    ? `${safeDownloadName(datasetsToExport[0].filename)}-cleaned.csv`
    : "insightpilot-cleaned-data.csv";

  res.writeHead(200, {
    "Content-Type": "text/csv; charset=utf-8",
    "Content-Disposition": `attachment; filename="${filename}"`,
  });
  res.end(csv);
}

function safeDownloadName(value) {
  return String(value)
    .replace(/\.[^.]+$/, "")
    .replace(/[^a-z0-9._-]+/gi, "_")
    .replace(/^_+|_+$/g, "") || "dataset";
}

async function runAgentAnalysis(datasetsList, prompt) {
  // We use the first dataset as the primary for basic KPI/Notebook metadata
  const primary = datasetsList[0];
  const columns = primary.profile.columns;
  const numericColumns = columns.filter((column) => column.type === "number");
  const dateColumn = columns.find((column) => column.type === "date");
  const categoryColumn = columns.find((column) => column.type === "text");

  let metricColumn = null;
  let sql = "";
  let narrative = "";
  let aiPythonCode = null;
  let aiSegments = null;

  // Prepare Dataset Context for AI
  const datasetsContext = datasetsList.map(d => ({
    filename: d.filename,
    filePath: d.filePath,
    columns: d.profile.columns,
    rowCount: d.profile.rowCount,
    sample: d.rows.slice(0, 5)
  }));

  if (process.env.NODE_ENV !== "test" && process.env.NVIDIA_API_KEY && openai) {
    try {
      // Visualizations are saved using the ID of the first dataset in the group
      const plotPath = path.join(UPLOAD_DIR, "plots", primary.id + "_plot.png");
      const systemPrompt = "You are a BI Analyst Agent specializing in Comparative Analysis. \\n" +
        "Available Datasets: " + JSON.stringify(datasetsContext, null, 2) + "\\n" +
        "\\n" +
        "Your goal is to analyze the dataset(s) based on the user's prompt.\\n" +
        "If multiple datasets are provided, you should look for common keys (like date, id, or category) to merge them and calculate la growth rates, deltas, or shifts.\\n" +
        "You must return a JSON response with:\\n" +
        "1. \\\"metricColumn\\\": The most relevant numeric column for the primary dataset.\\n" +
        "2. \\\"sql\\\": A valid SQL query to extract the trend (group by month).\\n" +
        "3. \\\"narrative\\\": A brief, professional business insight. If comparing, explicitly mention the difference/growth between files.\\n" +
        "4. \\\"python_code\\\": A complete Python script using pandas and matplotlib/seaborn. \\n" +
        "   - YOU MUST LOAD ALL DATASETS using the paths provided in the Available Datasets section.\\n" +
        "   - SAVE the resulting plot as: " + plotPath + "\\n" +
        "   - Use a high-contrast style (e.g., seaborn-v0_8) and ensure labels are clear.\\n" +
        "5. \\\"segmentData\\\": An array of objects [{label: \\\"segment name\\\", value: number}] for the top 5 segments.";

      const completion = await openai.chat.completions.create({
        model: "meta/llama-3.1-70b-instruct",
        messages: [
          { role: "system", content: systemPrompt },
          { role: "user", content: prompt || "Compare these datasets and summarize findings." },
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
    sql = buildSql(primary.filename, dateColumn, metricColumn, categoryColumn);
  }
  if (!narrative) {
    narrative = makeNarrative(prompt, metricColumn, dateColumn, categoryColumn, buildKpis(primary.rows, metricColumn, categoryColumn));
  }

  const kpis = buildKpis(primary.rows, metricColumn, categoryColumn);
  const trend = buildTrend(primary.rows, dateColumn, metricColumn);
  const segments = aiSegments || buildSegments(primary.rows, categoryColumn, metricColumn);

  let plotUrl = null;
  if (aiPythonCode) {
    try {
      const pyFile = path.join(UPLOAD_DIR, "plots", primary.id + ".py");
      await fs.mkdir(path.join(UPLOAD_DIR, "plots"), { recursive: true });
      await fs.writeFile(pyFile, aiPythonCode);
      
      await execPromise(`python3 ${pyFile}`);
      
      const plotFileName = primary.id + "_plot.png";
      plotUrl = "/plots/" + plotFileName;
    } catch (err) {
      console.error("Python execution failed:", err);
      plotUrl = null;
    }
  }

  return {
    question: prompt,
    dataset: {
      id: primary.id,
      filename: primary.filename,
      rowCount: primary.profile.rowCount,
      columns,
    },
    notebook: [
      {
        title: "Data understanding",
        body: `Analyzing ${datasetsList.length} dataset(s). Primary: ${primary.filename} (${primary.profile.rowCount} rows). Agent selected ${metricColumn ? metricColumn.name : "row count"} as the main measure.`,
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
      trend,
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
    numericColumns.find((column) => /revenue|sales/i.test(column.name)) ||
    numericColumns[0]
  );
}

async function serveStatic(req, res) {
  const ROOT = __dirname + '/..'; 
  let url = req.url === "/" ? "/index.html" : req.url;
  
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
    const ext = path.extname(filePath).toLowerCase();
    const contentType = mimeTypes[ext] || "application/octet-stream";
    res.writeHead(200, { "Content-Type": contentType });
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
  const text = body.toString("utf8").trim();
  return JSON.parse(text || "{}");
}

function sendJson(res, status, data) {
  res.writeHead(status, { "Content-Type": "application/json; charset=utf-8" });
  res.end(JSON.stringify(data));
}

module.exports = {
  setDependencies, handleUpload, handleAnalyze, handleConnectSource, handleExportCsv, serveStatic, sendJson, readJson
};
