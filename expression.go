package delta

import (
	"errors"
	"log"
	"reflect"
	"strconv"
	"strings"
)

var errUnknownOperator = errors.New("Unknown Operator")

type expression interface{}

type Operator string

type row []any

// simple expression w/o any logical op
type simpleExpression struct {
	left     int // column idx
	operator Operator
	right    expression // would be a primitive type int,float,string etc
}

// a complex expr can be expressed as below
// simpleExpr1 logicalOperator simpleExpr2 .....

// dc.remove(table, predicate)
// []any -> row.filter()
// from dc access table schema, get colIdx
// for each row:
// create simpleExpr
// invoke eval(simpleExpr), eval is a method on expression

// new
// expr -> compile(get colId) at dc.remove() body {}
// on row iterator -> row.filter(simpleExpression)
// filter -> return eval()

func (r row) filter(expr simpleExpression) bool {
	return eval(string(expr.operator), r[expr.left], expr.right)
}

// todo: change signature to eval(row,expr) can be evolved to complexExpr as well which calls each []simpleExpr in loop
func eval(cond string, leftValue any, rightValue any) bool {
	switch cond {
	case "==":
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			return leftValue.(int) == rightValue.(int)
		case reflect.TypeOf(string("")):
			return strings.Compare(leftValue.(string), rightValue.(string)) == 0
		default:
			return false
		}

	case ">":
		log.Printf("left value:%v and type:%v", leftValue, reflect.TypeOf(leftValue))
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			log.Printf("expr result in eval: %v", leftValue.(int) > rightValue.(int))
			return leftValue.(int) > rightValue.(int)
		case reflect.TypeOf(float64(0.0)):
			rt, err := strconv.ParseFloat(rightValue.(string), 64)
			if err != nil {
				log.Panic(err)
			}
			result := leftValue.(float64) > rt
			log.Printf("expr result in eval: %v", result)
			return result
		case reflect.TypeOf(string("")):
			return strings.Compare(leftValue.(string), rightValue.(string)) == 1
		default:
			return false
		}

	case "<":
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			return leftValue.(int) < rightValue.(int)
		case reflect.TypeOf(string("")):
			return strings.Compare(leftValue.(string), rightValue.(string)) == -1
		default:
			return false
		}
	case "<>":
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			return !(leftValue.(int) == rightValue.(int))
		case reflect.TypeOf(string("")):
			return !(strings.Compare(leftValue.(string), rightValue.(string)) == 0)
		default:
			return false
		}
	default:
		return false

	}
}

// there should be better ways to do this
func getTypeFrmStrVal(stringValue string) interface{} {
	for {
		if val, err := strconv.Atoi(stringValue); err != nil {
			return val
		}
		if val, err := strconv.ParseBool(stringValue); err != nil {
			return val
		}
		if val, err := strconv.ParseFloat(stringValue, 64); err != nil {
			return val
		}
	}
}
