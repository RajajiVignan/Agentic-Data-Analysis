"use client";

import React from "react";
import { FileText, CheckCircle2 } from "lucide-react";
import type { Dataset } from "@/lib/api";

type UploadAreaProps = {
  datasets: Dataset[];
  selectedDatasetIds: string[];
  onToggleDataset: (id: string) => void;
  onFileUpload: (e: React.ChangeEvent<HTMLInputElement>) => void;
  uploadLoading: boolean;
};

export function UploadArea({
  datasets,
  selectedDatasetIds,
  onToggleDataset,
  onFileUpload,
  uploadLoading,
}: UploadAreaProps) {
  return (
    <div className="bg-white p-6 rounded-2xl border border-slate-200 shadow-sm space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-bold text-slate-800 flex items-center gap-2">
          <FileText size={18} className="text-indigo-500" />
          Active Datasets
        </h3>
        <label className="cursor-pointer bg-indigo-600 text-white px-4 py-2 rounded-lg text-xs font-medium hover:bg-indigo-700 transition-colors">
          Upload New
          <input type="file" className="hidden" accept=".csv,.json" onChange={onFileUpload} disabled={uploadLoading} />
        </label>
      </div>

      <div className="flex flex-wrap gap-3">
        {datasets.length === 0 ? (
          <p className="text-sm text-slate-400 italic">No datasets available. Please upload a file.</p>
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
            </div>
          ))
        )}
      </div>
    </div>
  );
}
