
import { useState, useCallback, useEffect } from "react";
import {
  Palette,
  Plus,
  Trash2,
  Check,
  X,
  Save,
  ChevronDown,
  ChevronRight,
  Pipette,
} from "lucide-react";

const DEFAULT_PALETTES: Record<string, string[]> = {
  default: ["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4", "#a855f7"],
  warm: ["#f97316", "#ef4444", "#eab308", "#f43f5e", "#d97706", "#fb923c", "#a16207"],
  cool: ["#3b82f6", "#06b6d4", "#6366f1", "#14b8a6", "#0ea5e9", "#8b5cf6", "#2dd4bf"],
  mono: ["#64748b", "#94a3b8", "#475569", "#cbd5e1", "#334155", "#e2e8f0", "#1e293b"],
  sunset: ["#f43f5e", "#f97316", "#eab308", "#a855f7", "#ec4899", "#fbbf24", "#f87171"],
  ocean: ["#0ea5e9", "#06b6d4", "#14b8a6", "#3b82f6", "#2dd4bf", "#22d3ee", "#60a5fa"],
  forest: ["#10b981", "#059669", "#047857", "#34d399", "#6ee7b7", "#065f46", "#0d9488"],
  berry: ["#8b5cf6", "#a855f7", "#d946ef", "#7c3aed", "#c084fc", "#e879f9", "#9333ea"],
};

type PaletteData = {
  name: string;
  colors: string[];
  isBuiltIn: boolean;
};

const PRESET_NAMES = Object.keys(DEFAULT_PALETTES);

function hexToHsl(hex: string): { h: number; s: number; l: number } {
  const r = parseInt(hex.slice(1, 3), 16) / 255;
  const g = parseInt(hex.slice(3, 5), 16) / 255;
  const b = parseInt(hex.slice(5, 7), 16) / 255;
  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  let h = 0;
  let s = 0;
  const l = (max + min) / 2;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    switch (max) {
      case r: h = ((g - b) / d + (g < b ? 6 : 0)) / 6; break;
      case g: h = ((b - r) / d + 2) / 6; break;
      case b: h = ((r - g) / d + 4) / 6; break;
    }
  }

  return { h: Math.round(h * 360), s: Math.round(s * 100), l: Math.round(l * 100) };
}

function generateHarmony(baseHex: string, count: number): string[] {
  const base = hexToHsl(baseHex);
  const colors: string[] = [];
  const step = 360 / count;

  for (let i = 0; i < count; i++) {
    const h = (base.h + step * i) % 360;
    const s = Math.max(30, Math.min(90, base.s + (i % 2 === 0 ? 10 : -10)));
    const l = Math.max(30, Math.min(75, base.l + (i % 3 === 0 ? 5 : -5)));
    colors.push(hslToHex(h, s, l));
  }

  return colors;
}

function hslToHex(h: number, s: number, l: number): string {
  s /= 100;
  l /= 100;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => {
    const k = (n + h / 30) % 12;
    const color = l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1);
    return Math.round(255 * color)
      .toString(16)
      .padStart(2, "0");
  };
  return `#${f(0)}${f(8)}${f(4)}`;
}

export function ColorPaletteEditor({
  currentScheme,
  onSchemeChange,
  onCustomPaletteApply,
}: {
  currentScheme: string;
  onSchemeChange: (scheme: string) => void;
  onCustomPaletteApply?: (colors: string[]) => void;
}) {
  const [palettes, setPalettes] = useState<PaletteData[]>(() => {
    const saved = typeof window !== "undefined" ? localStorage.getItem("insightpilot-custom-palettes") : null;
    const custom: PaletteData[] = saved ? JSON.parse(saved) : [];
    const builtins: PaletteData[] = PRESET_NAMES.map((name) => ({
      name,
      colors: DEFAULT_PALETTES[name],
      isBuiltIn: true,
    }));
    return [...builtins, ...custom];
  });

  const [isEditing, setIsEditing] = useState(false);
  const [editingPalette, setEditingPalette] = useState<PaletteData | null>(null);
  const [newPaletteName, setNewPaletteName] = useState("");
  const [newPaletteColors, setNewPaletteColors] = useState<string[]>(["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4", "#a855f7"]);
  const [expanded, setExpanded] = useState(false);
  const [baseColor, setBaseColor] = useState("#6366f1");

  const saveCustomPalettes = useCallback((updated: PaletteData[]) => {
    const customs = updated.filter((p) => !p.isBuiltIn);
    localStorage.setItem("insightpilot-custom-palettes", JSON.stringify(customs));
    setPalettes(updated);
  }, []);

  const handleCreatePalette = useCallback(() => {
    if (!newPaletteName.trim()) return;
    const newPalette: PaletteData = {
      name: newPaletteName.trim().toLowerCase().replace(/\s+/g, "-"),
      colors: [...newPaletteColors],
      isBuiltIn: false,
    };
    const updated = [...palettes, newPalette];
    saveCustomPalettes(updated);
    setNewPaletteName("");
    setIsEditing(false);
  }, [newPaletteName, newPaletteColors, palettes, saveCustomPalettes]);

  const handleDeletePalette = useCallback(
    (name: string) => {
      const updated = palettes.filter((p) => p.name !== name);
      saveCustomPalettes(updated);
      if (currentScheme === name) {
        onSchemeChange("default");
      }
    },
    [palettes, currentScheme, onSchemeChange, saveCustomPalettes]
  );

  const handleAutoGenerate = useCallback(() => {
    const colors = generateHarmony(baseColor, 7);
    setNewPaletteColors(colors);
  }, [baseColor]);

  const handleColorChange = useCallback((index: number, color: string) => {
    setNewPaletteColors((prev) => {
      const updated = [...prev];
      updated[index] = color;
      return updated;
    });
  }, []);

  const handleAddColor = useCallback(() => {
    setNewPaletteColors((prev) => [...prev, "#6366f1"]);
  }, []);

  const handleRemoveColor = useCallback((index: number) => {
    setNewPaletteColors((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const handleApplyPalette = useCallback(
    (palette: PaletteData) => {
      onSchemeChange(palette.name);
      if (onCustomPaletteApply) {
        onCustomPaletteApply(palette.colors);
      }
    },
    [onSchemeChange, onCustomPaletteApply]
  );

  const handleEditPalette = useCallback((palette: PaletteData) => {
    setEditingPalette(palette);
    setNewPaletteName(palette.name);
    setNewPaletteColors([...palette.colors]);
    setIsEditing(true);
  }, []);

  const handleSaveEdit = useCallback(() => {
    if (!editingPalette) return;
    const updated = palettes.map((p) =>
      p.name === editingPalette.name ? { ...p, colors: [...newPaletteColors] } : p
    );
    saveCustomPalettes(updated);
    setEditingPalette(null);
    setIsEditing(false);
    setNewPaletteName("");
    setNewPaletteColors(["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4", "#a855f7"]);
  }, [editingPalette, newPaletteColors, palettes, saveCustomPalettes]);

  return (
    <div className="space-y-3">
      {/* Section header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 px-1">
          <div className="p-1.5 bg-indigo-100 rounded-lg">
            <Palette size={14} className="text-indigo-600" />
          </div>
          <span className="text-xs font-semibold text-slate-700 uppercase tracking-wider">
            Color Palettes
          </span>
        </div>
        <button
          onClick={() => {
            setExpanded(!expanded);
            if (!expanded) setIsEditing(false);
          }}
          className="p-1 text-slate-400 hover:text-slate-600 transition-colors"
        >
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </button>
      </div>

      {/* Palette grid (always visible) */}
      <div className="grid grid-cols-4 gap-1.5">
        {palettes.map((palette) => {
          const isActive = currentScheme === palette.name;
          return (
            <button
              key={palette.name}
              onClick={() => handleApplyPalette(palette)}
              className={`relative group flex flex-col items-center gap-1 p-2 rounded-xl text-[10px] font-medium transition-all ${
                isActive
                  ? "bg-indigo-50 border-2 border-indigo-300 text-indigo-700 shadow-sm"
                  : "bg-slate-50 border-2 border-transparent text-slate-500 hover:bg-slate-100 hover:text-slate-700"
              }`}
              title={palette.name}
            >
              <div className="flex gap-0.5">
                {palette.colors.slice(0, 5).map((c, i) => (
                  <div
                    key={i}
                    className="w-2.5 h-2.5 rounded-full"
                    style={{ backgroundColor: c }}
                  />
                ))}
              </div>
              <span className="truncate w-full text-center capitalize">
                {palette.name.replace(/-/g, " ")}
              </span>
              {isActive && (
                <div className="absolute -top-1 -right-1 w-3.5 h-3.5 bg-indigo-500 rounded-full flex items-center justify-center">
                  <Check size={8} className="text-white" />
                </div>
              )}
            </button>
          );
        })}

        {/* Add new button */}
        <button
          onClick={() => {
            setIsEditing(true);
            setEditingPalette(null);
            setNewPaletteName("");
            setNewPaletteColors(["#6366f1", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4", "#a855f7"]);
            setExpanded(true);
          }}
          className="flex flex-col items-center gap-1 p-2 rounded-xl text-[10px] font-medium bg-slate-50 border-2 border-dashed border-slate-300 text-slate-400 hover:bg-indigo-50 hover:border-indigo-300 hover:text-indigo-600 transition-all"
        >
          <Plus size={14} />
          <span>New</span>
        </button>
      </div>

      {/* Expanded editor */}
      {expanded && (
        <div className="animate-slide-up space-y-3 pt-2 border-t border-slate-100">
          {isEditing ? (
            <div className="space-y-3 p-3 bg-slate-50 rounded-xl">
              {/* Palette name */}
              <div className="space-y-1.5">
                <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider px-1">
                  {editingPalette ? "Edit Palette" : "Palette Name"}
                </label>
                <input
                  type="text"
                  value={newPaletteName}
                  onChange={(e) => setNewPaletteName(e.target.value)}
                  disabled={!!editingPalette}
                  placeholder="my-palette"
                  className="w-full px-2.5 py-1.5 text-xs bg-white border border-slate-200 rounded-lg text-slate-700 placeholder:text-slate-400 disabled:opacity-50 focus:outline-none focus:ring-1 focus:ring-indigo-300"
                />
              </div>

              {/* Auto-generate from base color */}
              <div className="space-y-1.5">
                <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider px-1 flex items-center gap-1">
                  <Pipette size={10} />
                  Auto-Generate from Base
                </label>
                <div className="flex gap-2">
                  <input
                    type="color"
                    value={baseColor}
                    onChange={(e) => setBaseColor(e.target.value)}
                    className="w-8 h-8 rounded-lg border border-slate-200 cursor-pointer"
                  />
                  <button
                    onClick={handleAutoGenerate}
                    className="flex-1 px-3 py-1.5 text-xs font-medium text-indigo-600 bg-indigo-50 hover:bg-indigo-100 rounded-lg transition-colors"
                  >
                    Generate Harmony
                  </button>
                </div>
              </div>

              {/* Color inputs */}
              <div className="space-y-1.5">
                <label className="text-[10px] font-medium text-slate-500 uppercase tracking-wider px-1">
                  Colors ({newPaletteColors.length})
                </label>
                <div className="grid grid-cols-7 gap-1.5">
                  {newPaletteColors.map((color, i) => (
                    <div key={i} className="relative group">
                      <input
                        type="color"
                        value={color}
                        onChange={(e) => handleColorChange(i, e.target.value)}
                        className="w-full h-8 rounded-lg border border-slate-200 cursor-pointer"
                      />
                      {newPaletteColors.length > 2 && (
                        <button
                          onClick={() => handleRemoveColor(i)}
                          className="absolute -top-1 -right-1 w-3.5 h-3.5 bg-red-500 rounded-full items-center justify-center text-white opacity-0 group-hover:opacity-100 transition-opacity hidden group-hover:flex"
                        >
                          <X size={8} />
                        </button>
                      )}
                    </div>
                  ))}
                  {newPaletteColors.length < 12 && (
                    <button
                      onClick={handleAddColor}
                      className="h-8 rounded-lg border-2 border-dashed border-slate-300 flex items-center justify-center text-slate-400 hover:border-indigo-300 hover:text-indigo-500 transition-colors"
                    >
                      <Plus size={12} />
                    </button>
                  )}
                </div>
              </div>

              {/* Actions */}
              <div className="flex gap-2">
                <button
                  onClick={editingPalette ? handleSaveEdit : handleCreatePalette}
                  disabled={!newPaletteName.trim()}
                  className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <Save size={12} />
                  {editingPalette ? "Save Changes" : "Create Palette"}
                </button>
                <button
                  onClick={() => {
                    setIsEditing(false);
                    setEditingPalette(null);
                  }}
                  className="px-3 py-2 text-xs font-medium text-slate-600 hover:text-slate-800 bg-white border border-slate-200 hover:border-slate-300 rounded-lg transition-colors"
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              {/* Custom palettes management */}
              {palettes.filter((p) => !p.isBuiltIn).length > 0 && (
                <div className="space-y-1.5">
                  <p className="text-[10px] font-semibold text-slate-500 uppercase tracking-wider px-1">
                    Your Custom Palettes
                  </p>
                  {palettes
                    .filter((p) => !p.isBuiltIn)
                    .map((palette) => (
                      <div
                        key={palette.name}
                        className="flex items-center justify-between p-2 bg-slate-50 rounded-lg group"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <div className="flex gap-0.5">
                            {palette.colors.slice(0, 5).map((c, i) => (
                              <div
                                key={i}
                                className="w-3 h-3 rounded-full"
                                style={{ backgroundColor: c }}
                              />
                            ))}
                          </div>
                          <span className="text-xs font-medium text-slate-700 truncate capitalize">
                            {palette.name.replace(/-/g, " ")}
                          </span>
                        </div>
                        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                          <button
                            onClick={() => handleEditPalette(palette)}
                            className="p-1 text-slate-400 hover:text-indigo-600 rounded transition-colors"
                            title="Edit"
                          >
                            <Pipette size={12} />
                          </button>
                          <button
                            onClick={() => handleDeletePalette(palette.name)}
                            className="p-1 text-slate-400 hover:text-red-500 rounded transition-colors"
                            title="Delete"
                          >
                            <Trash2 size={12} />
                          </button>
                        </div>
                      </div>
                    ))}
                </div>
              )}

              <p className="text-[10px] text-slate-400 text-center px-1">
                Click a palette to apply it. Create your own with the + button above.
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
