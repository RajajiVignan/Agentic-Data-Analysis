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
	Enabled       bool
	APIKey        string
	BaseURL       string
	Model         string
	MaxTokens     int
	Temperature   float64
	TimeoutSec    int
}

// VizType constants
const (
	VizTypeMatplotlib = "matplotlib"
	VizTypeBokeh      = "bokeh"
	VizTypePlotly     = "plotly"
)

// PythonBridge handles execution of Python visualization scripts.
type PythonBridge struct {
	plotsDir    string
	llmConfig   LLMConfig
	validator   *SandboxValidator
	httpClient  *http.Client
}

// NewPythonBridge creates a new Python bridge with the given plots directory.
func NewPythonBridge(plotsDir string) *PythonBridge {
	os.MkdirAll(plotsDir, 0755)
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
// The script receives csvPath as sys.argv[1] and plotPath as sys.argv[2].
// The vizType determines the output format: matplotlib -> .png, bokeh/plotly -> .html.
func (pb *PythonBridge) ExecuteScript(scriptID, scriptContent, csvPath, vizType string) (string, error) {
	scriptPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s.py", scriptID))
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}

	ext := ".png"
	if vizType == VizTypeBokeh || vizType == VizTypePlotly {
		ext = ".html"
	}
	plotPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s_plot%s", scriptID, ext))

	cmd := exec.Command("python3", scriptPath, csvPath, plotPath)

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
		cmd.Process.Kill()
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
func (pb *PythonBridge) GeneratePlotScriptLLM(scriptID, prompt, profileJSON, vizType string) (string, error) {
	if !pb.llmConfig.Enabled || pb.llmConfig.APIKey == "" || pb.llmConfig.BaseURL == "" {
		return "", nil
	}

	code, err := pb.callLLMForCode(prompt, profileJSON, vizType)
	if err != nil {
		return "", fmt.Errorf("LLM code generation failed: %w", err)
	}

	// Validate the generated code
	result := pb.validator.Validate(code)
	if !result.OK {
		return "", fmt.Errorf("sandbox validation failed: %s", strings.Join(result.Violations, "; "))
	}

	return code, nil
}

// callLLMForCode sends a request to the LLM asking for Python visualization code
// using the specified library (matplotlib, bokeh, or plotly).
func (pb *PythonBridge) callLLMForCode(prompt, profileJSON, vizType string) (string, error) {
	var systemPrompt string
	switch vizType {
	case VizTypeBokeh:
		systemPrompt = `You are a Python data visualization expert. Write a complete, self-contained Python script using pandas and bokeh.

CRITICAL RULES:
- The script MUST read the CSV file path from sys.argv[1]
- The script MUST save the plot to the path in sys.argv[2] using output_file(sys.argv[2]) and save()
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
- Only use these libraries: pandas, matplotlib, seaborn, numpy, json, sys, datetime
- Do NOT import: os, subprocess, socket, urllib, requests, shutil, pickle, ctypes, threading, multiprocessing
- Do NOT use: eval, exec, compile, open(), __import__, globals, locals
- Do NOT make network calls or access the filesystem except reading sys.argv[1] and saving to sys.argv[2]
- The script should be a complete runnable .py file (include all imports)
- Output ONLY the Python code, no markdown, no code fences, no explanations`
	}

	userPrompt := fmt.Sprintf(`User question: %s

Dataset profile (JSON): %s

Write a Python visualization script that best answers the user's question about this data.`, prompt, profileJSON)

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
		Timeout: time.Duration(timeout) * time.Second,
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
func (pb *PythonBridge) GeneratePlotScript(scriptID, prompt, vizType string) string {
	switch vizType {
	case VizTypeBokeh:
		return pb.bokehTemplate()
	case VizTypePlotly:
		return pb.plotlyTemplate()
	default:
		return pb.matplotlibTemplate()
	}
}

func (pb *PythonBridge) matplotlibTemplate() string {
	return `import sys
import pandas as pd
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import seaborn as sns

if len(sys.argv) < 3:
    print("Error: CSV path or plot path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]
plot_path = sys.argv[2]

try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

sns.set_style("whitegrid")
plt.figure(figsize=(10, 6))

numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]

if len(numeric_cols) >= 2:
    if date_cols:
        date_col = date_cols[0]
        metric_col = numeric_cols[0]
        data[date_col] = pd.to_datetime(data[date_col], errors='coerce')
        sorted_data = data.sort_values(date_col)
        plt.plot(sorted_data[date_col], sorted_data[metric_col], marker='o', linewidth=2)
        plt.title(f'{metric_col} over {date_col}')
        plt.xlabel(date_col)
        plt.ylabel(metric_col)
        plt.xticks(rotation=45)
    else:
        if cat_cols:
            cat_col = cat_cols[0]
            metric_col = numeric_cols[0]
            grouped = data.groupby(cat_col)[metric_col].sum().sort_values(ascending=False).head(10)
            ax = grouped.plot(kind='bar', color='#6366f1')
            plt.title(f'Total {metric_col} by {cat_col}')
            plt.xlabel(cat_col)
            plt.ylabel(metric_col)
            plt.xticks(rotation=45)
            for container in ax.containers:
                ax.bar_label(container, fmt='%.0f', padding=3, fontsize=8)
        else:
            plt.hist(data[numeric_cols[0]].dropna(), bins=20, color='#6366f1', edgecolor='white')
            plt.title(f'Distribution of {numeric_cols[0]}')
            plt.xlabel(numeric_cols[0])
            plt.ylabel('Frequency')
elif len(numeric_cols) == 1:
    plt.hist(data[numeric_cols[0]].dropna(), bins=20, color='#6366f1', edgecolor='white')
    plt.title(f'Distribution of {numeric_cols[0]}')
    plt.xlabel(numeric_cols[0])
    plt.ylabel('Frequency')
else:
    plt.text(0.5, 0.5, 'No numeric columns found in dataset', ha='center', va='center', fontsize=14)
    plt.title('Dataset Overview')

plt.tight_layout()
plt.savefig(plot_path, dpi=150, bbox_inches='tight')
print(f"Plot saved to {plot_path}")
`
}

func (pb *PythonBridge) bokehTemplate() string {
	return `import sys
import pandas as pd
from bokeh.plotting import figure, output_file, save
from bokeh.io import output_file, save
from bokeh.models import ColumnDataSource, HoverTool, NumeralTickFormatter
from bokeh.layouts import column
from bokeh.transform import factor_cmap
from bokeh.palettes import Category10
import numpy as np
import json
from math import pi

if len(sys.argv) < 3:
    print("Error: CSV path or plot path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]
plot_path = sys.argv[2]

try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]

TOOLS = "pan,wheel_zoom,box_zoom,reset,hover,save"

if len(numeric_cols) >= 2:
    if date_cols:
        date_col = date_cols[0]
        metric_col = numeric_cols[0]
        df = data.copy()
        df[date_col] = pd.to_datetime(df[date_col], errors='coerce')
        df = df.sort_values(date_col).dropna(subset=[date_col, metric_col])
        source = ColumnDataSource(data=dict(x=df[date_col].tolist(), y=df[metric_col].tolist()))
        p = figure(title=f'{metric_col} over {date_col}', x_axis_label=date_col, y_axis_label=metric_col,
                   tools=TOOLS, x_axis_type='datetime', width=900, height=500, toolbar_location='above')
        p.line('x', 'y', source=source, line_width=2, color='#6366f1')
        p.circle('x', 'y', source=source, size=6, color='#6366f1', alpha=0.6)
        p.add_tools(HoverTool(tooltips=[('Date', '@x{%F}'), ('Value', '@y{0,0.00}')], formatters={'@x': 'datetime'}))
        output_file(plot_path, title='Time Series')
        save(p)
    else:
        if cat_cols:
            cat_col = cat_cols[0]
            metric_col = numeric_cols[0]
            grouped = data.groupby(cat_col)[metric_col].sum().sort_values(ascending=False).head(10)
            cats = grouped.index.tolist()
            vals = grouped.values.tolist()
            source = ColumnDataSource(data=dict(cats=cats, vals=vals))
            p = figure(x_range=cats, title=f'Total {metric_col} by {cat_col}', x_axis_label=cat_col, y_axis_label=metric_col,
                       tools=TOOLS, width=900, height=500, toolbar_location='above')
            p.vbar(x='cats', top='vals', source=source, width=0.7, color='#6366f1', legend_label=metric_col)
            p.xgrid.grid_line_color = None
            p.xaxis.major_label_orientation = pi/4
            p.add_tools(HoverTool(tooltips=[(cat_col, '@cats'), (metric_col, '@vals{0,0.00}')]))
            output_file(plot_path, title='Bar Chart')
            save(p)
        else:
            col = numeric_cols[0]
            hist, edges = np.histogram(data[col].dropna(), bins=20)
            source = ColumnDataSource(data=dict(top=hist, left=edges[:-1], right=edges[1:]))
            p = figure(title=f'Distribution of {col}', x_axis_label=col, y_axis_label='Frequency',
                       tools=TOOLS, width=900, height=500, toolbar_location='above')
            p.quad(top='top', bottom=0, left='left', right='right', source=source, fill_color='#6366f1', line_color='white')
            output_file(plot_path, title='Histogram')
            save(p)
elif len(numeric_cols) == 1:
    col = numeric_cols[0]
    hist, edges = np.histogram(data[col].dropna(), bins=20)
    source = ColumnDataSource(data=dict(top=hist, left=edges[:-1], right=edges[1:]))
    p = figure(title=f'Distribution of {col}', x_axis_label=col, y_axis_label='Frequency',
               tools=TOOLS, width=900, height=500, toolbar_location='above')
    p.quad(top='top', bottom=0, left='left', right='right', source=source, fill_color='#6366f1', line_color='white')
    output_file(plot_path, title='Histogram')
    save(p)
else:
    from bokeh.io import show
    p = figure(title='Dataset Overview', tools=TOOLS, width=900, height=500, toolbar_location='above')
    p.text(x=0.5, y=0.5, text=['No numeric columns found in dataset'], text_font_size='14pt', text_align='center', text_baseline='middle')
    output_file(plot_path, title='Overview')
    save(p)

print(f"Plot saved to {plot_path}")
`
}

func (pb *PythonBridge) plotlyTemplate() string {
	return `import sys
import pandas as pd
import plotly.express as px
import plotly.io as pio
import numpy as np
import json

if len(sys.argv) < 3:
    print("Error: CSV path or plot path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]
plot_path = sys.argv[2]

try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]

if len(numeric_cols) >= 2:
    if date_cols:
        date_col = date_cols[0]
        metric_col = numeric_cols[0]
        df = data.copy()
        df[date_col] = pd.to_datetime(df[date_col], errors='coerce')
        df = df.sort_values(date_col).dropna(subset=[date_col, metric_col])
        fig = px.line(df, x=date_col, y=metric_col, title=f'{metric_col} over {date_col}',
                      markers=True, template='plotly_white')
        fig.update_layout(width=900, height=500, hovermode='x unified')
    else:
        if cat_cols:
            cat_col = cat_cols[0]
            metric_col = numeric_cols[0]
            grouped = data.groupby(cat_col)[metric_col].sum().sort_values(ascending=False).head(10).reset_index()
            fig = px.bar(grouped, x=cat_col, y=metric_col, title=f'Total {metric_col} by {cat_col}',
                         color=cat_col, color_discrete_sequence=['#6366f1'], template='plotly_white')
            fig.update_layout(width=900, height=500, showlegend=False, xaxis_tickangle=-45)
        else:
            col = numeric_cols[0]
            fig = px.histogram(data, x=col, nbins=20, title=f'Distribution of {col}',
                               template='plotly_white', color_discrete_sequence=['#6366f1'])
            fig.update_layout(width=900, height=500, bargap=0.05)
elif len(numeric_cols) == 1:
    col = numeric_cols[0]
    fig = px.histogram(data, x=col, nbins=20, title=f'Distribution of {col}',
                       template='plotly_white', color_discrete_sequence=['#6366f1'])
    fig.update_layout(width=900, height=500, bargap=0.05)
else:
    fig = px.scatter(title='Dataset Overview', template='plotly_white')
    fig.add_annotation(text='No numeric columns found in dataset', showarrow=False,
                       font=dict(size=14), x=0.5, y=0.5, xref='paper', yref='paper')
    fig.update_layout(width=900, height=500)

pio.write_html(fig, plot_path, auto_open=False, include_plotlyjs='cdn')
print(f"Plot saved to {plot_path}")
`
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
