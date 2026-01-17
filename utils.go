package delta

import (
	"fmt"
	"strings"
)

func filterDeletes(actions []Action) []Action {
	removedFileMap := make(map[string]struct{})
	var filteredActions []Action

	for _, action := range actions {
		if action.DataActionObject.ActionType == "Remove" {
			removedFileMap[action.DataActionObject.Name] = struct{}{}
		}
	}
	// loop and delete the actions of type Add and Remove for a table file
	for _, action := range actions {
		if _, exists := removedFileMap[action.DataActionObject.Name]; !exists {
			filteredActions = append(filteredActions, action)
		}
	}
	return filteredActions
}

func getSimpleExpr(schema []string, predicate string) (simpleExpression, error) {
	// simple expression is of type col > | < | == | != value
	predicateTokens := strings.Split(predicate, " ")
	if len(predicateTokens) < 3 {
		return simpleExpression{}, errIncorrectExprFormat
	}
	colIdx, err := getColIndex(schema, predicateTokens[0])
	if err != nil {
		return simpleExpression{}, err
	}
	expr := simpleExpression{
		left:     colIdx,
		operator: Operator(strings.TrimSpace(predicateTokens[1])),
		right:    expression(strings.TrimSpace(predicateTokens[2])),
	}
	return expr, nil
}

func getColIndex(schema []string, col string) (int, error) {
	for idx := range schema {
		if strings.Compare(schema[idx], col) == 0 {
			return idx, nil
		}
	}
	return 0, nil
}
func getLogName(index int, exts ...string) string {
	return fmt.Sprintf("%020d%s", index, strings.Join(exts, ""))
}
