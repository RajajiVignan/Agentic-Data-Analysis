"use client";

import React, { useState } from "react";
import {
  Database,
  Server,
  Cloud,
  CheckCircle2,
  XCircle,
  Loader2,
  Trash2,
  ChevronRight,
  Shield,
  Zap,
} from "lucide-react";
import type { ConnectionConfig } from "@/lib/api";

type Provider = {
  id: string;
  name: string;
  icon: React.ReactNode;
  description: string;
  color: string;
  defaultPort: string;
  fields: FieldSpec[];
};

type FieldSpec = {
  key: keyof ConnectionConfig;
  label: string;
  type: "text" | "password" | "checkbox";
  placeholder?: string;
  required?: boolean;
  help?: string;
};

const PROVIDERS: Provider[] = [
  {
    id: "postgresql",
    name: "PostgreSQL",
    icon: <Database size={24} />,
    description: "Connect to a PostgreSQL database",
    color: "bg-blue-500",
    defaultPort: "5432",
    fields: [
      { key: "host", label: "Host", type: "text", placeholder: "db.example.com", required: true },
      { key: "port", label: "Port", type: "text", placeholder: "5432" },
      { key: "database", label: "Database", type: "text", placeholder: "mydb", required: true },
      { key: "username", label: "Username", type: "text", placeholder: "postgres", required: true },
      { key: "password", label: "Password", type: "password", placeholder: "••••••••", required: true },
      { key: "useSsl", label: "Use SSL", type: "checkbox", help: "Recommended for production" },
    ],
  },
  {
    id: "mysql",
    name: "MySQL",
    icon: <Server size={24} />,
    description: "Connect to a MySQL or MariaDB database",
    color: "bg-orange-500",
    defaultPort: "3306",
    fields: [
      { key: "host", label: "Host", type: "text", placeholder: "db.example.com", required: true },
      { key: "port", label: "Port", type: "text", placeholder: "3306" },
      { key: "database", label: "Database", type: "text", placeholder: "mydb", required: true },
      { key: "username", label: "Username", type: "text", placeholder: "root", required: true },
      { key: "password", label: "Password", type: "password", placeholder: "••••••••", required: true },
      { key: "useSsl", label: "Use SSL", type: "checkbox", help: "Recommended for production" },
    ],
  },
  {
    id: "bigquery",
    name: "BigQuery",
    icon: <Cloud size={24} />,
    description: "Connect to Google BigQuery",
    color: "bg-sky-500",
    defaultPort: "",
    fields: [
      { key: "projectId", label: "Project ID", type: "text", placeholder: "my-gcp-project", required: true },
      { key: "database", label: "Dataset", type: "text", placeholder: "my_dataset", required: true },
      { key: "username", label: "Service Account Email", type: "text", placeholder: "sa@project.iam.gserviceaccount.com" },
      { key: "password", label: "Service Account Key (JSON)", type: "password", placeholder: "Paste JSON key..." },
    ],
  },
  {
    id: "snowflake",
    name: "Snowflake",
    icon: <Cloud size={24} />,
    description: "Connect to Snowflake data warehouse",
    color: "bg-cyan-500",
    defaultPort: "",
    fields: [
      { key: "accountId", label: "Account ID", type: "text", placeholder: "xy12345", required: true },
      { key: "warehouse", label: "Warehouse", type: "text", placeholder: "COMPUTE_WH", required: true },
      { key: "database", label: "Database", type: "text", placeholder: "MY_DB", required: true },
      { key: "username", label: "Username", type: "text", placeholder: "admin", required: true },
      { key: "password", label: "Password", type: "password", placeholder: "••••••••", required: true },
      { key: "role", label: "Role", type: "text", placeholder: "ACCOUNTADMIN" },
    ],
  },
  {
    id: "redshift",
    name: "Redshift",
    icon: <Database size={24} />,
    description: "Connect to Amazon Redshift",
    color: "bg-purple-500",
    defaultPort: "5439",
    fields: [
      { key: "host", label: "Host", type: "text", placeholder: "cluster.xxxxx.region.redshift.amazonaws.com", required: true },
      { key: "port", label: "Port", type: "text", placeholder: "5439" },
      { key: "database", label: "Database", type: "text", placeholder: "mydb", required: true },
      { key: "username", label: "Username", type: "text", placeholder: "awsuser", required: true },
      { key: "password", label: "Password", type: "password", placeholder: "••••••••", required: true },
      { key: "region", label: "Region", type: "text", placeholder: "us-east-1" },
      { key: "useSsl", label: "Use SSL", type: "checkbox", help: "Recommended for production" },
    ],
  },
];

type DataConnectionsProps = {
  connections: ConnectionConfig[];
  onRefreshConnections: () => void;
  onConnectionCreated: (datasetId: string, filename: string) => void;
};

export function DataConnections({
  connections,
  onRefreshConnections,
  onConnectionCreated,
}: DataConnectionsProps) {
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);
  const [formData, setFormData] = useState<Record<string, string | boolean>>({});
  const [testStatus, setTestStatus] = useState<"idle" | "testing" | "success" | "error">("idle");
  const [testError, setTestError] = useState("");
  const [connectStatus, setConnectStatus] = useState<"idle" | "connecting" | "success">("idle");
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const provider = PROVIDERS.find((p) => p.id === selectedProvider);

  function openProvider(id: string) {
    const p = PROVIDERS.find((pr) => pr.id === id)!;
    setSelectedProvider(id);
    setFormData({ port: p.defaultPort, useSsl: true });
    setTestStatus("idle");
    setTestError("");
    setConnectStatus("idle");
  }

  function closeForm() {
    setSelectedProvider(null);
    setFormData({});
    setTestStatus("idle");
    setTestError("");
    setConnectStatus("idle");
  }

  function updateField(key: string, value: string | boolean) {
    setFormData((prev) => ({ ...prev, [key]: value }));
  }

  async function handleTest() {
    if (!provider) return;
    setTestStatus("testing");
    setTestError("");
    try {
      const res = await fetch("/api/connections/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(buildPayload(provider, formData)),
      });
      const data = await res.json();
      if (data.ok) {
        setTestStatus("success");
      } else {
        setTestStatus("error");
        setTestError(data.error || "Connection test failed");
      }
    } catch (err) {
      setTestStatus("error");
      setTestError(err instanceof Error ? err.message : "Test failed");
    }
  }

  async function handleConnect() {
    if (!provider) return;
    setConnectStatus("connecting");
    try {
      const res = await fetch("/api/connections", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(buildPayload(provider, formData)),
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "Failed to connect");
      }
      setConnectStatus("success");
      onConnectionCreated(data.datasetId, data.filename);
      onRefreshConnections();
      setTimeout(() => {
        closeForm();
      }, 1500);
    } catch (err) {
      setConnectStatus("idle");
      setTestStatus("error");
      setTestError(err instanceof Error ? err.message : "Connection failed");
    }
  }

  async function handleDelete(id: string) {
    try {
      const res = await fetch(`/api/connections?id=${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Failed to delete");
      onRefreshConnections();
    } catch (err) {
      console.error("Delete failed", err);
    }
    setDeleteConfirm(null);
  }

  function buildPayload(p: Provider, data: Record<string, string | boolean>): Record<string, unknown> {
    const payload: Record<string, unknown> = { provider: p.id };
    for (const field of p.fields) {
      const val = data[field.key];
      if (val !== undefined && val !== "") {
        payload[field.key] = val;
      }
    }
    return payload;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-lg font-bold text-slate-900">Data Connections</h2>
        <p className="text-sm text-slate-500 mt-1">
          Connect to databases and cloud data warehouses to analyze your data.
        </p>
      </div>

      {/* Active Connections */}
      {connections.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold text-slate-700">Active Connections</h3>
          <div className="space-y-2">
            {connections.map((conn) => (
              <div
                key={conn.id}
                className="flex items-center justify-between p-4 bg-white rounded-xl border border-slate-200 shadow-sm"
              >
                <div className="flex items-center gap-3">
                  <div className={`w-8 h-8 rounded-lg flex items-center justify-center text-white ${
                    conn.provider === "postgresql" ? "bg-blue-500" :
                    conn.provider === "mysql" ? "bg-orange-500" :
                    conn.provider === "bigquery" ? "bg-sky-500" :
                    conn.provider === "snowflake" ? "bg-cyan-500" :
                    "bg-purple-500"
                  }`}>
                    <Database size={16} />
                  </div>
                  <div>
                    <div className="text-sm font-medium text-slate-800">
                      {conn.provider} / {conn.database}
                    </div>
                    <div className="text-xs text-slate-400">
                      {conn.connectedAt
                        ? `Connected ${new Date(conn.connectedAt).toLocaleDateString()}`
                        : "Connected"}
                    </div>
                  </div>
                  {conn.connected && (
                    <span className="ml-2 px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-600 text-[10px] font-bold uppercase">
                      Active
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  {conn.datasetId && (
                    <span className="text-xs text-slate-400">
                      Dataset: {conn.filename}
                    </span>
                  )}
                  {deleteConfirm === conn.id ? (
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleDelete(conn.id)}
                        className="px-2 py-1 text-xs bg-red-500 text-white rounded-md hover:bg-red-600"
                      >
                        Confirm
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(null)}
                        className="px-2 py-1 text-xs bg-slate-200 text-slate-600 rounded-md hover:bg-slate-300"
                      >
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setDeleteConfirm(conn.id)}
                      className="p-1.5 text-slate-400 hover:text-red-500 hover:bg-red-50 rounded-md transition-colors"
                      title="Delete connection"
                    >
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Provider Selection or Form */}
      {!selectedProvider ? (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold text-slate-700">Add New Connection</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {PROVIDERS.map((p) => (
              <button
                key={p.id}
                onClick={() => openProvider(p.id)}
                className="p-5 bg-white rounded-xl border border-slate-200 shadow-sm hover:border-indigo-300 hover:shadow-md transition-all text-left group"
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-white ${p.color}`}>
                    {p.icon}
                  </div>
                  <div>
                    <div className="text-sm font-semibold text-slate-800">{p.name}</div>
                    <div className="text-xs text-slate-400">{p.description}</div>
                  </div>
                </div>
                <div className="flex items-center gap-1 text-xs text-indigo-500 font-medium opacity-0 group-hover:opacity-100 transition-opacity">
                  Connect <ChevronRight size={12} />
                </div>
              </button>
            ))}
          </div>
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
          {/* Form Header */}
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between bg-slate-50">
            <div className="flex items-center gap-3">
              <div className={`w-8 h-8 rounded-lg flex items-center justify-center text-white ${provider!.color}`}>
                {provider!.icon}
              </div>
              <div>
                <div className="text-sm font-semibold text-slate-800">
                  Connect to {provider!.name}
                </div>
                <div className="text-xs text-slate-400">{provider!.description}</div>
              </div>
            </div>
            <button
              onClick={closeForm}
              className="p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-200 rounded-md transition-colors"
            >
              <XCircle size={18} />
            </button>
          </div>

          {/* Form Body */}
          <div className="p-6 space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {provider!.fields.map((field) =>
                field.type === "checkbox" ? (
                  <div key={field.key} className="md:col-span-2 flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={!!formData[field.key]}
                      onChange={(e) => updateField(field.key, e.target.checked)}
                      className="w-4 h-4 rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                    />
                    <label className="text-sm text-slate-700">{field.label}</label>
                    {field.help && (
                      <span className="text-xs text-slate-400">({field.help})</span>
                    )}
                  </div>
                ) : (
                  <div key={field.key}>
                    <label className="block text-xs font-medium text-slate-600 mb-1">
                      {field.label}
                      {field.required && <span className="text-red-400 ml-0.5">*</span>}
                    </label>
                    <input
                      type={field.type}
                      placeholder={field.placeholder}
                      value={String(formData[field.key] || "")}
                      onChange={(e) => updateField(field.key, e.target.value)}
                      className="w-full px-3 py-2 border border-slate-200 rounded-lg text-sm text-slate-800 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent"
                    />
                    {field.help && (
                      <p className="text-[10px] text-slate-400 mt-0.5">{field.help}</p>
                    )}
                  </div>
                )
              )}
            </div>

            {/* Security note */}
            <div className="flex items-start gap-2 p-3 bg-slate-50 rounded-lg border border-slate-100">
              <Shield size={14} className="text-slate-400 mt-0.5 shrink-0" />
              <p className="text-[11px] text-slate-500">
                Credentials are used only to establish the connection and are not stored permanently.
                Data is fetched and cached locally for analysis.
              </p>
            </div>

            {/* Test result */}
            {testStatus === "success" && (
              <div className="flex items-center gap-2 p-3 bg-emerald-50 rounded-lg border border-emerald-200">
                <CheckCircle2 size={14} className="text-emerald-500" />
                <span className="text-xs text-emerald-700 font-medium">
                  Connection test passed!
                </span>
              </div>
            )}
            {testStatus === "error" && (
              <div className="flex items-center gap-2 p-3 bg-red-50 rounded-lg border border-red-200">
                <XCircle size={14} className="text-red-500" />
                <span className="text-xs text-red-700 font-medium">{testError}</span>
              </div>
            )}
            {connectStatus === "success" && (
              <div className="flex items-center gap-2 p-3 bg-emerald-50 rounded-lg border border-emerald-200">
                <CheckCircle2 size={14} className="text-emerald-500" />
                <span className="text-xs text-emerald-700 font-medium">
                  Connected! Dataset is now available.
                </span>
              </div>
            )}
          </div>

          {/* Form Footer */}
          <div className="px-6 py-4 border-t border-slate-100 bg-slate-50 flex items-center justify-between">
            <button
              onClick={handleTest}
              disabled={testStatus === "testing"}
              className="px-4 py-2 text-sm font-medium text-slate-600 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors disabled:opacity-50 flex items-center gap-2"
            >
              {testStatus === "testing" ? (
                <Loader2 size={14} className="animate-spin" />
              ) : (
                <Zap size={14} />
              )}
              Test Connection
            </button>
            <div className="flex items-center gap-2">
              <button
                onClick={closeForm}
                className="px-4 py-2 text-sm font-medium text-slate-500 hover:text-slate-700 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleConnect}
                disabled={connectStatus === "connecting" || connectStatus === "success"}
                className="px-5 py-2 text-sm font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors disabled:opacity-50 flex items-center gap-2"
              >
                {connectStatus === "connecting" ? (
                  <Loader2 size={14} className="animate-spin" />
                ) : connectStatus === "success" ? (
                  <CheckCircle2 size={14} />
                ) : null}
                {connectStatus === "connecting"
                  ? "Connecting..."
                  : connectStatus === "success"
                  ? "Connected!"
                  : "Connect & Import"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
