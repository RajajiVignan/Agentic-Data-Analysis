"use client";

import React from "react";
import { Sparkles, Loader2, Trash2, Filter } from "lucide-react";

type AnalysisPromptProps = {
  prompt: string;
  onPromptChange: (value: string) => void;
  onRun: () => void;
  onNewAnalysis: () => void;
  loading: boolean;
  disabled: boolean;
  error: string | null;
  hasActiveSession: boolean;
};

export function AnalysisPrompt({
  prompt,
  onPromptChange,
  onRun,
  onNewAnalysis,
  loading,
  disabled,
  error,
  hasActiveSession,
}: AnalysisPromptProps) {
  return (
    <div className="space-y-4">
      {/* Active session indicator */}
      {hasActiveSession && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-indigo-50 border border-indigo-100 rounded-lg">
          <div className="flex items-center gap-2 text-xs text-indigo-700">
            <Filter size={12} />
            Follow-up mode — ask a follow-up question or start fresh
          </div>
          <button
            onClick={onNewAnalysis}
            className="flex items-center gap-1 text-xs font-medium text-red-600 hover:text-red-700 transition-colors"
          >
            <Trash2 size={12} />
            New Analysis
          </button>
        </div>
      )}

      {/* Prompt input */}
      <div className="relative group">
        <div className="absolute -inset-1 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-2xl blur opacity-25 group-hover:opacity-50 transition duration-1000"></div>
        <div className="relative bg-white p-2 rounded-2xl border border-slate-200 shadow-sm flex items-center gap-2">
          <div className="p-3 text-indigo-500">
            <Sparkles size={24} />
          </div>
          <input
            className="flex-1 py-3 px-2 outline-none text-slate-700 placeholder:text-slate-400"
            placeholder={
              hasActiveSession
                ? "Ask a follow-up question... (e.g., 'filter by region APAC', 'drill down', 'group by product')"
                : "Ask your data a question... (e.g., 'Compare revenue between these files')"
            }
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
            {loading ? <Loader2 className="animate-spin" size={18} /> : hasActiveSession ? "Follow Up" : "Run Analysis"}
          </button>
        </div>
      </div>
      {error && <p className="text-sm text-red-600">{error}</p>}
    </div>
  );
}
