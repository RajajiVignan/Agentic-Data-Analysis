package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LLMConfig holds the minimal config needed for LLM-driven code generation.
type LLMConfig struct {
	Enabled     bool
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float64
	TimeoutSec  int
}

// VizType constants
const (
	VizTypeMatplotlib = "matplotlib"
	VizTypeBokeh      = "bokeh"
	VizTypePlotly     = "plotly"
)

// DesignConfig holds visual design settings and axis/aggregation configuration for plot generation.
type DesignConfig struct {
	AccentColor string `json:"accentColor,omitempty"`
	ChartScheme string `json:"chartScheme,omitempty"`
	FontFamily  string `json:"fontFamily,omitempty"`
	FontSize    string `json:"fontSize,omitempty"`
	ChartType   string `json:"chartType,omitempty"`
	XAxis       string `json:"xAxis,omitempty"`
	YAxis       string `json:"yAxis,omitempty"`
	Aggregation string `json:"aggregation,omitempty"`
}

func defaultDesignConfig() DesignConfig {
	return DesignConfig{
		AccentColor: "#6366f1",
		ChartScheme: "default",
		FontFamily:  "sans-serif",
		FontSize:    "medium",
		ChartType:   "auto",
		Aggregation: "sum",
	}
}

// hexAccent returns the CSS-compatible hex color from the design config.
// If the config value is a named color (e.g. "indigo", "emerald"), map it.
func (dc DesignConfig) hexAccent() string {
	palette := map[string]string{
		"indigo":  "#6366f1",
		"blue":    "#3b82f6",
		"emerald": "#10b981",
		"amber":   "#f59e0b",
		"rose":    "#f43f5e",
		"violet":  "#8b5cf6",
	}
	if c, ok := palette[dc.AccentColor]; ok {
		return c
	}
	if dc.AccentColor != "" {
		return dc.AccentColor
	}
	return "#6366f1"
}

// PythonBridge handles execution of Python visualization scripts.
type PythonBridge struct {
	plotsDir   string
	llmConfig  LLMConfig
	validator  *SandboxValidator
	httpClient *http.Client
}

// NewPythonBridge creates a new Python bridge with the given plots directory.
func NewPythonBridge(plotsDir string) *PythonBridge {
	_ = os.MkdirAll(plotsDir, 0755)
	return &PythonBridge{
		plotsDir:  plotsDir,
		validator: NewSandboxValidator(),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetLLMConfig sets the LLM configuration for code generation.
// If not set or disabled, only the deterministic fallback is used.
func (pb *PythonBridge) SetLLMConfig(cfg LLMConfig) {
	pb.llmConfig = cfg
}

// CleanupOlderThan removes generated Python plot artifacts older than maxAge.
func (pb *PythonBridge) CleanupOlderThan(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(pb.plotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".py" && ext != ".png" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(pb.plotsDir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// ExecuteScript runs a Python script and returns the path to the generated plot.
// The script receives csvPath as sys.argv[1], plotPath as sys.argv[2],
// and an optional design JSON as sys.argv[3].
// The vizType determines the output format: matplotlib -> .png, bokeh/plotly -> .html.
func (pb *PythonBridge) ExecuteScript(scriptID, scriptContent, csvPath, vizType, designJSON string) (string, error) {
	scriptPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s.py", scriptID))
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}

	ext := ".png"
	if vizType == VizTypeBokeh || vizType == VizTypePlotly {
		ext = ".html"
	}
	plotPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s_plot%s", scriptID, ext))

	args := []string{scriptPath, csvPath, plotPath}
	if designJSON != "" {
		args = append(args, designJSON)
	}
	cmd := exec.Command("python3", args...)

	done := make(chan error, 1)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start python: %w", err)
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("python execution failed: %v, stderr: %s", err, stderr.String())
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("python script timed out after 30s")
	}

	if _, err := os.Stat(plotPath); os.IsNotExist(err) {
		return "", fmt.Errorf("plot file not generated at %s, stdout: %s, stderr: %s", plotPath, stdout.String(), stderr.String())
	}

	return fmt.Sprintf("/plots/%s_plot%s", scriptID, ext), nil
}

// GeneratePlotScriptLLM asks the LLM to write a Python visualization script
// based on the dataset profile, user prompt, and target library. The generated
// code is validated by the sandbox validator before being returned.
//
// If the LLM is not configured, returns an empty string and nil error
// (caller should fall back to GeneratePlotScript).
func (pb *PythonBridge) GeneratePlotScriptLLM(scriptID, prompt, profileJSON, vizType, designJSON string) (string, error) {
	if !pb.llmConfig.Enabled || pb.llmConfig.APIKey == "" || pb.llmConfig.BaseURL == "" {
		return "", nil
	}

	code, err := pb.callLLMForCode(prompt, profileJSON, vizType, designJSON)
	if err != nil {
		return "", fmt.Errorf("LLM code generation failed: %w", err)
	}

	// Validate the generated code
	result := pb.validator.Validate(code)
	if !result.OK {
		return "", fmt.Errorf("sandbox validation failed: %s", strings.Join(result.Violations, "; "))
	}

	// The LLM sometimes omits required imports (e.g. `import matplotlib.pyplot as
	// plt`) or uses `design` instead of `_design`. Rather than reject otherwise
	// valid code, prepend a guaranteed-safe prologue that provides the common
	// imports and design globals. This runs after validation so the untrusted
	// code is still the only thing the sandbox inspects.
	return llmScriptPrologue(vizType) + "\n" + code, nil
}

// llmScriptPrologue returns a safe Python header that guarantees the standard
// visualization imports and design config are available, so LLM-generated code
// that forgets `import matplotlib.pyplot as plt` or uses `design`/`_design`
// still runs. Library imports are best-effort (guarded) so a missing optional
// dependency does not break scripts that don't need it.
func llmScriptPrologue(vizType string) string {
	return `import sys, json
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import pandas as pd
import numpy as np
try:
    from bokeh.plotting import figure as _bk_figure, output_file as _bk_output_file, save as _bk_save
    import bokeh
except Exception:
    pass
try:
    import plotly.graph_objects as go
    import plotly.io as pio
except Exception:
    pass
try:
    _design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
except Exception:
    _design = {}
design = _design
_accent = _design.get('accentColor', '#6366f1')
`
}

// callLLMForCode sends a request to the LLM asking for Python visualization code
// using the specified library (matplotlib, bokeh, or plotly).
func (pb *PythonBridge) callLLMForCode(prompt, profileJSON, vizType, designJSON string) (string, error) {
	var systemPrompt string
	switch vizType {
	case VizTypeBokeh:
		systemPrompt = `You are a Python data visualization expert. Write a complete, self-contained Python script using pandas and bokeh.

CRITICAL RULES:
- The script MUST read the CSV file path from sys.argv[1]
- The script MUST save the plot to the path in sys.argv[2] using output_file(sys.argv[2]) and save()
- sys.argv[3] (if present) contains a JSON design config. Parse it and apply:
  import json
  _design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
  _accent = _design.get('accentColor', '#6366f1')
- Use _accent as the primary color for all plot elements (bars, lines, markers, fills)
- Do NOT use matplotlib or plotly; only use bokeh and pandas
- Only use these libraries: pandas, bokeh, numpy, json, sys, datetime
- Do NOT import: os, subprocess, socket, urllib, requests, shutil, pickle, ctypes, threading, multiprocessing, matplotlib
- Do NOT use: eval, exec, compile, open(), __import__, globals, locals
- Do NOT make network calls or access the filesystem except reading sys.argv[1] and saving to sys.argv[2]
- The script should be a complete runnable .py file (include all imports)
- Output ONLY the Python code, no markdown, no code fences, no explanations`

	case VizTypePlotly:
		systemPrompt = `You are a Python data visualization expert. Write a complete, self-contained Python script using pandas and plotly.

CRITICAL RULES:
- The script MUST read the CSV file path from sys.argv[1]
- The script MUST save the plot to the path in sys.argv[2] using plotly.io.write_html(fig, sys.argv[2])
- sys.argv[3] (if present) contains a JSON design config. Parse it and apply:
  import json
  _design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
  _accent = _design.get('accentColor', '#6366f1')
- Use _accent as the primary color for all plot elements (bars, lines, markers, fills) via color_discrete_sequence=[_accent] or similar
- Do NOT use matplotlib or bokeh; only use plotly and pandas
- Only use these libraries: pandas, plotly, numpy, json, sys, datetime
- Do NOT import: os, subprocess, socket, urllib, requests, shutil, pickle, ctypes, threading, multiprocessing, matplotlib
- Do NOT use: eval, exec, compile, open(), __import__, globals, locals
- Do NOT make network calls or access the filesystem except reading sys.argv[1] and saving to sys.argv[2]
- The script should be a complete runnable .py file (include all imports)
- Output ONLY the Python code, no markdown, no code fences, no explanations`

	default: // matplotlib
		systemPrompt = `You are a Python data visualization expert. Write a complete, self-contained Python script using pandas, matplotlib, and seaborn.

CRITICAL RULES:
- The script MUST read the CSV file path from sys.argv[1]
- The script MUST save the plot to the path in sys.argv[2] using plt.savefig(sys.argv[2], dpi=150, bbox_inches='tight')
- Use matplotlib.use('Agg') at the very top (after imports)
- sys.argv[3] (if present) contains a JSON design config. Parse it and apply:
  import json
  _design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
  _accent = _design.get('accentColor', '#6366f1')
- Use _accent as the primary color for all plot elements (bars, lines, markers, fills)
- Only use these libraries: pandas, matplotlib, seaborn, numpy, json, sys, datetime
- Do NOT import: os, subprocess, socket, urllib, requests, shutil, pickle, ctypes, threading, multiprocessing
- Do NOT use: eval, exec, compile, open(), __import__, globals, locals
- Do NOT make network calls or access the filesystem except reading sys.argv[1] and saving to sys.argv[2]
- The script should be a complete runnable .py file (include all imports)
- Output ONLY the Python code, no markdown, no code fences, no explanations`
	}

	userPrompt := fmt.Sprintf(`User question: %s

Dataset profile (JSON): %s
Design config (JSON): %s

Write a Python visualization script that best answers the user's question about this data. Use the accent color from the design config for all plot elements.`, prompt, profileJSON, designJSON)

	model := pb.llmConfig.Model
	if model == "" {
		model = "stepfun-ai/step-3.7-flash"
	}
	maxTokens := pb.llmConfig.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	temperature := pb.llmConfig.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	timeout := pb.llmConfig.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}

	req, err := http.NewRequest(http.MethodPost, pb.llmConfig.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+normalizeBearer(pb.llmConfig.APIKey))

	client := &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: pb.httpClient.Transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return "", fmt.Errorf("parse LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	code := llmResp.Choices[0].Message.Content
	code = stripCodeFences(code)
	return code, nil
}

// GeneratePlotScript creates a deterministic Python script for visualizing the given dataset
// using the specified library (matplotlib, bokeh, or plotly).
// This is the fallback when LLM is not available or fails.
// designJSON is an optional JSON string with visual design settings.
func (pb *PythonBridge) GeneratePlotScript(scriptID, prompt, vizType, designJSON string) string {
	dc := defaultDesignConfig()
	if designJSON != "" {
		_ = json.Unmarshal([]byte(designJSON), &dc)
	}
	switch vizType {
	case VizTypeBokeh:
		return pb.bokehTemplate(dc)
	case VizTypePlotly:
		return pb.plotlyTemplate(dc)
	default:
		return pb.matplotlibTemplate(dc)
	}
}

func (pb *PythonBridge) matplotlibTemplate(dc DesignConfig) string {
	accent := dc.hexAccent()
	return fmt.Sprintf(`import sys
import json
import pandas as pd
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import seaborn as sns
from math import pi

if len(sys.argv) < 3:
    print("Error: CSV path or plot path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]
plot_path = sys.argv[2]

# Design config
_design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
_accent = _design.get('accentColor', '') or '%[1]s'
_font_family = _design.get('fontFamily', '') or 'sans-serif'
_font_size = _design.get('fontSize', '') or 'medium'
_chart_type = _design.get('chartType', '') or 'auto'
_x_axis = _design.get('xAxis', '')
_y_axis = _design.get('yAxis', '')
_aggregation = _design.get('aggregation', '') or 'sum'

# Aggregation mapping
_agg_funcs = {'sum': 'sum', 'mean': 'mean', 'median': 'median', 'min': 'min', 'max': 'max', 'count': 'count', 'none': None}

try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

sns.set_style("whitegrid")
sns.set_context("notebook", font_scale={"small": 0.8, "medium": 1.0, "large": 1.2}.get(_font_size, 1.0))
plt.rcParams['font.family'] = _font_family

numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]
all_cols = data.columns.tolist()

def do_agg(series, func_name):
    if func_name == 'sum': return series.sum()
    elif func_name == 'mean': return series.mean()
    elif func_name == 'median': return series.median()
    elif func_name == 'min': return series.min()
    elif func_name == 'max': return series.max()
    elif func_name == 'count': return series.count()
    else: return series

_agg = _agg_funcs.get(_aggregation, 'sum')

def draw_gauge_chart(values, label):
    val = values.mean() if len(values) > 0 else 0
    mn = values.min() if len(values) > 0 else 0
    mx = values.max() if len(values) > 0 else 100
    rng = mx - mn if mx > mn else 1
    frac = (val - mn) / rng
    frac = max(0, min(1, frac))
    fig, ax = plt.subplots(figsize=(8, 6))
    ax.set_xlim(-1.5, 1.5)
    ax.set_ylim(-0.6, 1.5)
    bg = mpatches.Wedge((0, 0), 1, 0, 180, facecolor='lightgray', edgecolor='white', linewidth=2)
    ax.add_patch(bg)
    fg = mpatches.Wedge((0, 0), 1, 0, 180 * frac, facecolor=_accent, edgecolor='white', linewidth=2, alpha=0.85)
    ax.add_patch(fg)
    ax.plot([0, 0.85 * np.cos(frac * pi)], [0, 0.85 * np.sin(frac * pi)], color='#1a1a2e', linewidth=3)
    ax.plot([0, -0.1 * np.cos(frac * pi)], [0, -0.1 * np.sin(frac * pi)], color='#1a1a2e', linewidth=3)
    ax.plot([0, 0.1 * np.cos(frac * pi)], [0, 0.1 * np.sin(frac * pi)], color='#1a1a2e', linewidth=3)
    ax.add_patch(plt.Circle((0, 0), 0.08, color='#1a1a2e', zorder=5))
    ax.text(0, -0.15, f'{val:,.2f}', ha='center', va='center', fontsize=28, fontweight='bold', color='#1a1a2e')
    ax.text(0, -0.35, label, ha='center', va='center', fontsize=13, color='#64748b')
    ax.text(-1.2, -0.05, f'{mn:,.2f}', ha='center', fontsize=10, color='#94a3b8')
    ax.text(1.2, -0.05, f'{mx:,.2f}', ha='center', fontsize=10, color='#94a3b8')
    ax.axis('off')
    plt.tight_layout()
    return fig

def draw_table_chart(grouped_df, x_label, y_label, agg_label):
    tbl_data = grouped_df.reset_index()
    tbl_data.columns = [x_label, y_label]
    fig, ax = plt.subplots(figsize=(max(6, len(tbl_data) * 0.8 + 2), max(4, len(tbl_data) * 0.4 + 1)))
    ax.axis('off')
    cell_text = [[str(r[0]), f'{r[1]:,.2f}'] for r in tbl_data.values]
    header = [x_label, f'{agg_label} of {y_label}' if agg_label != 'none' else y_label]
    table = ax.table(cellText=cell_text, colLabels=header, loc='center', cellLoc='left')
    table.auto_set_font_size(False)
    table.set_fontsize(9)
    table.auto_set_column_width(col=list(range(len(header))))
    for i in range(len(header)):
        table[0, i].set_facecolor(_accent)
        table[0, i].set_text_props(color='white', fontweight='bold')
    for j in range(1, len(cell_text) + 1):
        for i in range(len(header)):
            table[j, i].set_facecolor('#f8fafc' if j %% 2 == 0 else 'white')
    plt.title(f'{agg_label.capitalize() if agg_label != "none" else "Values"} of {y_label} by {x_label}', fontsize=13, pad=20)
    plt.tight_layout()
    return fig

# --- Determine effective columns ---
if _x_axis and _x_axis in all_cols:
    eff_x = _x_axis
else:
    eff_x = ''
if _y_axis and _y_axis in numeric_cols:
    eff_y = _y_axis
else:
    eff_y = ''

# --- Chart type dispatch ---
fig = None

if _chart_type == 'gauge':
    if eff_y:
        vals = data[eff_y].dropna()
        fig = draw_gauge_chart(vals, eff_y)
    elif len(numeric_cols) >= 1:
        vals = data[numeric_cols[0]].dropna()
        fig = draw_gauge_chart(vals, numeric_cols[0])
    else:
        _chart_type = 'auto'

if _chart_type == 'matrix':
    if eff_x and eff_y:
        if _agg and _agg in ['sum', 'mean', 'median', 'min', 'max']:
            grouped = data.groupby(eff_x)[eff_y].agg(_agg).sort_values(ascending=False).head(20)
        elif _agg == 'count':
            grouped = data.groupby(eff_x)[eff_y].count().sort_values(ascending=False).head(20)
        else:
            grouped = data.groupby(eff_x)[eff_y].sum().sort_values(ascending=False).head(20)
        fig = draw_table_chart(grouped, eff_x, eff_y, _aggregation)
    elif cat_cols and numeric_cols:
        cx = cat_cols[0]
        cy = numeric_cols[0]
        grouped = data.groupby(cx)[cy].sum().sort_values(ascending=False).head(20)
        fig = draw_table_chart(grouped, cx, cy, _aggregation)
    elif numeric_cols:
        grouped = data[numeric_cols].describe().T
        fig, ax = plt.subplots(figsize=(10, max(4, len(grouped) * 0.5 + 2)))
        ax.axis('off')
        cell_text = [[idx] + [f'{v:,.2f}' if isinstance(v, (int, float)) else str(v) for v in row] for idx, row in grouped.iterrows()]
        header = ['Column'] + list(grouped.columns)
        table = ax.table(cellText=cell_text, colLabels=header, loc='center', cellLoc='left')
        table.auto_set_font_size(False)
        table.set_fontsize(8)
        for i in range(len(header)):
            table[0, i].set_facecolor(_accent)
            table[0, i].set_text_props(color='white', fontweight='bold')
        plt.title('Summary Statistics', fontsize=13, pad=20)
        plt.tight_layout()
    else:
        _chart_type = 'auto'

if fig is None and _chart_type == 'stacked_bar':
    if eff_x and eff_y:
        if _agg:
            grouped = data.groupby(eff_x)[eff_y].agg(_agg) if _agg != 'count' else data.groupby(eff_x)[eff_y].count()
        else:
            grouped = data.groupby(eff_x)[eff_y].sum()
        grouped = grouped.sort_values(ascending=False).head(10)
        fig, ax = plt.subplots(figsize=(10, 6))
        ax = grouped.plot(kind='bar', color=_accent, ax=ax, legend=False)
        plt.title(f'{_aggregation.capitalize() if _agg else "Sum"} of {eff_y} by {eff_x}')
        plt.xlabel(eff_x)
        plt.ylabel(eff_y)
        plt.xticks(rotation=45)
    elif len(numeric_cols) >= 2 and cat_cols:
        fig, ax = plt.subplots(figsize=(12, 6))
        top_cats = data.groupby(cat_cols[0])[numeric_cols[0]].sum().sort_values(ascending=False).head(5).index
        subset = data[data[cat_cols[0]].isin(top_cats)]
        pivot = subset.pivot_table(index=cat_cols[0], columns=numeric_cols[1] if len(numeric_cols) > 1 else cat_cols[1] if len(cat_cols) > 1 else 'index',
                                   values=numeric_cols[0], aggfunc=_agg if _agg else 'sum')
        pivot.plot(kind='bar', stacked=True, ax=ax, colormap='viridis')
        plt.title(f'Stacked Bar: {numeric_cols[0]} by {cat_cols[0]}')
        plt.xlabel(cat_cols[0])
        plt.ylabel(numeric_cols[0])
        plt.xticks(rotation=45)
        plt.legend(bbox_to_anchor=(1.02, 1), loc='upper left', fontsize=8)
    elif cat_cols and numeric_cols:
        fig, ax = plt.subplots(figsize=(10, 6))
        grouped = data.groupby(cat_cols[0])[numeric_cols[0]].sum().sort_values(ascending=False).head(10)
        ax = grouped.plot(kind='bar', color=_accent, ax=ax, legend=False)
        plt.title(f'{numeric_cols[0]} by {cat_cols[0]}')
        plt.xlabel(cat_cols[0])
        plt.ylabel(numeric_cols[0])
        plt.xticks(rotation=45)
    else:
        _chart_type = 'auto'

if fig is None and _chart_type == 'histogram':
    if eff_y:
        fig, ax = plt.subplots(figsize=(10, 6))
        ax.hist(data[eff_y].dropna(), bins=20, color=_accent, edgecolor='white')
        plt.title(f'Distribution of {eff_y}')
        plt.xlabel(eff_y)
        plt.ylabel('Frequency')
    elif len(numeric_cols) >= 1:
        fig, ax = plt.subplots(figsize=(10, 6))
        ax.hist(data[numeric_cols[0]].dropna(), bins=20, color=_accent, edgecolor='white')
        plt.title(f'Distribution of {numeric_cols[0]}')
        plt.xlabel(numeric_cols[0])
        plt.ylabel('Frequency')
    else:
        _chart_type = 'auto'

if fig is None and _chart_type == 'bar':
    if eff_x and eff_y:
        if _agg:
            grouped = data.groupby(eff_x)[eff_y].agg(_agg) if _agg != 'count' else data.groupby(eff_x)[eff_y].count()
        else:
            grouped = data.groupby(eff_x)[eff_y].sum()
        grouped = grouped.sort_values(ascending=False).head(15)
        fig, ax = plt.subplots(figsize=(10, 6))
        ax = grouped.plot(kind='bar', color=_accent, ax=ax)
        plt.title(f'{_aggregation.capitalize() if _agg else "Sum"} of {eff_y} by {eff_x}')
        plt.xlabel(eff_x)
        plt.ylabel(eff_y)
        plt.xticks(rotation=45)
        for container in ax.containers:
            ax.bar_label(container, fmt='%%.0f', padding=3, fontsize=8)
    elif cat_cols and numeric_cols:
        fig, ax = plt.subplots(figsize=(10, 6))
        grouped = data.groupby(cat_cols[0])[numeric_cols[0]].sum().sort_values(ascending=False).head(10)
        ax = grouped.plot(kind='bar', color=_accent, ax=ax)
        plt.title(f'Total {numeric_cols[0]} by {cat_cols[0]}')
        plt.xlabel(cat_cols[0])
        plt.ylabel(numeric_cols[0])
        plt.xticks(rotation=45)
        for container in ax.containers:
            ax.bar_label(container, fmt='%%.0f', padding=3, fontsize=8)
    else:
        _chart_type = 'auto'

if fig is None and _chart_type == 'line':
    if eff_x and eff_y:
        data[eff_x] = pd.to_datetime(data[eff_x], errors='coerce')
        sorted_data = data.sort_values(eff_x).dropna(subset=[eff_x, eff_y])
        fig, ax = plt.subplots(figsize=(10, 6))
        if _agg and _agg != 'none':
            sorted_data = sorted_data.groupby(eff_x)[eff_y].agg(_agg).reset_index()
        ax.plot(sorted_data[eff_x], sorted_data[eff_y], marker='o', linewidth=2, color=_accent)
        plt.title(f'{eff_y} over {eff_x}')
        plt.xlabel(eff_x)
        plt.ylabel(eff_y)
        plt.xticks(rotation=45)
    elif date_cols and numeric_cols:
        dc = date_cols[0]
        mc = eff_y if eff_y else numeric_cols[0]
        data[dc] = pd.to_datetime(data[dc], errors='coerce')
        sorted_data = data.sort_values(dc).dropna(subset=[dc, mc])
        fig, ax = plt.subplots(figsize=(10, 6))
        ax.plot(sorted_data[dc], sorted_data[mc], marker='o', linewidth=2, color=_accent)
        plt.title(f'{mc} over {dc}')
        plt.xlabel(dc)
        plt.ylabel(mc)
        plt.xticks(rotation=45)
    else:
        _chart_type = 'auto'

# --- Auto-detect (fallback / default) ---
if fig is None:
    plt.figure(figsize=(10, 6))
    if len(numeric_cols) >= 2:
        if date_cols:
            date_col = date_cols[0]
            metric_col = eff_y if eff_y else numeric_cols[0]
            data[date_col] = pd.to_datetime(data[date_col], errors='coerce')
            sorted_data = data.sort_values(date_col)
            plt.plot(sorted_data[date_col], sorted_data[metric_col], marker='o', linewidth=2, color=_accent)
            plt.title(f'{metric_col} over {date_col}')
            plt.xlabel(date_col)
            plt.ylabel(metric_col)
            plt.xticks(rotation=45)
        else:
            if cat_cols:
                cat_col = eff_x if eff_x else cat_cols[0]
                metric_col = eff_y if eff_y else numeric_cols[0]
                grouped = data.groupby(cat_col)[metric_col].sum().sort_values(ascending=False).head(10)
                ax = grouped.plot(kind='bar', color=_accent)
                plt.title(f'Total {metric_col} by {cat_col}')
                plt.xlabel(cat_col)
                plt.ylabel(metric_col)
                plt.xticks(rotation=45)
                for container in ax.containers:
                    ax.bar_label(container, fmt='%%.0f', padding=3, fontsize=8)
            else:
                plt.hist(data[numeric_cols[0]].dropna(), bins=20, color=_accent, edgecolor='white')
                plt.title(f'Distribution of {numeric_cols[0]}')
                plt.xlabel(numeric_cols[0])
                plt.ylabel('Frequency')
    elif len(numeric_cols) == 1:
        plt.hist(data[numeric_cols[0]].dropna(), bins=20, color=_accent, edgecolor='white')
        plt.title(f'Distribution of {numeric_cols[0]}')
        plt.xlabel(numeric_cols[0])
        plt.ylabel('Frequency')
    else:
        plt.text(0.5, 0.5, 'No numeric columns found in dataset', ha='center', va='center', fontsize=14)
        plt.title('Dataset Overview')

plt.tight_layout()
plt.savefig(plot_path, dpi=150, bbox_inches='tight')
print(f"Plot saved to {plot_path}")
`, accent)
}

func (pb *PythonBridge) bokehTemplate(dc DesignConfig) string {
	accent := dc.hexAccent()
	return fmt.Sprintf(`import sys
import json
import pandas as pd
from bokeh.plotting import figure, output_file, save
from bokeh.io import output_file, save
from bokeh.models import ColumnDataSource, HoverTool, NumeralTickFormatter
from bokeh.layouts import column
from bokeh.transform import factor_cmap
from bokeh.palettes import Category10, Viridis
import numpy as np
from math import pi

if len(sys.argv) < 3:
    print("Error: CSV path or plot path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]
plot_path = sys.argv[2]

# Design config
_design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
_accent = _design.get('accentColor', '') or '%[1]s'
_chart_type = _design.get('chartType', '') or 'auto'
_x_axis = _design.get('xAxis', '')
_y_axis = _design.get('yAxis', '')
_aggregation = _design.get('aggregation', '') or 'sum'

try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]
all_cols = data.columns.tolist()

_agg_funcs = {'sum': 'sum', 'mean': 'mean', 'median': 'median', 'min': 'min', 'max': 'max', 'count': 'count', 'none': None}
_agg = _agg_funcs.get(_aggregation, 'sum')

TOOLS = "pan,wheel_zoom,box_zoom,reset,hover,save"

eff_x = _x_axis if _x_axis in all_cols else ''
eff_y = _y_axis if _y_axis in numeric_cols else ''

p = None

if _chart_type == 'histogram':
    col = eff_y if eff_y else (numeric_cols[0] if numeric_cols else None)
    if col:
        hist, edges = np.histogram(data[col].dropna(), bins=20)
        source = ColumnDataSource(data=dict(top=hist, left=edges[:-1], right=edges[1:]))
        p = figure(title=f'Distribution of {col}', x_axis_label=col, y_axis_label='Frequency',
                   tools=TOOLS, width=900, height=500, toolbar_location='above')
        p.quad(top='top', bottom=0, left='left', right='right', source=source, fill_color=_accent, line_color='white')

if p is None and _chart_type == 'stacked_bar':
    if len(numeric_cols) >= 2 and cat_cols:
        top_cats = data.groupby(cat_cols[0])[numeric_cols[0]].sum().sort_values(ascending=False).head(5).index
        subset = data[data[cat_cols[0]].isin(top_cats)]
        pivot = subset.pivot_table(index=cat_cols[0], columns=numeric_cols[1] if len(numeric_cols) > 1 else cat_cols[1] if len(cat_cols) > 1 else 'index',
                                   values=numeric_cols[0], aggfunc=_agg if _agg else 'sum').fillna(0)
        cats = pivot.index.tolist()
        cols = pivot.columns.tolist()
        palette = Viridis[len(cols)] if len(cols) <= 11 else Category10[10]
        p = figure(x_range=cats, title=f'Stacked Bar: {numeric_cols[0]} by {cat_cols[0]}', tools=TOOLS,
                   width=900, height=500, toolbar_location='above')
        p.xgrid.grid_line_color = None
        p.xaxis.major_label_orientation = pi/4
        for i, col in enumerate(cols):
            vals = pivot[col].tolist()
            color = palette[i] if i < len(palette) else _accent
            if i == 0:
                p.vbar(x=cats, top=vals, width=0.7, color=color, legend_label=str(col))
            else:
                bottom_vals = pivot[cols[:i]].sum(axis=1).tolist()
                p.vbar(x=cats, top=vals, bottom=bottom_vals, width=0.7, color=color, legend_label=str(col))
        p.legend.location = 'top_right'
    elif cat_cols and numeric_cols:
        cx = eff_x if eff_x else cat_cols[0]
        cy = eff_y if eff_y else numeric_cols[0]
        grouped = data.groupby(cx)[cy].agg(_agg) if _agg and _agg != 'count' else data.groupby(cx)[cy].count()
        grouped = grouped.sort_values(ascending=False).head(10)
        cats = grouped.index.tolist()
        vals = grouped.values.tolist()
        source = ColumnDataSource(data=dict(cats=cats, vals=vals))
        p = figure(x_range=cats, title=f'{_aggregation.capitalize() if _agg else "Sum"} of {cy} by {cx}', x_axis_label=cx, y_axis_label=cy,
                   tools=TOOLS, width=900, height=500, toolbar_location='above')
        p.vbar(x='cats', top='vals', source=source, width=0.7, color=_accent)
        p.xgrid.grid_line_color = None
        p.xaxis.major_label_orientation = pi/4

if p is None:
    if len(numeric_cols) >= 2:
        if date_cols:
            date_col = eff_x if eff_x else date_cols[0]
            metric_col = eff_y if eff_y else numeric_cols[0]
            df = data.copy()
            df[date_col] = pd.to_datetime(df[date_col], errors='coerce')
            df = df.sort_values(date_col).dropna(subset=[date_col, metric_col])
            source = ColumnDataSource(data=dict(x=df[date_col].tolist(), y=df[metric_col].tolist()))
            p = figure(title=f'{metric_col} over {date_col}', x_axis_label=date_col, y_axis_label=metric_col,
                       tools=TOOLS, x_axis_type='datetime', width=900, height=500, toolbar_location='above')
            p.line('x', 'y', source=source, line_width=2, color=_accent)
            p.circle('x', 'y', source=source, size=6, color=_accent, alpha=0.6)
            p.add_tools(HoverTool(tooltips=[('Date', '@x{%%F}'), ('Value', '@y{%%0,0.00}')], formatters={'@x': 'datetime'}))
        else:
            if cat_cols:
                cat_col = eff_x if eff_x else cat_cols[0]
                metric_col = eff_y if eff_y else numeric_cols[0]
                grouped = data.groupby(cat_col)[metric_col].agg(_agg) if _agg and _agg != 'count' else data.groupby(cat_col)[metric_col].count()
                grouped = grouped.sort_values(ascending=False).head(10)
                cats = grouped.index.tolist()
                vals = grouped.values.tolist()
                source = ColumnDataSource(data=dict(cats=cats, vals=vals))
                p = figure(x_range=cats, title=f'{_aggregation.capitalize() if _agg else "Total"} {metric_col} by {cat_col}', x_axis_label=cat_col, y_axis_label=metric_col,
                           tools=TOOLS, width=900, height=500, toolbar_location='above')
                p.vbar(x='cats', top='vals', source=source, width=0.7, color=_accent, legend_label=metric_col)
                p.xgrid.grid_line_color = None
                p.xaxis.major_label_orientation = pi/4
                p.add_tools(HoverTool(tooltips=[(cat_col, '@cats'), (metric_col, '@vals{%%0,0.00}')]))
            else:
                col = numeric_cols[0]
                hist, edges = np.histogram(data[col].dropna(), bins=20)
                source = ColumnDataSource(data=dict(top=hist, left=edges[:-1], right=edges[1:]))
                p = figure(title=f'Distribution of {col}', x_axis_label=col, y_axis_label='Frequency',
                           tools=TOOLS, width=900, height=500, toolbar_location='above')
                p.quad(top='top', bottom=0, left='left', right='right', source=source, fill_color=_accent, line_color='white')
    elif len(numeric_cols) == 1:
        col = numeric_cols[0]
        hist, edges = np.histogram(data[col].dropna(), bins=20)
        source = ColumnDataSource(data=dict(top=hist, left=edges[:-1], right=edges[1:]))
        p = figure(title=f'Distribution of {col}', x_axis_label=col, y_axis_label='Frequency',
                   tools=TOOLS, width=900, height=500, toolbar_location='above')
        p.quad(top='top', bottom=0, left='left', right='right', source=source, fill_color=_accent, line_color='white')
    else:
        from bokeh.io import show
        p = figure(title='Dataset Overview', tools=TOOLS, width=900, height=500, toolbar_location='above')
        p.text(x=0.5, y=0.5, text=['No numeric columns found in dataset'], text_font_size='14pt', text_align='center', text_baseline='middle')

if p:
    output_file(plot_path)
    save(p)
print(f"Plot saved to {plot_path}")
`, accent)
}

func (pb *PythonBridge) plotlyTemplate(dc DesignConfig) string {
	accent := dc.hexAccent()
	return fmt.Sprintf(`import sys
import json
import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
import plotly.io as pio
import numpy as np

if len(sys.argv) < 3:
    print("Error: CSV path or plot path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]
plot_path = sys.argv[2]

# Design config
_design = json.loads(sys.argv[3]) if len(sys.argv) > 3 else {}
_accent = _design.get('accentColor', '') or '%[1]s'
_chart_type = _design.get('chartType', '') or 'auto'
_x_axis = _design.get('xAxis', '')
_y_axis = _design.get('yAxis', '')
_aggregation = _design.get('aggregation', '') or 'sum'

try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]
all_cols = data.columns.tolist()

_agg_funcs = {'sum': 'sum', 'mean': 'mean', 'median': 'median', 'min': 'min', 'max': 'max', 'count': 'count', 'none': None}
_agg = _agg_funcs.get(_aggregation, 'sum')

eff_x = _x_axis if _x_axis in all_cols else ''
eff_y = _y_axis if _y_axis in numeric_cols else ''

fig = None

if _chart_type == 'histogram':
    col = eff_y if eff_y else (numeric_cols[0] if numeric_cols else None)
    if col:
        fig = px.histogram(data, x=col, nbins=20, title=f'Distribution of {col}',
                           template='plotly_white', color_discrete_sequence=[_accent])
        fig.update_layout(width=900, height=500, bargap=0.05)

if fig is None and (_chart_type == 'stacked_bar' or _chart_type == 'bar'):
    if eff_x and eff_y:
        grouped = data.groupby(eff_x)[eff_y].agg(_agg) if _agg and _agg != 'count' else data.groupby(eff_x)[eff_y].count()
        grouped = grouped.sort_values(ascending=False).head(15).reset_index()
        if _chart_type == 'stacked_bar' and len(numeric_cols) >= 2:
            fig = px.bar(grouped, x=eff_x, y=eff_y, title=f'{_aggregation.capitalize() if _agg else "Sum"} of {eff_y} by {eff_x}',
                         color=eff_x, barmode='stack', template='plotly_white', color_discrete_sequence=[_accent])
        else:
            fig = px.bar(grouped, x=eff_x, y=eff_y, title=f'{_aggregation.capitalize() if _agg else "Sum"} of {eff_y} by {eff_x}',
                         color=eff_x, template='plotly_white', color_discrete_sequence=[_accent])
        fig.update_layout(width=900, height=500, showlegend=False, xaxis_tickangle=-45)
    elif cat_cols and numeric_cols:
        cx = eff_x if eff_x else cat_cols[0]
        cy = eff_y if eff_y else numeric_cols[0]
        grouped = data.groupby(cx)[cy].agg(_agg) if _agg and _agg != 'count' else data.groupby(cx)[cy].count()
        grouped = grouped.sort_values(ascending=False).head(10).reset_index()
        fig = px.bar(grouped, x=cx, y=cy, title=f'{_aggregation.capitalize() if _agg else "Total"} {cy} by {cx}',
                     color=cx, template='plotly_white', color_discrete_sequence=[_accent])
        fig.update_layout(width=900, height=500, showlegend=False, xaxis_tickangle=-45)

if fig is None and _chart_type == 'matrix':
    if eff_x and eff_y:
        grouped = data.groupby(eff_x)[eff_y].agg(_agg) if _agg and _agg != 'count' else data.groupby(eff_x)[eff_y].count()
        grouped = grouped.sort_values(ascending=False).head(20).reset_index()
        header = [eff_x, f'{_aggregation.capitalize() if _agg else "Sum"} of {eff_y}']
        fig = go.Figure(data=[go.Table(
            header=dict(values=header, fill_color=_accent, font=dict(color='white'), align='left'),
            cells=dict(values=[grouped[eff_x].tolist(), [f'{v:,.2f}' for v in grouped[eff_y].tolist()]],
                       fill_color=[['#f8fafc', 'white'] * len(grouped)], align='left')
        )])
        fig.update_layout(title=f'{header[1]} by {eff_x}', width=900, height=max(400, 40 * len(grouped)))
    elif cat_cols and numeric_cols:
        cx = cat_cols[0]
        cy = numeric_cols[0]
        grouped = data.groupby(cx)[cy].sum().sort_values(ascending=False).head(20).reset_index()
        fig = go.Figure(data=[go.Table(
            header=dict(values=[cx, f'Sum of {cy}'], fill_color=_accent, font=dict(color='white'), align='left'),
            cells=dict(values=[grouped[cx].tolist(), [f'{v:,.2f}' for v in grouped[cy].tolist()]], align='left')
        )])
        fig.update_layout(title=f'Sum of {cy} by {cx}', width=900, height=max(400, 40 * len(grouped)))

if fig is None and _chart_type == 'gauge':
    col = eff_y if eff_y else (numeric_cols[0] if numeric_cols else None)
    if col is not None:
        val = data[col].mean()
        mn = data[col].min()
        mx = data[col].max()
        fig = go.Figure(go.Indicator(
            mode='gauge+number',
            value=val,
            title={'text': col},
            number={'font': {'size': 40}},
            gauge={
                'axis': {'range': [mn, mx]},
                'bar': {'color': _accent},
                'steps': [
                    {'range': [mn, mn + (mx - mn) * 0.5], 'color': '#f1f5f9'},
                    {'range': [mn + (mx - mn) * 0.5, mx], 'color': '#e2e8f0'}
                ],
                'threshold': {
                    'line': {'color': 'red', 'width': 4},
                    'thickness': 0.75,
                    'value': val
                }
            }
        ))
        fig.update_layout(width=700, height=450)

if fig is None:
    if len(numeric_cols) >= 2:
        if date_cols:
            date_col = eff_x if eff_x else date_cols[0]
            metric_col = eff_y if eff_y else numeric_cols[0]
            df = data.copy()
            df[date_col] = pd.to_datetime(df[date_col], errors='coerce')
            df = df.sort_values(date_col).dropna(subset=[date_col, metric_col])
            fig = px.line(df, x=date_col, y=metric_col, title=f'{metric_col} over {date_col}',
                          markers=True, template='plotly_white', color_discrete_sequence=[_accent])
            fig.update_layout(width=900, height=500, hovermode='x unified')
        else:
            if cat_cols:
                cat_col = eff_x if eff_x else cat_cols[0]
                metric_col = eff_y if eff_y else numeric_cols[0]
                grouped = data.groupby(cat_col)[metric_col].agg(_agg) if _agg and _agg != 'count' else data.groupby(cat_col)[metric_col].count()
                grouped = grouped.sort_values(ascending=False).head(10).reset_index()
                fig = px.bar(grouped, x=cat_col, y=metric_col, title=f'{_aggregation.capitalize() if _agg else "Total"} {metric_col} by {cat_col}',
                             color=cat_col, color_discrete_sequence=[_accent], template='plotly_white')
                fig.update_layout(width=900, height=500, showlegend=False, xaxis_tickangle=-45)
            else:
                col = numeric_cols[0]
                fig = px.histogram(data, x=col, nbins=20, title=f'Distribution of {col}',
                                   template='plotly_white', color_discrete_sequence=[_accent])
                fig.update_layout(width=900, height=500, bargap=0.05)
    elif len(numeric_cols) == 1:
        col = numeric_cols[0]
        fig = px.histogram(data, x=col, nbins=20, title=f'Distribution of {col}',
                           template='plotly_white', color_discrete_sequence=[_accent])
        fig.update_layout(width=900, height=500, bargap=0.05)
    else:
        fig = px.scatter(title='Dataset Overview', template='plotly_white')
        fig.add_annotation(text='No numeric columns found in dataset', showarrow=False,
                           font=dict(size=14), x=0.5, y=0.5, xref='paper', yref='paper')
        fig.update_layout(width=900, height=500)

pio.write_html(fig, plot_path, auto_open=False, include_plotlyjs='cdn')
print(f"Plot saved to {plot_path}")
`, accent)
}

// stripCodeFences removes ```python ... ``` or ``` ... ``` wrappers from LLM output.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Find the newline after the opening ```
		if idx := strings.Index(s[3:], "\n"); idx >= 0 {
			s = s[3+idx+1:]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

// normalizeBearer strips any "Bearer " prefix and surrounding quotes from the token.
func normalizeBearer(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"")
	key = strings.Trim(key, "'")
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	return key
}
