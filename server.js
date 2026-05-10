
const http = require("node:http");
const fs = require("node:fs/promises");
const path = require("node:path");
const OpenAI = require("openai");
require("dotenv").config();

const apiController = require("./controllers/api.controller");

const PORT = process.env.PORT || 3000;
const HOST = process.env.HOST || "127.0.0.1";
const ROOT = __dirname;
const UPLOAD_DIR = path.join(ROOT, "uploads");
const datasets = new Map();
const connections = new Map();

const openai = new OpenAI({
  apiKey: process.env.NVIDIA_API_KEY,
  baseURL: process.env.NVIDIA_API_BASE_URL || "https://integrate.api.nvidia.com/v1",
});

const mimeTypes = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
};

// Set shared state in controller
apiController.setDependencies({
  datasets,
  connections,
  UPLOAD_DIR,
  openai // Pass openai instance if needed in the future
});


const server = http.createServer(async (req, res) => {
  // Add CORS headers to allow the frontend to connect
  res.setHeader("Access-Control-Allow-Origin", "*");
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Content-Type");

  if (req.method === "OPTIONS") {
    res.writeHead(204);
    res.end();
    return;
  }

  try {
    if (req.method === "GET" && req.url === "/api/health") {

      return apiController.sendJson(res, 200, {
        ok: true,
        service: "InsightPilot API",
        agentMode: process.env.OPENAI_API_KEY ? "llm-ready" : "local-deterministic",
        datasets: datasets.size,
        connections: connections.size,
      });
    }

    if (req.method === "GET" && req.url === "/api/datasets") {
      return apiController.sendJson(res, 200, {
        datasets: [...datasets.values()].map((dataset) => ({
          id: dataset.id,
          filename: dataset.filename,
          profile: dataset.profile,
        })),
      });
    }

    if (req.method === "POST" && req.url === "/api/upload") {
      return apiController.handleUpload(req, res);
    }

    if (req.method === "POST" && req.url === "/api/analyze") {
      return apiController.handleAnalyze(req, res);
    }

    if (req.method === "POST" && req.url === "/api/connect-source") {
      return apiController.handleConnectSource(req, res);
    }

    if (req.method === "GET") {
      return apiController.serveStatic(req, res);
    }

    apiController.sendJson(res, 404, { error: "Route not found" });
  } catch (error) {
    console.error(error);
    apiController.sendJson(res, 500, { error: "Unexpected server error" });
  }
});

server.listen(PORT, HOST, async () => {
  await fs.mkdir(UPLOAD_DIR, { recursive: true });
  console.log(`InsightPilot running at http://${HOST}:${PORT}`);
});

module.exports = server;
