"use client";

import React from "react";
import { Sparkles, Loader2 } from "lucide-react";

type AnalysisPromptProps = {
  prompt: string;
  onPromptChange: (value: string) => void;
  onRun: () => void;
  loading: boolean;
  disabled: boolean;
  error: string | null;
};

export function AnalysisPrompt({
  prompt,
  onPromptChange,
  onRun,
  loading,
  disabled,
  error,
}: AnalysisPromptProps) {
  return (
    <div className="relative group">
      <div className="absolute -inset-1 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-2xl blur opacity-25 group-hover:opacity-50 transition duration-1000"></div>
      <div className="relative bg-white p-2 rounded-2xl border border-slate-200 shadow-sm flex items-center gap-2">
        <div className="p-3 text-indigo-500">
          <Sparkles size={24} />
        </div>
        <input
          className="flex-1 py-3 px-2 outline-none text-slate-700 placeholder:text-slate-400"
          placeholder="Ask your data a question... (e.g., 'Compare revenue between these files')"
          value={prompt}
          onChange={(e) => onPromptChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !loading) onRun();
          }}
        />
        <button
          onClick={onRun}
          disabled={disabled}
          className="bg-indigo-600 hover:bg-indigo-700 text-white px-6 py-2 rounded-xl font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
        >
          {loading ? <Loader2 className="animate-spin" size={18} /> : "Run Analysis"}
        </button>
      </div>
      {error && <p className="px-5 pb-4 text-sm text-red-600">{error}</p>}
    </div>
  );
}
