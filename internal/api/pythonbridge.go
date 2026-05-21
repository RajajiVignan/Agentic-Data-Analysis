package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PythonBridge handles execution of Python visualization scripts.
type PythonBridge struct {
	plotsDir string
}

// NewPythonBridge creates a new Python bridge with the given plots directory.
func NewPythonBridge(plotsDir string) *PythonBridge {
	os.MkdirAll(plotsDir, 0755)
	return &PythonBridge{plotsDir: plotsDir}
}

// ExecuteScript runs a Python script and returns the path to the generated plot.
// The script is expected to save a plot as {scriptID}_plot.png in the plots directory.
// The csvPath is passed as a command-line argument to avoid string interpolation in the script.
func (pb *PythonBridge) ExecuteScript(scriptID, scriptContent, csvPath string) (string, error) {
	// Write script to a temp file
	scriptPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s.py", scriptID))
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}

	// Execute the script, passing csvPath as a CLI argument
	cmd := exec.Command("python3", scriptPath, csvPath)

	// Set a timeout via context
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

	// Check if the plot was generated
	plotPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s_plot.png", scriptID))
	if _, err := os.Stat(plotPath); os.IsNotExist(err) {
		return "", fmt.Errorf("plot file not generated at %s, stdout: %s, stderr: %s", plotPath, stdout.String(), stderr.String())
	}

	return fmt.Sprintf("/plots/%s_plot.png", scriptID), nil
}

// GeneratePlotScript creates a Python script for visualizing the given dataset.
// The script reads the CSV path from sys.argv[1], eliminating the need for
// string interpolation and preventing code injection via filenames.
func (pb *PythonBridge) GeneratePlotScript(scriptID, prompt string) string {
	plotPath := filepath.Join(pb.plotsDir, fmt.Sprintf("%s_plot.png", scriptID))

	return fmt.Sprintf(`import sys
import pandas as pd
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import seaborn as sns

# Read the dataset path from command-line argument (safe from injection)
if len(sys.argv) < 2:
    print("Error: CSV path not provided", file=sys.stderr)
    sys.exit(1)

csv_path = sys.argv[1]

# Read the dataset
try:
    data = pd.read_csv(csv_path)
except Exception as e:
    print(f"Error reading CSV: {e}", file=sys.stderr)
    sys.exit(1)

# Set style
sns.set_style("whitegrid")
plt.figure(figsize=(10, 6))

# Auto-generate visualization based on data
numeric_cols = data.select_dtypes(include='number').columns.tolist()
date_cols = [c for c in data.columns if 'date' in c.lower() or 'month' in c.lower() or 'year' in c.lower() or 'time' in c.lower()]
cat_cols = [c for c in data.columns if data[c].dtype == 'object' and c not in date_cols]

if len(numeric_cols) >= 2:
    # Scatter or line plot for two numeric columns
    if date_cols:
        # Time series: use first date col and first numeric col
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
        # Bar chart of first numeric column grouped by first categorical column
        if cat_cols:
            cat_col = cat_cols[0]
            metric_col = numeric_cols[0]
            grouped = data.groupby(cat_col)[metric_col].sum().sort_values(ascending=False).head(10)
            ax = grouped.plot(kind='bar', color='#6366f1')
            plt.title(f'Total {metric_col} by {cat_col}')
            plt.xlabel(cat_col)
            plt.ylabel(metric_col)
            plt.xticks(rotation=45)
            # Add value labels on bars
            for container in ax.containers:
                ax.bar_label(container, fmt='%%.0f', padding=3, fontsize=8)
        else:
            # Histogram of first numeric column
            plt.hist(data[numeric_cols[0]].dropna(), bins=20, color='#6366f1', edgecolor='white')
            plt.title(f'Distribution of {numeric_cols[0]}')
            plt.xlabel(numeric_cols[0])
            plt.ylabel('Frequency')
elif len(numeric_cols) == 1:
    # Single numeric column: histogram
    plt.hist(data[numeric_cols[0]].dropna(), bins=20, color='#6366f1', edgecolor='white')
    plt.title(f'Distribution of {numeric_cols[0]}')
    plt.xlabel(numeric_cols[0])
    plt.ylabel('Frequency')
else:
    # No numeric columns: show a text-based summary
    plt.text(0.5, 0.5, 'No numeric columns found in dataset', ha='center', va='center', fontsize=14)
    plt.title('Dataset Overview')

plt.tight_layout()
plt.savefig('%s', dpi=150, bbox_inches='tight')
print("Plot saved to %s")
`, plotPath, plotPath)
}
