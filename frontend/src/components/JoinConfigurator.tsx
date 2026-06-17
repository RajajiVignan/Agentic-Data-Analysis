"use client";

import React, { useState, useEffect } from "react";
import {
  ArrowRight,
  GitMerge,
  Plus,
  X,
} from "lucide-react";
import { joinDatasets } from "@/lib/api";
import type { Dataset } from "@/lib/api";

type JoinConfig = {
  leftDatasetId: string;
  rightDatasetId: string;
  leftKey: string;
  rightKey: string;
  joinType: "inner" | "left" | "right" | "outer";
};

type Props = {
  datasets: Dataset[];
  onJoinComplete: (datasetId: string) => void;
};

export function JoinConfigurator({ datasets, onJoinComplete }: Props) {
  const [config, setConfig] = useState<JoinConfig>({
    leftDatasetId: "",
    rightDatasetId: "",
    leftKey: "",
    rightKey: "",
    joinType: "inner",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const leftDS = datasets.find((d) => d.id === config.leftDatasetId);
  const rightDS = datasets.find((d) => d.id === config.rightDatasetId);

  const leftColumns = leftDS?.profile?.columns ?? [];
  const rightColumns = rightDS?.profile?.columns ?? [];

  const canJoin =
    config.leftDatasetId &&
    config.rightDatasetId &&
    config.leftKey &&
    config.rightKey &&
    config.leftDatasetId !== config.rightDatasetId;

  async function handleJoin() {
    if (!canJoin) return;
    setLoading(true);
    setError(null);
    try {
      const result = await joinDatasets(config);
      onJoinComplete(result.datasetId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Join failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="bg-white rounded-2xl border border-slate-200 shadow-sm p-5 space-y-4">
      <div className="flex items-center gap-2">
        <GitMerge size={18} className="text-indigo-500" />
        <h3 className="text-sm font-semibold text-slate-800">Cross-Dataset Join</h3>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-[1fr_auto_1fr] gap-4 items-start">
        {/* Left dataset */}
        <div className="space-y-2">
          <label className="text-xs font-medium text-slate-500">Left Dataset</label>
          <select
            value={config.leftDatasetId}
            onChange={(e) =>
              setConfig({ ...config, leftDatasetId: e.target.value, leftKey: "" })
            }
            className="w-full p-2 text-sm border border-slate-200 rounded-lg bg-white"
          >
            <option value="">Select...</option>
            {datasets.map((d) => (
              <option key={d.id} value={d.id}>
                {d.filename}
              </option>
            ))}
          </select>
          {leftColumns.length > 0 && (
            <select
              value={config.leftKey}
              onChange={(e) => setConfig({ ...config, leftKey: e.target.value })}
              className="w-full p-2 text-sm border border-slate-200 rounded-lg bg-white"
            >
              <option value="">Join key column...</option>
              {leftColumns.map((c) => (
                <option key={c.name} value={c.name}>
                  {c.name} ({c.type})
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Join type */}
        <div className="flex flex-col items-center gap-2 pt-6">
          <select
            value={config.joinType}
            onChange={(e) =>
              setConfig({
                ...config,
                joinType: e.target.value as JoinConfig["joinType"],
              })
            }
            className="p-1.5 text-xs border border-slate-200 rounded-lg bg-white font-medium"
          >
            <option value="inner">INNER</option>
            <option value="left">LEFT</option>
            <option value="right">RIGHT</option>
            <option value="outer">OUTER</option>
          </select>
          <ArrowRight size={16} className="text-slate-400" />
        </div>

        {/* Right dataset */}
        <div className="space-y-2">
          <label className="text-xs font-medium text-slate-500">Right Dataset</label>
          <select
            value={config.rightDatasetId}
            onChange={(e) =>
              setConfig({ ...config, rightDatasetId: e.target.value, rightKey: "" })
            }
            className="w-full p-2 text-sm border border-slate-200 rounded-lg bg-white"
          >
            <option value="">Select...</option>
            {datasets.map((d) => (
              <option key={d.id} value={d.id}>
                {d.filename}
              </option>
            ))}
          </select>
          {rightColumns.length > 0 && (
            <select
              value={config.rightKey}
              onChange={(e) => setConfig({ ...config, rightKey: e.target.value })}
              className="w-full p-2 text-sm border border-slate-200 rounded-lg bg-white"
            >
              <option value="">Join key column...</option>
              {rightColumns.map((c) => (
                <option key={c.name} value={c.name}>
                  {c.name} ({c.type})
                </option>
              ))}
            </select>
          )}
        </div>
      </div>

      <button
        onClick={handleJoin}
        disabled={!canJoin || loading}
        className="w-full py-2 px-4 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
      >
        {loading ? (
          <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
        ) : (
          <>
            <GitMerge size={14} />
            Run Join
          </>
        )}
      </button>

      {error && (
        <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-xs text-red-600">
          {error}
        </div>
      )}
    </div>
  );
}
