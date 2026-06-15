package agent

import "time"

type ConversationSession struct {
	ID           string
	DatasetIDs   []string
	History      []ConversationTurn
	ActiveContext *ConversationContext
	CreatedAt    time.Time
	LastActiveAt time.Time
}

type ConversationTurn struct {
	Prompt    string         `json:"prompt"`
	Response  AnalysisResponse `json:"response"`
	Timestamp time.Time      `json:"timestamp"`
}

type ConversationContext struct {
	Filters     []FilterClause `json:"filters"`
	GroupBy     string         `json:"groupBy,omitempty"`
	MetricCol   string         `json:"metricCol,omitempty"`
	CategoryCol string         `json:"categoryCol,omitempty"`
	DateCol     string         `json:"dateCol,omitempty"`
}

type FilterClause struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type FilterOperation string

const (
	FilterEQ       FilterOperation = "eq"
	FilterNEQ      FilterOperation = "neq"
	FilterGT       FilterOperation = "gt"
	FilterGTE      FilterOperation = "gte"
	FilterLT       FilterOperation = "lt"
	FilterLTE      FilterOperation = "lte"
	FilterContains FilterOperation = "contains"
	FilterIn       FilterOperation = "in"
)

func NewSession(id string, datasetIDs []string) *ConversationSession {
	now := time.Now()
	return &ConversationSession{
		ID:            id,
		DatasetIDs:    datasetIDs,
		History:       make([]ConversationTurn, 0),
		ActiveContext: &ConversationContext{},
		CreatedAt:     now,
		LastActiveAt:  now,
	}
}

func (s *ConversationSession) AddTurn(prompt string, resp AnalysisResponse) {
	s.History = append(s.History, ConversationTurn{
		Prompt:    prompt,
		Response:  resp,
		Timestamp: time.Now(),
	})
	s.LastActiveAt = time.Now()
}

func (s *ConversationSession) UpdateContext(ctx *ConversationContext) {
	s.ActiveContext = ctx
	s.LastActiveAt = time.Now()
}

func (ctx *ConversationContext) Clone() *ConversationContext {
	if ctx == nil {
		return &ConversationContext{}
	}
	filters := make([]FilterClause, len(ctx.Filters))
	copy(filters, ctx.Filters)
	return &ConversationContext{
		Filters:     filters,
		GroupBy:     ctx.GroupBy,
		MetricCol:   ctx.MetricCol,
		CategoryCol: ctx.CategoryCol,
		DateCol:     ctx.DateCol,
	}
}
