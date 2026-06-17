"use client";

import React, { useState, useEffect } from "react";
import {
  CalendarClock,
  Bell,
  Plus,
  Trash2,
  Mail,
  Webhook,
} from "lucide-react";
import {
  fetchReports,
  createReport,
  deleteReport,
  fetchAlerts,
  createAlert,
  deleteAlert,
} from "@/lib/api";
import type { Dataset, ScheduledReport, AlertRule } from "@/lib/api";

type Props = {
  datasets: Dataset[];
};

export function ScheduleManager({ datasets }: Props) {
  const [reports, setReports] = useState<ScheduledReport[]>([]);
  const [alerts, setAlerts] = useState<AlertRule[]>([]);
  const [tab, setTab] = useState<"reports" | "alerts">("reports");
  const [showNewReport, setShowNewReport] = useState(false);
  const [showNewAlert, setShowNewAlert] = useState(false);

  useEffect(() => {
    loadAll();
  }, []);

  async function loadAll() {
    try {
      const [r, a] = await Promise.all([fetchReports(), fetchAlerts()]);
      setReports(r);
      setAlerts(a);
    } catch {
      // ignore
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 border-b border-slate-200 pb-3">
        <button
          onClick={() => setTab("reports")}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            tab === "reports"
              ? "bg-indigo-100 text-indigo-700"
              : "text-slate-500 hover:text-slate-700 hover:bg-slate-100"
          }`}
        >
          <CalendarClock size={16} />
          Scheduled Reports
        </button>
        <button
          onClick={() => setTab("alerts")}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            tab === "alerts"
              ? "bg-indigo-100 text-indigo-700"
              : "text-slate-500 hover:text-slate-700 hover:bg-slate-100"
          }`}
        >
          <Bell size={16} />
          Alert Rules
        </button>
      </div>

      {tab === "reports" && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-700">Scheduled Reports</h3>
            <button
              onClick={() => setShowNewReport(true)}
              className="flex items-center gap-1 px-3 py-1.5 bg-indigo-600 text-white text-xs font-medium rounded-lg hover:bg-indigo-700"
            >
              <Plus size={12} />
              New Report
            </button>
          </div>

          {showNewReport && (
            <ReportForm
              datasets={datasets}
              onSave={async (r) => {
                await createReport(r);
                setShowNewReport(false);
                loadAll();
              }}
              onCancel={() => setShowNewReport(false)}
            />
          )}

          {reports.length === 0 && !showNewReport && (
            <div className="p-8 text-center text-sm text-slate-400 bg-white rounded-2xl border border-slate-200">
              No scheduled reports yet. Create one to get email/Slack alerts on a schedule.
            </div>
          )}

          {reports.map((r) => (
            <div
              key={r.id}
              className="bg-white rounded-2xl border border-slate-200 shadow-sm p-4"
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <CalendarClock size={14} className="text-indigo-500" />
                  <span className="text-sm font-semibold text-slate-800">{r.name}</span>
                </div>
                <button
                  onClick={async () => {
                    await deleteReport(r.id);
                    loadAll();
                  }}
                  className="p-1 hover:bg-red-50 rounded text-slate-400 hover:text-red-500"
                >
                  <Trash2 size={14} />
                </button>
              </div>
              <div className="flex flex-wrap gap-2 text-xs text-slate-500">
                <span className="px-2 py-0.5 bg-slate-100 rounded">
                  {r.frequency}
                  {r.frequency === "weekly" && ` (day ${r.dayOfWeek})`}
                  {r.frequency === "monthly" && ` (day ${r.dayOfMonth})`}
                </span>
                <span className="px-2 py-0.5 bg-slate-100 rounded">at {r.hour}:00</span>
                {r.emails.length > 0 && (
                  <span className="px-2 py-0.5 bg-blue-50 text-blue-600 rounded flex items-center gap-1">
                    <Mail size={10} />
                    {r.emails.length} email(s)
                  </span>
                )}
                {r.slackWebhook && (
                  <span className="px-2 py-0.5 bg-purple-50 text-purple-600 rounded flex items-center gap-1">
                    <Webhook size={10} />
                    Slack
                  </span>
                )}
                {r.nextRun && (
                  <span className="text-slate-400">
                    Next: {new Date(r.nextRun).toLocaleString()}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {tab === "alerts" && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-700">Alert Rules</h3>
            <button
              onClick={() => setShowNewAlert(true)}
              className="flex items-center gap-1 px-3 py-1.5 bg-indigo-600 text-white text-xs font-medium rounded-lg hover:bg-indigo-700"
            >
              <Plus size={12} />
              New Alert
            </button>
          </div>

          {showNewAlert && (
            <AlertForm
              datasets={datasets}
              onSave={async (a) => {
                await createAlert(a);
                setShowNewAlert(false);
                loadAll();
              }}
              onCancel={() => setShowNewAlert(false)}
            />
          )}

          {alerts.length === 0 && !showNewAlert && (
            <div className="p-8 text-center text-sm text-slate-400 bg-white rounded-2xl border border-slate-200">
              No alert rules yet. Create one to get notified when metrics change.
            </div>
          )}

          {alerts.map((a) => (
            <div
              key={a.id}
              className="bg-white rounded-2xl border border-slate-200 shadow-sm p-4"
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Bell size={14} className="text-amber-500" />
                  <span className="text-sm font-semibold text-slate-800">{a.name}</span>
                </div>
                <button
                  onClick={async () => {
                    await deleteAlert(a.id);
                    loadAll();
                  }}
                  className="p-1 hover:bg-red-50 rounded text-slate-400 hover:text-red-500"
                >
                  <Trash2 size={14} />
                </button>
              </div>
              <div className="flex flex-wrap gap-2 text-xs text-slate-500">
                <span className="px-2 py-0.5 bg-amber-50 text-amber-600 rounded font-medium">
                  {a.condition === "drop" ? "Drops >" : "Rises >"} {a.threshold}%
                </span>
                <span className="px-2 py-0.5 bg-slate-100 rounded">
                  {a.metricCol}
                </span>
                {a.emails.length > 0 && (
                  <span className="px-2 py-0.5 bg-blue-50 text-blue-600 rounded flex items-center gap-1">
                    <Mail size={10} />
                    {a.emails.length} email(s)
                  </span>
                )}
                {a.lastChecked && (
                  <span className="text-slate-400">
                    Last checked: {new Date(a.lastChecked).toLocaleString()}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ReportForm({
  datasets,
  onSave,
  onCancel,
}: {
  datasets: Dataset[];
  onSave: (r: Partial<ScheduledReport>) => Promise<void>;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [frequency, setFrequency] = useState<"daily" | "weekly" | "monthly">("daily");
  const [dayOfWeek, setDayOfWeek] = useState(1);
  const [dayOfMonth, setDayOfMonth] = useState(1);
  const [hour, setHour] = useState(9);
  const [emails, setEmails] = useState("");
  const [slackWebhook, setSlackWebhook] = useState("");
  const [saving, setSaving] = useState(false);

  async function handleSave() {
    if (!name) return;
    setSaving(true);
    try {
      await onSave({
        name,
        frequency,
        dayOfWeek,
        dayOfMonth,
        hour,
        datasetIds: datasets.map((d) => d.id),
        emails: emails.split(",").map((e) => e.trim()).filter(Boolean),
        slackWebhook: slackWebhook || undefined,
      });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="bg-white rounded-2xl border border-indigo-200 shadow-sm p-4 space-y-3">
      <input
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Report name"
        className="w-full p-2 text-sm border border-slate-200 rounded-lg"
      />
      <div className="grid grid-cols-2 gap-3">
        <select
          value={frequency}
          onChange={(e) => setFrequency(e.target.value as "daily" | "weekly" | "monthly")}
          className="p-2 text-sm border border-slate-200 rounded-lg"
        >
          <option value="daily">Daily</option>
          <option value="weekly">Weekly</option>
          <option value="monthly">Monthly</option>
        </select>
        <select
          value={hour}
          onChange={(e) => setHour(Number(e.target.value))}
          className="p-2 text-sm border border-slate-200 rounded-lg"
        >
          {Array.from({ length: 24 }, (_, i) => (
            <option key={i} value={i}>
              {i.toString().padStart(2, "0")}:00
            </option>
          ))}
        </select>
      </div>
      <input
        type="text"
        value={emails}
        onChange={(e) => setEmails(e.target.value)}
        placeholder="Email recipients (comma separated)"
        className="w-full p-2 text-sm border border-slate-200 rounded-lg"
      />
      <input
        type="text"
        value={slackWebhook}
        onChange={(e) => setSlackWebhook(e.target.value)}
        placeholder="Slack webhook URL (optional)"
        className="w-full p-2 text-sm border border-slate-200 rounded-lg"
      />
      <div className="flex gap-2">
        <button
          onClick={handleSave}
          disabled={saving || !name}
          className="flex-1 py-2 bg-indigo-600 text-white text-sm font-medium rounded-lg hover:bg-indigo-700 disabled:opacity-50"
        >
          {saving ? "Saving..." : "Create Report"}
        </button>
        <button
          onClick={onCancel}
          className="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-lg"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

function AlertForm({
  datasets,
  onSave,
  onCancel,
}: {
  datasets: Dataset[];
  onSave: (a: Partial<AlertRule>) => Promise<void>;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [datasetId, setDatasetId] = useState(datasets[0]?.id ?? "");
  const [metricCol, setMetricCol] = useState("");
  const [condition, setCondition] = useState<"drop" | "rise">("drop");
  const [threshold, setThreshold] = useState(10);
  const [emails, setEmails] = useState("");
  const [saving, setSaving] = useState(false);

  const ds = datasets.find((d) => d.id === datasetId);
  const numericCols = ds?.profile?.columns?.filter((c) => c.type === "number") ?? [];

  async function handleSave() {
    if (!name || !datasetId || !metricCol) return;
    setSaving(true);
    try {
      await onSave({
        name,
        datasetId,
        metricCol,
        condition,
        threshold,
        emails: emails.split(",").map((e) => e.trim()).filter(Boolean),
      });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="bg-white rounded-2xl border border-amber-200 shadow-sm p-4 space-y-3">
      <input
        type="text"
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder="Alert name"
        className="w-full p-2 text-sm border border-slate-200 rounded-lg"
      />
      <select
        value={datasetId}
        onChange={(e) => {
          setDatasetId(e.target.value);
          setMetricCol("");
        }}
        className="w-full p-2 text-sm border border-slate-200 rounded-lg"
      >
        {datasets.map((d) => (
          <option key={d.id} value={d.id}>
            {d.filename}
          </option>
        ))}
      </select>
      <div className="grid grid-cols-2 gap-3">
        <select
          value={metricCol}
          onChange={(e) => setMetricCol(e.target.value)}
          className="p-2 text-sm border border-slate-200 rounded-lg"
        >
          <option value="">Select metric...</option>
          {numericCols.map((c) => (
            <option key={c.name} value={c.name}>
              {c.name}
            </option>
          ))}
        </select>
        <div className="flex gap-2">
          <select
            value={condition}
            onChange={(e) => setCondition(e.target.value as "drop" | "rise")}
            className="flex-1 p-2 text-sm border border-slate-200 rounded-lg"
          >
            <option value="drop">Drops by</option>
            <option value="rise">Rises by</option>
          </select>
          <input
            type="number"
            value={threshold}
            onChange={(e) => setThreshold(Number(e.target.value))}
            className="w-20 p-2 text-sm border border-slate-200 rounded-lg"
          />
          <span className="flex items-center text-xs text-slate-500">%</span>
        </div>
      </div>
      <input
        type="text"
        value={emails}
        onChange={(e) => setEmails(e.target.value)}
        placeholder="Alert emails (comma separated)"
        className="w-full p-2 text-sm border border-slate-200 rounded-lg"
      />
      <div className="flex gap-2">
        <button
          onClick={handleSave}
          disabled={saving || !name || !metricCol}
          className="flex-1 py-2 bg-amber-600 text-white text-sm font-medium rounded-lg hover:bg-amber-700 disabled:opacity-50"
        >
          {saving ? "Saving..." : "Create Alert"}
        </button>
        <button
          onClick={onCancel}
          className="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-lg"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
