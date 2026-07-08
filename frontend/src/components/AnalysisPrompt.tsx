
import { useRef, useEffect } from "react";
import { Sparkles, Loader2, Trash2, Filter, Send } from "lucide-react";

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
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!loading && inputRef.current) {
      inputRef.current.focus();
    }
  }, [loading]);

  return (
    <div className="space-y-4">
      {/* Active session indicator */}
      {hasActiveSession && (
        <div className="flex items-center justify-between px-4 py-2 bg-gradient-to-r from-indigo-50/80 to-purple-50/80 border border-indigo-100/80 rounded-xl backdrop-blur-sm animate-slide-up">
          <div className="flex items-center gap-2 text-xs text-indigo-700">
            <div className="p-1 bg-indigo-100 rounded-md">
              <Filter size={12} className="text-indigo-600" />
            </div>
            <span className="font-medium">Follow-up mode</span>
            <span className="text-indigo-400 hidden sm:inline">— ask a follow-up question or start fresh</span>
          </div>
          <button
            onClick={onNewAnalysis}
            className="flex items-center gap-1.5 text-xs font-medium text-red-500 hover:text-red-600 transition-colors px-2 py-1 rounded-lg hover:bg-red-50"
          >
            <Trash2 size={12} />
            New Analysis
          </button>
        </div>
      )}

      {/* Prompt input */}
      <div className="relative group">
        <div className="absolute -inset-1 bg-gradient-to-r from-indigo-500 via-purple-500 to-pink-500 rounded-2xl blur-lg opacity-20 group-hover:opacity-30 transition duration-700"></div>
        <div className="relative bg-white p-1.5 rounded-2xl border border-slate-200/80 shadow-sm flex items-center gap-2 group-focus-within:border-indigo-300 group-focus-within:shadow-md transition-all">
          <div className="pl-3 text-indigo-400">
            <Sparkles size={20} />
          </div>
          <input
            ref={inputRef}
            className="flex-1 py-3 px-2 outline-none text-slate-700 placeholder:text-slate-400 bg-transparent"
            placeholder={
              hasActiveSession
                ? "Ask a follow-up question..."
                : "Ask your data a question..."
            }
            value={prompt}
            onChange={(e) => onPromptChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !loading && prompt.trim()) onRun();
            }}
          />
          <button
            onClick={onRun}
            disabled={disabled || !prompt.trim()}
            className="bg-gradient-to-r from-indigo-600 to-indigo-500 hover:from-indigo-700 hover:to-indigo-600 text-white px-5 py-2.5 rounded-xl font-medium transition-all disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-2 shadow-sm shadow-indigo-500/20 hover:shadow-md hover:shadow-indigo-500/30 active:scale-[0.97]"
          >
            {loading ? (
              <Loader2 className="animate-spin" size={18} />
            ) : (
              <Send size={16} />
            )}
            <span className="hidden sm:inline">{loading ? "Analyzing..." : hasActiveSession ? "Send" : "Analyze"}</span>
          </button>
        </div>
      </div>
      {error && (
        <p className="text-sm text-red-500 flex items-center gap-2 px-1 animate-fade-in">
          <span className="w-1.5 h-1.5 rounded-full bg-red-500 shrink-0" />
          {error}
        </p>
      )}
    </div>
  );
}
