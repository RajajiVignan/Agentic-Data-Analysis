
import React, { useState, useEffect } from "react";
import { Plus, Pencil, Trash2, BookOpen, X, Check } from "lucide-react";
import { useToast } from "@/components/ToastProvider";
import type { MetricDefinition } from "@/lib/api";
import {
  fetchGlossary,
  createGlossaryDef,
  updateGlossaryDef,
  deleteGlossaryDef,
} from "@/lib/api";

const RETURN_TYPES = [
  { value: "number", label: "Number" },
  { value: "percentage", label: "Percentage" },
  { value: "currency", label: "Currency" },
  { value: "ratio", label: "Ratio" },
];

export function GlossaryPanel() {
  const { addToast } = useToast();
  const [defs, setDefs] = useState<MetricDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<MetricDefinition | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", expression: "", description: "", returnType: "number" as MetricDefinition["returnType"] });

  useEffect(() => {
    loadDefs();
  }, []);

  async function loadDefs() {
    setLoading(true);
    try {
      setDefs(await fetchGlossary());
    } catch {
      addToast("Failed to load glossary", "error");
    } finally {
      setLoading(false);
    }
  }

  function resetForm() {
    setForm({ name: "", expression: "", description: "", returnType: "number" });
    setEditing(null);
    setShowForm(false);
  }

  async function handleSave() {
    if (!form.name.trim()) {
      addToast("Name is required", "error");
      return;
    }
    try {
      if (editing) {
        await updateGlossaryDef({ ...editing, ...form });
        addToast("Definition updated", "success");
      } else {
        await createGlossaryDef(form);
        addToast("Definition created", "success");
      }
      resetForm();
      await loadDefs();
    } catch (e) {
      addToast(e instanceof Error ? e.message : "Failed to save", "error");
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteGlossaryDef(id);
      addToast("Definition deleted", "success");
      await loadDefs();
    } catch (e) {
      addToast(e instanceof Error ? e.message : "Failed to delete", "error");
    }
  }

  function startEdit(def: MetricDefinition) {
    setForm({ name: def.name, expression: def.expression, description: def.description, returnType: def.returnType });
    setEditing(def);
    setShowForm(true);
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-slate-900">Business Glossary</h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Define reusable metrics that the AI understands across all analyses
          </p>
        </div>
        <button
          onClick={() => { resetForm(); setShowForm(true); }}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors"
        >
          <Plus size={14} />
          Add Definition
        </button>
      </div>

      {showForm && (
        <div className="p-4 bg-white rounded-2xl border border-indigo-100 card-modern space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. Net Revenue"
                className="w-full mt-1 px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-400"
              />
            </div>
            <div>
              <label className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">Return Type</label>
              <select
                value={form.returnType}
                onChange={(e) => setForm({ ...form, returnType: e.target.value as MetricDefinition["returnType"] })}
                className="w-full mt-1 px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-400 bg-white"
              >
                {RETURN_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>
          </div>
          <div>
            <label className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">Expression</label>
            <input
              type="text"
              value={form.expression}
              onChange={(e) => setForm({ ...form, expression: e.target.value })}
              placeholder="e.g. revenue - discounts - returns"
              className="w-full mt-1 px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-400 font-mono"
            />
          </div>
          <div>
            <label className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider">Description</label>
            <textarea
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder="Describe what this metric means and how it should be used"
              rows={2}
              className="w-full mt-1 px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-400"
            />
          </div>
          <div className="flex items-center justify-end gap-2">
            <button
              onClick={resetForm}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
            >
              <X size={14} />
              Cancel
            </button>
            <button
              onClick={handleSave}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-white bg-indigo-600 rounded-lg hover:bg-indigo-700 transition-colors"
            >
              <Check size={14} />
              {editing ? "Update" : "Create"}
            </button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-8">
          <div className="w-5 h-5 border-2 border-indigo-600 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : defs.length === 0 ? (
        <div className="p-8 text-center text-sm text-slate-400 bg-white rounded-2xl card-modern">
          <BookOpen size={32} className="mx-auto mb-3 opacity-50" />
          <p>No metric definitions yet</p>
          <p className="text-xs text-slate-400 mt-1">
            Define business terms so the AI understands your domain
          </p>
        </div>
      ) : (
        <div className="space-y-2">
          {defs.map((def) => (
            <div key={def.id} className="p-4 bg-white rounded-2xl card-hover card-modern flex items-start justify-between gap-4">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <strong className="text-sm font-semibold text-slate-800">{def.name}</strong>
                  <span className="px-1.5 py-0.5 text-[10px] font-medium bg-indigo-100 text-indigo-600 rounded">{def.returnType}</span>
                </div>
                {def.expression && (
                  <code className="block mt-1 text-xs font-mono text-slate-500 bg-slate-50 px-2 py-1 rounded">{def.expression}</code>
                )}
                {def.description && (
                  <p className="mt-1 text-xs text-slate-400">{def.description}</p>
                )}
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <button
                  onClick={() => startEdit(def)}
                  className="p-1.5 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-indigo-600 transition-all"
                  title="Edit"
                >
                  <Pencil size={14} />
                </button>
                <button
                  onClick={() => handleDelete(def.id)}
                  className="p-1.5 hover:bg-red-50 rounded-lg text-slate-400 hover:text-red-500 transition-all"
                  title="Delete"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
