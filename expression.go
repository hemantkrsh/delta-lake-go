package delta

import (
	"errors"
	"log"
	"reflect"
	"strconv"
	"strings"
)

var errUnknownOperator = errors.New("unknown operator")

type expression interface{}

type Operator string

type row []any

// simple expression w/o any logical op
type simpleExpression struct {
	left     int // column idx
	operator Operator
	right    expression // would be a primitive type int,float,string etc
}

func (r row) filter(expr simpleExpression) (bool, error) {
	return eval(string(expr.operator), r[expr.left], expr.right)
}

// todo: change signature to eval(row,expr) can be evolved to complexExpr as well which calls each []simpleExpr in loop
func eval(cond string, leftValue any, rightValue any) (bool, error) {
	switch cond {
	case "==":
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			return leftValue.(int) == rightValue.(int), nil
		case reflect.TypeOf(string("")):
			return strings.Compare(leftValue.(string), rightValue.(string)) == 0, nil
		default:
			return false, nil
		}

	case ">":
		log.Printf("left value:%v and type:%v", leftValue, reflect.TypeOf(leftValue))
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			log.Printf("expr result in eval: %v", leftValue.(int) > rightValue.(int))
			return leftValue.(int) > rightValue.(int), nil
		case reflect.TypeOf(float64(0.0)):
			rt, err := strconv.ParseFloat(rightValue.(string), 64)
			if err != nil {
				log.Panic(err)
			}
			result := leftValue.(float64) > rt
			log.Printf("expr result in eval: %v", result)
			return result, nil
		case reflect.TypeOf(string("")):
			return strings.Compare(leftValue.(string), rightValue.(string)) == 1, nil
		default:
			return false, nil
		}

	case "<":
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			return leftValue.(int) < rightValue.(int), nil
		case reflect.TypeOf(string("")):
			return strings.Compare(leftValue.(string), rightValue.(string)) == -1, nil
		default:
			return false, nil
		}
	case "<>":
		switch reflect.TypeOf(leftValue) {
		case reflect.TypeOf(int(0)):
			return !(leftValue.(int) == rightValue.(int)), nil
		case reflect.TypeOf(string("")):
			return !(strings.Compare(leftValue.(string), rightValue.(string)) == 0), nil
		default:
			return false, nil
		}
	default:
		return false, errUnknownOperator

	}
}

// TODO: there should be better ways to do this
// func getTypeFrmStrVal(stringValue string) any {
// 	for {
// 		if val, err := strconv.Atoi(stringValue); err != nil {
// 			return val
// 		}
// 		if val, err := strconv.ParseBool(stringValue); err != nil {
// 			return val
// 		}
// 		if val, err := strconv.ParseFloat(stringValue, 64); err != nil {
// 			return val
// 		}
// 	}
// }
