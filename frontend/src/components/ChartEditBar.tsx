
import { useState, useRef, useCallback, useEffect } from "react";
import {
  Wand2,
  Loader2,
  ArrowRight,
  Sparkles,
  X,
  BarChart3,
  LineChart,
  PieChart,
  AreaChart,
  ScatterChart,
  Palette,
  TrendingUp,
} from "lucide-react";

type ChartEditCommand = {
  type: "chartType" | "colorScheme" | "filter" | "sort" | "style";
  value: string;
  label: string;
};

const CHART_TYPE_COMMANDS: { patterns: RegExp[]; type: ChartEditCommand["type"]; value: string; label: string }[] = [
  { patterns: [/bar\s*chart/i, /bars?\b/i, /column/i], type: "chartType", value: "bar", label: "Bar Chart" },
  { patterns: [/line\s*chart/i, /lines?\b/i, /trend/i], type: "chartType", value: "line", label: "Line Chart" },
  { patterns: [/pie\s*chart/i, /pie\b/i, /donut/i, /circle/i], type: "chartType", value: "pie", label: "Pie Chart" },
  { patterns: [/area\s*chart/i, /area\b/i, /filled/i], type: "chartType", value: "area", label: "Area Chart" },
  { patterns: [/scatter/i, /correlation/i, /xy\s*plot/i], type: "chartType", value: "scatter", label: "Scatter Plot" },
];

const COLOR_COMMANDS: { patterns: RegExp[]; value: string; label: string }[] = [
  { patterns: [/warm/i, /sunset/i, /fire/i, /orange/i, /red/i], value: "warm", label: "Warm" },
  { patterns: [/cool/i, /ocean/i, /blue/i, /water/i, /ice/i], value: "cool", label: "Cool" },
  { patterns: [/mono/i, /grayscale/i, /black\s*and\s*white/i, /grey/i], value: "mono", label: "Monochrome" },
  { patterns: [/forest/i, /green/i, /nature/i, /earth/i], value: "forest", label: "Forest" },
  { patterns: [/berry/i, /purple/i, /violet/i, /lavender/i], value: "berry", label: "Berry" },
  { patterns: [/default/i, /standard/i, /normal/i, /original/i], value: "default", label: "Default" },
];

const STYLE_COMMANDS: { patterns: RegExp[]; value: string; label: string }[] = [
  { patterns: [/rounded/i, /soft/i, /gentle/i], value: "rounded", label: "Rounded" },
  { patterns: [/sharp/i, /angular/i, /crisp/i], value: "sharp", label: "Sharp" },
  { patterns: [/gradient/i, /fade/i, /blend/i], value: "gradient", label: "Gradient" },
];

function parseCommand(input: string): ChartEditCommand | null {
  const trimmed = input.trim().toLowerCase();

  // Chart type commands
  for (const cmd of CHART_TYPE_COMMANDS) {
    for (const pattern of cmd.patterns) {
      if (pattern.test(trimmed)) {
        return { type: "chartType", value: cmd.value, label: cmd.label };
      }
    }
  }

  // Color commands
  for (const cmd of COLOR_COMMANDS) {
    for (const pattern of cmd.patterns) {
      if (pattern.test(trimmed)) {
        return { type: "colorScheme", value: cmd.value, label: cmd.label };
      }
    }
  }

  // Style commands
  for (const cmd of STYLE_COMMANDS) {
    for (const pattern of cmd.patterns) {
      if (pattern.test(trimmed)) {
        return { type: "style", value: cmd.value, label: cmd.label };
      }
    }
  }

  // Sort commands
  if (/\b(sort|order|arrange)\b/i.test(trimmed)) {
    if (/\b(asc|ascending|low\s*to\s*high|a\s*to\s*z)\b/i.test(trimmed)) {
      return { type: "sort", value: "asc", label: "Sort Ascending" };
    }
    if (/\b(desc|descending|high\s*to\s*low|z\s*to\s*a)\b/i.test(trimmed)) {
      return { type: "sort", value: "desc", label: "Sort Descending" };
    }
  }

  return null;
}

const SUGGESTED_COMMANDS = [
  "Switch to line chart",
  "Use warm colors",
  "Change to pie chart",
  "Sort descending",
  "Try scatter plot",
  "Use monochrome palette",
];

export function ChartEditBar({
  onChartTypeChange,
  onColorSchemeChange,
  onSortChange,
  hasResults,
}: {
  onChartTypeChange?: (type: string) => void;
  onColorSchemeChange?: (scheme: string) => void;
  onSortChange?: (order: "asc" | "desc") => void;
  hasResults: boolean;
}) {
  const [input, setInput] = useState("");
  const [command, setCommand] = useState<ChartEditCommand | null>(null);
  const [applied, setApplied] = useState(false);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (input.trim()) {
      const parsed = parseCommand(input);
      setCommand(parsed);
      setApplied(false);
    } else {
      setCommand(null);
    }
  }, [input]);

  const handleApply = useCallback(() => {
    if (!command) return;

    switch (command.type) {
      case "chartType":
        onChartTypeChange?.(command.value);
        break;
      case "colorScheme":
        onColorSchemeChange?.(command.value);
        break;
      case "sort":
        onSortChange?.(command.value as "asc" | "desc");
        break;
    }

    setApplied(true);
    setTimeout(() => {
      setInput("");
      setCommand(null);
      setApplied(false);
    }, 1500);
  }, [command, onChartTypeChange, onColorSchemeChange, onSortChange]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && command && !applied) {
        handleApply();
      }
      if (e.key === "Escape") {
        setInput("");
        setCommand(null);
        setShowSuggestions(false);
      }
    },
    [command, applied, handleApply]
  );

  const handleSuggestionClick = useCallback((suggestion: string) => {
    setInput(suggestion);
    setShowSuggestions(false);
    inputRef.current?.focus();
  }, []);

  if (!hasResults) return null;

  return (
    <div className="relative">
      {/* Edit bar */}
      <div className="flex items-center gap-2 p-2 bg-gradient-to-r from-violet-50/80 to-pink-50/80 border border-violet-200/80 rounded-2xl backdrop-blur-sm">
        <div className="flex items-center gap-1.5 pl-2">
          <div className="p-1 bg-violet-100 rounded-md">
            <Wand2 size={12} className="text-violet-600" />
          </div>
          <span className="text-[10px] font-semibold text-violet-600 uppercase tracking-wider hidden sm:inline">
            Edit
          </span>
        </div>

        <div className="relative flex-1">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            onFocus={() => input.trim() === "" && setShowSuggestions(true)}
            onBlur={() => setTimeout(() => setShowSuggestions(false), 200)}
            placeholder="Type a command: &quot;switch to line chart&quot;, &quot;use warm colors&quot;..."
            className="w-full px-3 py-2 text-xs bg-white border border-violet-200 rounded-xl text-slate-700 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-violet-300 focus:border-violet-300 transition-all"
          />

          {/* Command preview */}
          {command && !applied && (
            <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1.5">
              <span className="px-2 py-0.5 text-[10px] font-medium bg-violet-100 text-violet-700 rounded-full">
                {command.label}
              </span>
              <button
                onClick={handleApply}
                className="p-1 bg-violet-600 text-white rounded-lg hover:bg-violet-700 transition-colors"
                title="Apply command"
              >
                <ArrowRight size={12} />
              </button>
            </div>
          )}

          {/* Applied feedback */}
          {applied && (
            <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1.5 animate-fade-in">
              <span className="px-2 py-0.5 text-[10px] font-medium bg-emerald-100 text-emerald-700 rounded-full flex items-center gap-1">
                <Sparkles size={10} />
                Applied!
              </span>
            </div>
          )}

          {/* Suggestions dropdown */}
          {showSuggestions && !command && (
            <div className="absolute top-full left-0 right-0 mt-1 bg-white rounded-xl border border-violet-200 shadow-lg py-1 z-30 animate-slide-up">
              <p className="px-3 py-1.5 text-[10px] font-semibold text-slate-400 uppercase tracking-wider">
                Quick Commands
              </p>
              {SUGGESTED_COMMANDS.map((s, i) => (
                <button
                  key={i}
                  onMouseDown={() => handleSuggestionClick(s)}
                  className="w-full text-left px-3 py-2 text-xs text-slate-600 hover:bg-violet-50 hover:text-violet-700 transition-colors flex items-center gap-2"
                >
                  <Wand2 size={10} className="text-violet-400" />
                  {s}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Clear button */}
        {input && (
          <button
            onClick={() => {
              setInput("");
              setCommand(null);
              setApplied(false);
            }}
            className="p-1.5 text-slate-400 hover:text-slate-600 rounded-lg hover:bg-white/80 transition-colors"
          >
            <X size={14} />
          </button>
        )}
      </div>

      {/* Command type indicators */}
      {!input && (
        <div className="flex items-center gap-1.5 mt-2 px-1 flex-wrap">
          {[
            { icon: BarChart3, label: "Chart type", example: "bar, line, pie, area, scatter" },
            { icon: Palette, label: "Colors", example: "warm, cool, mono, forest, berry" },
            { icon: TrendingUp, label: "Sort", example: "ascending, descending" },
          ].map((hint, i) => (
            <span
              key={i}
              className="inline-flex items-center gap-1 px-2 py-0.5 text-[10px] text-slate-400 bg-slate-50 rounded-full"
            >
              <hint.icon size={9} />
              {hint.example}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
