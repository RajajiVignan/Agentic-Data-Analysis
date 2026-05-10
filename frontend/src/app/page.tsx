"use client";
import React, { useState, useEffect } from 'react';
import { 
  Upload, 
  Sparkles, 
  Loader2, 
} from 'lucide-react';
import { Sidebar } from '../components/Sidebar';
import { MetricTile, TrendChart, SegmentChart, PythonPlot } from '../components/Charts';

const API_BASE = "http://127.0.0.1:3000/api";

type NotebookStep = {
  title: string;
  body?: string;
  code?: string;
};

type Kpi = {
  label: string;
  value: string;
  change: string;
};

type ChartPoint = {
  label: string;
  value: number;
};

type AnalysisResult = {
  notebook: NotebookStep[];
  dashboard: {
    kpis: Kpi[];
    trend: ChartPoint[];
    segments: ChartPoint[];
    recommendations: string[];
    plotUrl?: string | null;
  };
};

type BackendStatus = "checking" | "online" | "offline";

export default function Workspace() {
  const [activeDatasetId, setActiveDatasetId] = useState<string | null>(null);
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [backendStatus, setBackendStatus] = useState<BackendStatus>("checking");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    checkBackend();
  }, []);

  async function checkBackend() {
    try {
      const res = await fetch(`${API_BASE}/health`);
      if (res.ok) setBackendStatus("online");
      else setBackendStatus("offline");
    } catch {
      setBackendStatus("offline");
    }
  }

  async function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    const formData = new FormData();
    formData.append("file", file);

    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/upload`, { method: "POST", body: formData });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      setActiveDatasetId(data.datasetId);
      const starterPrompt = "Analyze this dataset and suggest key growth drivers.";
      setPrompt(starterPrompt);
      await runAnalysis(starterPrompt, data.datasetId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setLoading(false);
    }
  }

  async function runAnalysis(inputValue: string, datasetId = activeDatasetId) {
    if (!datasetId) {
      setError("Upload a dataset before running analysis.");
      return;
    }
    
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/analyze`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ datasetId, prompt: inputValue }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);
      setResult(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Analysis failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex h-screen bg-slate-50 text-slate-900">
      <Sidebar />
      
      <main className="flex-1 overflow-y-auto">
        <header className="px-8 py-6 flex justify-between items-center border-b border-slate-200 bg-white">
          <div>
            <p className="text-xs font-medium text-slate-400 uppercase tracking-wider">Explore Workspace</p>
            <h1 className="text-2xl font-bold text-slate-900">AI Data Analyst</h1>
          </div>
          <div className="flex items-center gap-3">
            <div className={`px-2 py-1 rounded-full text-[10px] font-bold uppercase flex items-center gap-1.5 ${backendStatus === 'online' ? 'bg-emerald-100 text-emerald-600' : 'bg-red-100 text-red-600'}`}>
              <span className={`w-1.5 h-1.5 rounded-full ${backendStatus === 'online' ? 'bg-emerald-500' : 'bg-red-500'}`} />
              Backend {backendStatus}
            </div>
          </div>
        </header>

        <div className="p-8 max-w-6xl mx-auto space-y-8">
          {/* Prompt Area */}
          <div className="relative group">
            <div className="absolute -inset-1 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-2xl blur opacity-25 group-hover:opacity-50 transition duration-1000"></div>
            <div className="relative bg-white p-2 rounded-2xl border border-slate-200 shadow-sm flex items-center gap-2">
              <div className="p-3 text-indigo-500">
                <Sparkles size={24} />
              </div>
              <input 
                className="flex-1 py-3 px-2 outline-none text-slate-700 placeholder:text-slate-400"
                placeholder="Ask your data a question..."
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
              />
              <button 
                onClick={() => runAnalysis(prompt)}
                disabled={loading || !activeDatasetId}
                className="bg-indigo-600 hover:bg-indigo-700 text-white px-6 py-2 rounded-xl font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              >
                {loading ? <Loader2 className="animate-spin" size={18} /> : "Run Analysis"}
              </button>
            </div>
            {error && (
              <p className="px-5 pb-4 text-sm text-red-600">
                {error}
              </p>
            )}
          </div>
          
          {/* Upload Area */}
          {!activeDatasetId && (
            <div className="flex flex-col items-center justify-center py-20 border-2 border-dashed border-slate-200 rounded-3xl bg-white text-center space-y-4">
              <div className="p-4 bg-indigo-50 text-indigo-600 rounded-full">
                <Upload size={32} />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-slate-900">No Dataset Loaded</h3>
                <p className="text-sm text-slate-500 max-w-xs mx-auto">Upload a CSV or JSON file to start the AI-powered analysis.</p>
              </div>
              <label className="cursor-pointer bg-slate-900 text-white px-6 py-2.5 rounded-xl font-medium hover:bg-slate-800 transition-colors">
                Upload Data
                <input type="file" className="hidden" accept=".csv,.json" onChange={handleFileUpload} />
              </label>
            </div>
          )}

          {/* Results Grid */}
          {result && (
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
              <div className="lg:col-span-1 space-y-6">
                <div className="p-6 bg-white rounded-2xl border border-slate-200 shadow-sm space-y-4">
                  <h3 className="font-bold text-slate-800 flex items-center gap-2">
                    <div className="w-1.5 h-5 bg-indigo-500 rounded-full" />
                    Agent Notebook
                  </h3>
                  <div className="space-y-4">
                    {result.notebook.map((step, i) => (
                      <div key={i} className="p-3 rounded-lg bg-slate-50 border border-slate-100 space-y-2">
                        <span className="text-[10px] font-bold uppercase text-slate-400">Step 0{i+1}: {step.title}</span>
                        <p className="text-xs text-slate-600 leading-relaxed whitespace-pre-wrap">{step.body || step.code}</p>
                      </div>
                    ))}
                  </div>
                </div>
              </div>

              <div className="lg:col-span-2 space-y-8">
                <div className="grid grid-cols-3 gap-4">
                  {result.dashboard.kpis.map((kpi, i) => (
                    <MetricTile 
                      key={i} 
                      label={kpi.label} 
                      value={kpi.value} 
                      change={kpi.change} 
                    />
                  ))}
                </div>
                
                <PythonPlot url={result.dashboard.plotUrl} />
                <TrendChart data={result.dashboard.trend} />
                <SegmentChart 
                  data={result.dashboard.segments} 
                  recommendations={result.dashboard.recommendations} 
                />
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
