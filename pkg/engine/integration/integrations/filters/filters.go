package filters

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

type Filter struct {
	Field      string
	Operation  string
	Comparator interface{}
}

const (
	Equals             = "=="
	NotEquals          = "!="
	GreaterThan        = ">"
	LessThan           = "<"
	LessThanEqual      = "<="
	GreaterThanOrEqual = ">="
	Like               = "like"
)

func (f *Filter) ToBsonE() (bson.E, error) {
	switch f.Operation {
	case Equals:
		return bson.E{Key: f.Field, Value: f.Comparator}, nil
	case NotEquals:
		return bson.E{Key: f.Field, Value: bson.D{{"$ne", f.Comparator}}}, nil
	case GreaterThan:
		return bson.E{Key: f.Field, Value: bson.D{{"$gt", f.Comparator}}}, nil
	case LessThan:
		return bson.E{Key: f.Field, Value: bson.D{{"$lt", f.Comparator}}}, nil
	case GreaterThanOrEqual:
		return bson.E{Key: f.Field, Value: bson.D{{"$gte", f.Comparator}}}, nil
	case LessThanEqual:
		return bson.E{Key: f.Field, Value: bson.D{{"$lte", f.Comparator}}}, nil
	default:
		return bson.E{}, fmt.Errorf("invalid operation: %s", f.Operation)
	}
}

func (f *Filter) ToSQLComp() (string, error) {
	var op = f.Operation
	switch f.Operation {
	case Equals:
		op = "="
	case NotEquals, GreaterThan, LessThan, GreaterThanOrEqual, LessThanEqual, Like:
		op = f.Operation
	default:
		return "", fmt.Errorf("invalid operation: %s", f.Operation)
	}

	return fmt.Sprintf("%s %s ?", f.Field, op), nil
}

// FiltersToBSON converts an array of Filter structs to a BSON document.
func FiltersToBSON(filters []Filter) (bson.D, error) {
	if len(filters) == 0 {
		return bson.D{}, nil
	}
	var conditions bson.D

	for _, filter := range filters {
		bsonFilter, err := filter.ToBsonE()
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, bsonFilter)
	}

	return conditions, nil
}
