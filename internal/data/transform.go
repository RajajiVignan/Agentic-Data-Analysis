package data

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TransformStep describes a single transformation operation.
type TransformStep struct {
	Type        string                 `json:"type"`
	Params      map[string]interface{} `json:"params"`
	Description string                 `json:"description"`
}

// TransformPipeline holds an ordered sequence of steps with undo/redo stacks.
type TransformPipeline struct {
	Steps  []TransformStep `json:"steps"`
	Undone []TransformStep `json:"undone"`
}

func NewTransformPipeline() *TransformPipeline {
	return &TransformPipeline{
		Steps:  make([]TransformStep, 0),
		Undone: make([]TransformStep, 0),
	}
}

func (p *TransformPipeline) AddStep(s TransformStep) {
	p.Steps = append(p.Steps, s)
	p.Undone = nil
}

func (p *TransformPipeline) Undo() bool {
	if len(p.Steps) == 0 {
		return false
	}
	last := p.Steps[len(p.Steps)-1]
	p.Steps = p.Steps[:len(p.Steps)-1]
	p.Undone = append(p.Undone, last)
	return true
}

func (p *TransformPipeline) Redo() bool {
	if len(p.Undone) == 0 {
		return false
	}
	last := p.Undone[len(p.Undone)-1]
	p.Undone = p.Undone[:len(p.Undone)-1]
	p.Steps = append(p.Steps, last)
	return true
}

func (p *TransformPipeline) CanUndo() bool { return len(p.Steps) > 0 }
func (p *TransformPipeline) CanRedo() bool { return len(p.Undone) > 0 }

func (p *TransformPipeline) ApplyAll(ds *Dataset) *Dataset {
	result := copyDataset(ds)
	for _, step := range p.Steps {
		result = applyStep(result, step)
	}
	return result
}

func ApplySingle(ds *Dataset, step TransformStep) *Dataset {
	result := copyDataset(ds)
	return applyStep(result, step)
}

func copyDataset(ds *Dataset) *Dataset {
	rows := make([]map[string]string, len(ds.Rows))
	for i, row := range ds.Rows {
		nr := make(map[string]string, len(row))
		for k, v := range row {
			nr[k] = v
		}
		rows[i] = nr
	}
	cols := make([]Column, len(ds.Profile.Columns))
	copy(cols, ds.Profile.Columns)
	return &Dataset{
		ID:       ds.ID,
		Filename: ds.Filename,
		FilePath: ds.FilePath,
		Profile: Profile{
			RowCount: ds.Profile.RowCount,
			Columns:  cols,
		},
		Rows: rows,
	}
}

func applyStep(ds *Dataset, step TransformStep) *Dataset {
	switch step.Type {
	case "filter":
		return applyFilter(ds, step.Params)
	case "rename":
		return applyRename(ds, step.Params)
	case "drop":
		return applyDrop(ds, step.Params)
	case "null_handle":
		return applyNullHandle(ds, step.Params)
	case "derive":
		return applyDerive(ds, step.Params)
	case "aggregate":
		return applyAggregate(ds, step.Params)
	case "sort":
		return applySort(ds, step.Params)
	case "select":
		return applySelect(ds, step.Params)
	default:
		return ds
	}
}

// --- Filter ---

func applyFilter(ds *Dataset, params map[string]interface{}) *Dataset {
	col, _ := params["column"].(string)
	op, _ := params["operator"].(string)
	val, _ := params["value"].(string)

	var out []map[string]string
	for _, row := range ds.Rows {
		if matchFilter(row, col, op, val) {
			out = append(out, row)
		}
	}
	result := copyDataset(ds)
	result.Rows = out
	result.Profile = ProfileRows(objectsToRows(result.Rows, columnNames(ds)))
	return result
}

func matchFilter(row map[string]string, col, op, val string) bool {
	v, ok := row[col]
	if !ok {
		return false
	}
	switch op {
	case "eq":
		return strings.EqualFold(v, val)
	case "neq":
		return !strings.EqualFold(v, val)
	case "gt":
		a, _ := strconv.ParseFloat(v, 64)
		b, _ := strconv.ParseFloat(val, 64)
		return a > b
	case "gte":
		a, _ := strconv.ParseFloat(v, 64)
		b, _ := strconv.ParseFloat(val, 64)
		return a >= b
	case "lt":
		a, _ := strconv.ParseFloat(v, 64)
		b, _ := strconv.ParseFloat(val, 64)
		return a < b
	case "lte":
		a, _ := strconv.ParseFloat(v, 64)
		b, _ := strconv.ParseFloat(val, 64)
		return a <= b
	case "contains":
		return strings.Contains(strings.ToLower(v), strings.ToLower(val))
	case "is_empty":
		return strings.TrimSpace(v) == ""
	case "is_not_empty":
		return strings.TrimSpace(v) != ""
	case "starts_with":
		return strings.HasPrefix(strings.ToLower(v), strings.ToLower(val))
	case "ends_with":
		return strings.HasSuffix(strings.ToLower(v), strings.ToLower(val))
	default:
		return true
	}
}

// --- Rename ---

func applyRename(ds *Dataset, params map[string]interface{}) *Dataset {
	raw, _ := params["mappings"].(map[string]interface{})
	mappings := make(map[string]string)
	for k, v := range raw {
		mappings[k] = fmt.Sprintf("%v", v)
	}
	result := copyDataset(ds)
	for i, row := range result.Rows {
		for old, new := range mappings {
			if val, ok := row[old]; ok {
				delete(row, old)
				row[new] = val
			}
		}
		result.Rows[i] = row
	}
	result.Profile = ProfileRows(objectsToRows(result.Rows, columnNames(ds)))
	return result
}

// --- Drop Columns ---

func applyDrop(ds *Dataset, params map[string]interface{}) *Dataset {
	rawCols, _ := params["columns"].([]interface{})
	drop := make(map[string]bool)
	for _, c := range rawCols {
		drop[fmt.Sprintf("%v", c)] = true
	}
	// Also support single column string
	if colStr, ok := params["column"].(string); ok && colStr != "" {
		drop[colStr] = true
	}
	result := copyDataset(ds)
	for i, row := range result.Rows {
		for col := range drop {
			delete(row, col)
		}
		result.Rows[i] = row
	}
	result.Profile = ProfileRows(objectsToRows(result.Rows, columnNames(ds)))
	return result
}

// --- Null Handle ---

func applyNullHandle(ds *Dataset, params map[string]interface{}) *Dataset {
	strategy, _ := params["strategy"].(string)
	col, _ := params["column"].(string)
	fillVal, _ := params["fillValue"].(string)

	result := copyDataset(ds)

	switch strategy {
	case "drop_row":
		var out []map[string]string
		for _, row := range result.Rows {
			if col != "" {
				if row[col] != "" {
					out = append(out, row)
				}
			} else {
				hasEmpty := false
				for _, v := range row {
					if strings.TrimSpace(v) == "" {
						hasEmpty = true
						break
					}
				}
				if !hasEmpty {
					out = append(out, row)
				}
			}
		}
		result.Rows = out
	case "fill":
		for _, row := range result.Rows {
			if col != "" {
				if strings.TrimSpace(row[col]) == "" {
					row[col] = fillVal
				}
			} else {
				for k := range row {
					if strings.TrimSpace(row[k]) == "" {
						row[k] = fillVal
					}
				}
			}
		}
	case "fill_forward":
		prev := ""
		for _, row := range result.Rows {
			if col != "" {
				if strings.TrimSpace(row[col]) == "" {
					row[col] = prev
				} else {
					prev = row[col]
				}
			}
		}
	case "fill_backward":
		next := ""
		for i := len(result.Rows) - 1; i >= 0; i-- {
			if col != "" {
				if strings.TrimSpace(result.Rows[i][col]) == "" {
					result.Rows[i][col] = next
				} else {
					next = result.Rows[i][col]
				}
			}
		}
	case "fill_mean":
		if col == "" {
			break
		}
		var vals []float64
		for _, row := range result.Rows {
			if v, err := strconv.ParseFloat(strings.TrimSpace(row[col]), 64); err == nil {
				vals = append(vals, v)
			}
		}
		if len(vals) > 0 {
			sum := 0.0
			for _, v := range vals {
				sum += v
			}
			mean := sum / float64(len(vals))
			meanStr := strconv.FormatFloat(mean, 'f', 2, 64)
			for _, row := range result.Rows {
				if strings.TrimSpace(row[col]) == "" {
					row[col] = meanStr
				}
			}
		}
	}

	result.Profile = ProfileRows(objectsToRows(result.Rows, columnNames(ds)))
	return result
}

// --- Derive Column ---

func applyDerive(ds *Dataset, params map[string]interface{}) *Dataset {
	newCol, _ := params["newColumn"].(string)
	expr, _ := params["expression"].(string)

	if newCol == "" || expr == "" {
		return ds
	}

	result := copyDataset(ds)
	for _, row := range result.Rows {
		val := evalSimpleExpr(row, expr)
		row[newCol] = val
	}
	result.Profile = ProfileRows(objectsToRows(result.Rows, columnNames(ds)))
	return result
}

var exprRe = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

func evalSimpleExpr(row map[string]string, expr string) string {
	replaced := exprRe.ReplaceAllStringFunc(expr, func(match string) string {
		if v, ok := row[match]; ok && v != "" {
			return v
		}
		switch strings.ToLower(match) {
		case "pi":
			return fmt.Sprintf("%.10f", math.Pi)
		}
		return "0"
	})

	val, err := evalMath(replaced)
	if err != nil {
		return ""
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func evalMath(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, nil
	}
	tokens := tokenize(expr)
	if len(tokens) == 0 {
		return 0, fmt.Errorf("empty expression")
	}
	result, _, err := parseAddSub(tokens, 0)
	return result, err
}

func tokenize(s string) []string {
	var tokens []string
	cur := ""
	expectOperand := true
	for _, ch := range s {
		if ch == ' ' {
			continue
		}
		isOp := ch == '+' || ch == '-' || ch == '*' || ch == '/'
		if ch == '(' || ch == ')' {
			if cur != "" {
				tokens = append(tokens, cur)
				cur = ""
			}
			tokens = append(tokens, string(ch))
			expectOperand = ch == '('
		} else if isOp {
			if ch == '-' && expectOperand {
				cur += string(ch)
			} else {
				if cur != "" {
					tokens = append(tokens, cur)
					cur = ""
				}
				tokens = append(tokens, string(ch))
				expectOperand = true
			}
		} else {
			cur += string(ch)
			expectOperand = false
		}
	}
	if cur != "" {
		tokens = append(tokens, cur)
	}
	return tokens
}

func isOperator(s string) bool {
	return s == "+" || s == "-" || s == "*" || s == "/"
}

func parseAddSub(tokens []string, pos int) (float64, int, error) {
	left, pos, err := parseMulDiv(tokens, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(tokens) {
		op := tokens[pos]
		if op != "+" && op != "-" {
			break
		}
		pos++
		right, newPos, err := parseMulDiv(tokens, pos)
		if err != nil {
			return 0, newPos, err
		}
		pos = newPos
		if op == "+" {
			left += right
		} else {
			left -= right
		}
	}
	return left, pos, nil
}

func parseMulDiv(tokens []string, pos int) (float64, int, error) {
	left, pos, err := parsePrimary(tokens, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(tokens) {
		op := tokens[pos]
		if op != "*" && op != "/" {
			break
		}
		pos++
		right, newPos, err := parsePrimary(tokens, pos)
		if err != nil {
			return 0, newPos, err
		}
		pos = newPos
		if op == "*" {
			left *= right
		} else {
			if right == 0 {
				return 0, pos, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, pos, nil
}

func parsePrimary(tokens []string, pos int) (float64, int, error) {
	if pos >= len(tokens) {
		return 0, pos, fmt.Errorf("unexpected end")
	}
	if tokens[pos] == "(" {
		pos++
		val, newPos, err := parseAddSub(tokens, pos)
		if err != nil {
			return 0, newPos, err
		}
		if newPos >= len(tokens) || tokens[newPos] != ")" {
			return 0, newPos, fmt.Errorf("missing closing paren")
		}
		return val, newPos + 1, nil
	}
	// Handle unary minus: "-" followed by a number
	if tokens[pos] == "-" && pos+1 < len(tokens) && !isOperator(tokens[pos+1]) {
		v, err := strconv.ParseFloat(tokens[pos+1], 64)
		if err != nil {
			return 0, pos + 2, fmt.Errorf("cannot parse negative of %q", tokens[pos+1])
		}
		return -v, pos + 2, nil
	}
	v, err := strconv.ParseFloat(tokens[pos], 64)
	if err != nil {
		return 0, pos + 1, fmt.Errorf("cannot parse %q", tokens[pos])
	}
	return v, pos + 1, nil
}

// --- Aggregate ---

func applyAggregate(ds *Dataset, params map[string]interface{}) *Dataset {
	groupBy, _ := params["groupBy"].(string)
	aggCol, _ := params["aggregateColumn"].(string)
	fn, _ := params["function"].(string)
	newColName, _ := params["newColumnName"].(string)
	if groupBy == "" || aggCol == "" {
		return ds
	}
	if newColName == "" {
		newColName = fmt.Sprintf("%s_%s", fn, aggCol)
	}

	type entry struct {
		vals []float64
	}
	groups := make(map[string]*entry)
	order := make([]string, 0)

	for _, row := range ds.Rows {
		key := row[groupBy]
		if v, err := strconv.ParseFloat(strings.TrimSpace(row[aggCol]), 64); err == nil {
			if _, ok := groups[key]; !ok {
				groups[key] = &entry{}
				order = append(order, key)
			}
			groups[key].vals = append(groups[key].vals, v)
		}
	}

	var out []map[string]string
	for _, key := range order {
		e := groups[key]
		if len(e.vals) == 0 {
			continue
		}
		var computed float64
		switch fn {
		case "sum":
			for _, v := range e.vals {
				computed += v
			}
		case "avg", "mean":
			sum := 0.0
			for _, v := range e.vals {
				sum += v
			}
			computed = sum / float64(len(e.vals))
		case "count":
			computed = float64(len(e.vals))
		case "min":
			computed = e.vals[0]
			for _, v := range e.vals[1:] {
				if v < computed {
					computed = v
				}
			}
		case "max":
			computed = e.vals[0]
			for _, v := range e.vals[1:] {
				if v > computed {
					computed = v
				}
			}
		default:
			for _, v := range e.vals {
				computed += v
			}
		}
		row := map[string]string{
			groupBy:    key,
			newColName: formatNum(computed),
		}
		out = append(out, row)
	}

	result := copyDataset(ds)
	result.Rows = out
	result.Profile = ProfileRows(objectsToRows(out, []string{groupBy, newColName}))
	return result
}

// --- Sort ---

func applySort(ds *Dataset, params map[string]interface{}) *Dataset {
	col, _ := params["column"].(string)
	order, _ := params["order"].(string)
	desc := order == "desc"

	cols := columnNames(ds)
	result := copyDataset(ds)
	colIdx := -1
	for i, c := range cols {
		if c == col {
			colIdx = i
			break
		}
	}
	if colIdx < 0 {
		return result
	}

	sort.SliceStable(result.Rows, func(i, j int) bool {
		a := result.Rows[i][col]
		b := result.Rows[j][col]
		if desc {
			return a > b
		}
		return a < b
	})
	return result
}

// --- Select Columns ---

func applySelect(ds *Dataset, params map[string]interface{}) *Dataset {
	rawCols, _ := params["columns"].([]interface{})
	keep := make(map[string]bool)
	for _, c := range rawCols {
		keep[fmt.Sprintf("%v", c)] = true
	}

	result := copyDataset(ds)
	for i, row := range result.Rows {
		for k := range row {
			if !keep[k] {
				delete(row, k)
			}
		}
		result.Rows[i] = row
	}
	result.Profile = ProfileRows(objectsToRows(result.Rows, columnNames(ds)))
	return result
}

// --- Helpers ---

func columnNames(ds *Dataset) []string {
	names := make([]string, len(ds.Profile.Columns))
	for i, c := range ds.Profile.Columns {
		names[i] = c.Name
	}
	return names
}

func objectsToRows(objs []map[string]string, allCols []string) [][]string {
	if len(objs) == 0 {
		if len(allCols) == 0 {
			return [][]string{}
		}
		return [][]string{allCols}
	}

	// Collect all column names present in the data
	colSet := make(map[string]bool)
	for _, obj := range objs {
		for k := range obj {
			colSet[k] = true
		}
	}
	ordered := make([]string, 0, len(colSet))
	// Start with known columns in order
	added := make(map[string]bool)
	for _, c := range allCols {
		if colSet[c] {
			ordered = append(ordered, c)
			added[c] = true
		}
	}
	// Add any new columns
	for k := range colSet {
		if !added[k] {
			ordered = append(ordered, k)
		}
	}

	rows := make([][]string, len(objs)+1)
	rows[0] = ordered
	for i, obj := range objs {
		row := make([]string, len(ordered))
		for j, col := range ordered {
			row[j] = obj[col]
		}
		rows[i+1] = row
	}
	return rows
}

func formatNum(v float64) string {
	if v == math.Trunc(v) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
