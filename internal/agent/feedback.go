package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"insightpilot/internal/data"
)

const maxFeedbackIterations = 3

// FeedbackLoop orchestrates the plan → execute → validate → refine cycle.
// It uses a PlannerAgent to create plans, executes tools deterministically,
// and uses a ValidatorAgent to check results — iterating up to
// maxFeedbackIterations times until validation passes.
type FeedbackLoop struct {
	planner   *PlannerAgent
	validator *ValidatorAgent
	tools     map[string]Tool
}

// NewFeedbackLoop creates a FeedbackLoop with the given agents and tools.
func NewFeedbackLoop(planner *PlannerAgent, validator *ValidatorAgent, tools map[string]Tool) *FeedbackLoop {
	return &FeedbackLoop{
		planner:   planner,
		validator: validator,
		tools:     tools,
	}
}

// Execute runs the feedback-driven analysis loop.
// Returns an AnalysisResponse with the final plan (or deterministic fallback).
func (fl *FeedbackLoop) Execute(ctx context.Context, req AnalysisRequest, metas []data.DatasetMetadata) (AnalysisResponse, error) {
	primary := req.Datasets[0]

	// Planner produces the initial plan
	plan, notebook, err := fl.planner.Plan(ctx, req.Prompt, metas, req.History)
	if err != nil {
		return AnalysisResponse{}, fmt.Errorf("feedback: initial plan failed: %w", err)
	}

	var lastErr error
	for iteration := 0; iteration < maxFeedbackIterations; iteration++ {
		// Execute tools against the current plan
		toolResults := fl.executeTools(ctx, plan, primary)
		if len(toolResults) > 0 {
			log.Printf("[Feedback] Iteration %d: collected %d tool result(s)", iteration+1, len(toolResults))
		}

		// Validate the plan + results
		result, valErr := fl.validator.Validate(ctx, plan, toolResults, req.Prompt)
		if valErr != nil {
			log.Printf("[Feedback] Validator error on iteration %d: %v", iteration+1, valErr)
			// Validator error is non-fatal — assume valid
			result = &ValidationResult{Valid: true}
		}

		if result.Valid {
			log.Printf("[Feedback] Plan validated after %d iteration(s)", iteration+1)
			return fl.buildResponse(req, plan, notebook, toolResults), nil
		}

		log.Printf("[Feedback] Plan rejected on iteration %d: issues=%v", iteration+1, result.Issues)

		// Refine: give planner another shot with feedback context
		refinedPlan, refinedNotebook, refErr := fl.planner.Plan(ctx, fl.buildRefinePrompt(req.Prompt, result), metas, req.History)
		if refErr != nil {
			lastErr = refErr
			log.Printf("[Feedback] Refine failed on iteration %d: %v", iteration+1, refErr)
			// Use the last plan (even if unvalidated) rather than failing entirely
			break
		}
		plan = refinedPlan
		notebook = append(notebook, refinedNotebook...)
		notebook = append(notebook, NotebookStep{
			Title: fmt.Sprintf("Refinement Iteration %d", iteration+1),
			Body:  fmt.Sprintf("Issues: %v\nSuggestions: %v", result.Issues, result.Suggestions),
		})
	}

	// Exhausted iterations — return best-effort plan
	log.Printf("[Feedback] Max iterations reached, returning best-effort plan (last error: %v)", lastErr)
	return fl.buildResponse(req, plan, notebook, nil), nil
}

// executeTools runs all relevant tools against the plan's column selection.
func (fl *FeedbackLoop) executeTools(ctx context.Context, plan *LLMPlan, ds *data.Dataset) map[string]interface{} {
	results := make(map[string]interface{})

	// Always call get_dataset_profile
	if tool, ok := fl.tools["get_dataset_profile"]; ok {
		if res, err := tool.Execute(ctx, map[string]interface{}{"dataset": ds}); err == nil {
			results["profile"] = res
		} else {
			log.Printf("[Feedback] profile tool failed: %v", err)
		}
	}

	// Call aggregate_metric if we have a metric column
	if plan.MetricColumn != "" {
		if tool, ok := fl.tools["aggregate_metric"]; ok {
			if res, err := tool.Execute(ctx, map[string]interface{}{
				"dataset": ds,
				"column":  plan.MetricColumn,
			}); err == nil {
				results["aggregate"] = res
			} else {
				log.Printf("[Feedback] aggregate tool failed: %v", err)
			}
		}
	}

	// Call group_by_dimension if we have both a category and metric column
	if plan.CategoryColumn != "" && plan.MetricColumn != "" {
		if tool, ok := fl.tools["group_by_dimension"]; ok {
			if res, err := tool.Execute(ctx, map[string]interface{}{
				"dataset":         ds,
				"category_column": plan.CategoryColumn,
				"metric_column":   plan.MetricColumn,
			}); err == nil {
				results["group_by"] = res
			} else {
				log.Printf("[Feedback] group_by tool failed: %v", err)
			}
		}
	}

	// Call build_trend if we have both a date and metric column
	if plan.DateColumn != "" && plan.MetricColumn != "" {
		if tool, ok := fl.tools["build_trend"]; ok {
			if res, err := tool.Execute(ctx, map[string]interface{}{
				"dataset":       ds,
				"date_column":   plan.DateColumn,
				"metric_column": plan.MetricColumn,
			}); err == nil {
				results["trend"] = res
			} else {
				log.Printf("[Feedback] trend tool failed: %v", err)
			}
		}
	}

	return results
}

// buildRefinePrompt creates a refined user prompt that includes validation feedback.
func (fl *FeedbackLoop) buildRefinePrompt(originalPrompt string, val *ValidationResult) string {
	refine := "Original question: " + originalPrompt + "\n\n"
	if len(val.Issues) > 0 {
		refine += "Issues with previous plan:\n"
		for _, issue := range val.Issues {
			refine += "- " + issue + "\n"
		}
	}
	if len(val.Suggestions) > 0 {
		refine += "Suggestions for improvement:\n"
		for _, s := range val.Suggestions {
			refine += "- " + s + "\n"
		}
	}
	refine += "\nPlease create an improved analysis plan that addresses these issues."
	return refine
}

// buildResponse constructs the final AnalysisResponse from the validated plan.
func (fl *FeedbackLoop) buildResponse(req AnalysisRequest, plan *LLMPlan, notebook []NotebookStep, toolResults map[string]interface{}) AnalysisResponse {
	primary := req.Datasets[0]

	resp := AnalysisResponse{
		Question: req.Prompt,
		Dataset: DatasetSummary{
			ID:       primary.ID,
			Filename: primary.Filename,
			RowCount: primary.Profile.RowCount,
		},
		Plan:        plan,
		Notebook:    notebook,
		Assumptions: plan.Assumptions,
		Context: &ConversationContext{
			MetricCol:   plan.MetricColumn,
			CategoryCol: plan.CategoryColumn,
			DateCol:     plan.DateColumn,
			Filters:     plan.Filters,
		},
	}

	// Add tool result summary to notebook if available
	if len(toolResults) > 0 {
		summary := make(map[string]interface{})
		for k, v := range toolResults {
			switch val := v.(type) {
			case map[string]interface{}:
				// Extract key fields for a concise summary
				summary[k] = val
			default:
				summary[k] = v
			}
		}
		summaryJSON, _ := json.Marshal(summary)
		resp.Notebook = append(resp.Notebook, NotebookStep{
			Title: "Tool Results Summary",
			Body:  truncateStr(string(summaryJSON), 500),
		})
	}

	return resp
}
