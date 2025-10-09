package filters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestFilterToBsonE(t *testing.T) {

	tests := []struct {
		name     string
		filter   Filter
		expected bson.E
		wantErr  bool
	}{
		{
			name:     "Equals operator",
			filter:   Filter{Field: "name", Operation: Equals, Comparator: "test"},
			expected: bson.E{Key: "name", Value: "test"},
			wantErr:  false,
		},
		{
			name:     "not Equals operator",
			filter:   Filter{Field: "age", Operation: NotEquals, Comparator: 25},
			expected: bson.E{Key: "age", Value: bson.D{{"$ne", 25}}},
			wantErr:  false,
		},
		{
			name:     "greater than operator",
			filter:   Filter{Field: "count", Operation: GreaterThan, Comparator: 100},
			expected: bson.E{Key: "count", Value: bson.D{{"$gt", 100}}},
			wantErr:  false,
		},
		{
			name:    "invalid operator",
			filter:  Filter{Field: "test", Operation: "invalid", Comparator: "test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.filter.ToBsonE()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterToSQLComp(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		expected string
		wantErr  bool
	}{
		{
			name:     "Equals operator",
			filter:   Filter{Field: "name", Operation: Equals, Comparator: "test"},
			expected: "name = ?",
			wantErr:  false,
		},
		{
			name:     "greater than operator",
			filter:   Filter{Field: "age", Operation: GreaterThan, Comparator: 25},
			expected: "age > ?",
			wantErr:  false,
		},
		{
			name:    "invalid operator",
			filter:  Filter{Field: "test", Operation: "invalid", Comparator: "test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.filter.ToSQLComp()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFiltersToBSON(t *testing.T) {
	tests := []struct {
		name     string
		filters  []Filter
		expected bson.D
		wantErr  bool
	}{
		{
			name:     "empty filters",
			filters:  []Filter{},
			expected: bson.D{},
			wantErr:  false,
		},
		{
			name: "multiple valid filters",
			filters: []Filter{
				{Field: "name", Operation: Equals, Comparator: "test"},
				{Field: "age", Operation: GreaterThan, Comparator: 25},
			},
			expected: bson.D{
				{Key: "name", Value: "test"},
				{Key: "age", Value: bson.D{{"$gt", 25}}},
			},
			wantErr: false,
		},
		{
			name: "invalid filter",
			filters: []Filter{
				{Field: "test", Operation: "invalid", Comparator: "test"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FiltersToBSON(tt.filters)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
