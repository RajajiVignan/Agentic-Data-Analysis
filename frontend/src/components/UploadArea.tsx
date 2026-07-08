
import React from "react";
import { FileText, CheckCircle2, Database, RefreshCw, Loader2 } from "lucide-react";
import type { Dataset } from "@/lib/api";

type UploadAreaProps = {
  datasets: Dataset[];
  selectedDatasetIds: string[];
  onToggleDataset: (id: string) => void;
  onFileUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onConnectDatabase: () => void;
  onRefreshDataset?: (id: string) => Promise<void>;
  refreshingId?: string | null;
  uploadLoading: boolean;
};

export function UploadArea({
  datasets,
  selectedDatasetIds,
  onToggleDataset,
  onFileUpload,
  onConnectDatabase,
  onRefreshDataset,
  refreshingId,
  uploadLoading,
}: UploadAreaProps) {
  return (
    <div className="bg-white p-6 rounded-2xl border border-slate-200 shadow-sm space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <h3 className="font-bold text-slate-800 flex items-center gap-2">
            <FileText size={18} className="text-indigo-500 shrink-0" />
            <span className="truncate">Active Datasets</span>
          </h3>
          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={onConnectDatabase}
              className="cursor-pointer bg-white text-indigo-600 border border-indigo-200 px-3 sm:px-4 py-2 rounded-lg text-xs font-medium hover:bg-indigo-50 hover:border-indigo-300 transition-colors flex items-center gap-1.5"
            >
              <Database size={14} />
              <span className="hidden sm:inline">Connect </span>Database
            </button>
            <label className="cursor-pointer bg-indigo-600 text-white px-3 sm:px-4 py-2 rounded-lg text-xs font-medium hover:bg-indigo-700 transition-colors">
              Upload New
              <input type="file" className="hidden" accept=".csv,.json" onChange={onFileUpload} disabled={uploadLoading} />
            </label>
          </div>
        </div>

      <div className="flex flex-wrap gap-3">
        {datasets.length === 0 ? (
          <div className="w-full flex flex-col items-center justify-center py-6 text-center">
            <p className="text-sm text-slate-400 italic mb-3">No datasets available. Upload a file or connect a database.</p>
            <button
              onClick={onConnectDatabase}
              className="px-4 py-2 text-xs font-medium text-indigo-600 bg-indigo-50 border border-indigo-200 rounded-lg hover:bg-indigo-100 transition-colors flex items-center gap-1.5"
            >
              <Database size={14} />
              Browse Connections
            </button>
          </div>
        ) : (
          datasets.map((ds) => (
            <div
              key={ds.id}
              onClick={() => onToggleDataset(ds.id)}
              className={`px-3 py-2 rounded-xl border cursor-pointer transition-all flex items-center gap-2 text-xs font-medium ${
                selectedDatasetIds.includes(ds.id)
                  ? "bg-indigo-50 border-indigo-500 text-indigo-700 shadow-sm"
                  : "bg-white border-slate-200 text-slate-600 hover:border-slate-300"
              }`}
            >
              {selectedDatasetIds.includes(ds.id) ? (
                <CheckCircle2 size={14} />
              ) : (
                <div className="w-3.5 h-3.5 rounded-full border-2 border-slate-300" />
              )}
              {ds.filename}
              {ds.liveDb && onRefreshDataset && (
                <button
                  onClick={(e) => { e.stopPropagation(); onRefreshDataset(ds.id); }}
                  disabled={refreshingId === ds.id}
                  className="ml-1 p-1 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 rounded transition-colors"
                  title="Refresh data from database"
                >
                  {refreshingId === ds.id ? (
                    <Loader2 size={12} className="animate-spin" />
                  ) : (
                    <RefreshCw size={12} />
                  )}
                </button>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
