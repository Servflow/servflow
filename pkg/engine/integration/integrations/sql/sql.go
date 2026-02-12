package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/Servflow/servflow/pkg/engine/integration"
	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var supportedDrivers = []string{"postgres", "mysql"}

type Config struct {
	Type             string `json:"type"`
	ConnectionString string `json:"connectionString"`
}

// TODO change to table
var (
	tableOption           = "table"
	tableOptionCollection = "collection"
)

type SQL struct {
	db *sqlx.DB
}

func (s *SQL) Delete(ctx context.Context, options map[string]string, filters ...filters.Filter) error {
	t := s.getTableName(options)
	if t == "" {
		return fmt.Errorf("no table name provided")
	}
	if err := validateTableName(t); err != nil {
		return err
	}

	whereClause, values, err := generateWhereClause(filters...)
	if err != nil {
		return err
	}
	if whereClause != "" {
		whereClause = fmt.Sprintf("WHERE %s", whereClause)
	}

	query := fmt.Sprintf("DELETE FROM %s %s;", t, whereClause)
	query = s.db.Rebind(query)
	_, err = s.db.Exec(query, values...)
	return err
}

func (s *SQL) Type() string {
	return "sql"
}

func init() {
	fields := map[string]integration.FieldInfo{
		"type": {
			Type:        integration.FieldTypeSelect,
			Label:       "Database Type",
			Placeholder: "Select database type",
			Required:    true,
			Values:      supportedDrivers,
		},
		"connectionString": {
			Type:        integration.FieldTypePassword,
			Label:       "Connection String",
			Placeholder: "postgres://user:pass@localhost:5432/dbname",
			Required:    true,
		},
	}

	if err := integration.RegisterIntegration("sql", integration.IntegrationRegistrationInfo{
		Name:        "SQL Database",
		Description: "SQL database integration supporting PostgreSQL and MySQL",
		Fields:      fields,
		Constructor: func(m map[string]any) (integration.Integration, error) {
			return newWrapper(Config{
				Type:             m["type"].(string),
				ConnectionString: m["connectionString"].(string),
			})
		},
	}); err != nil {
		panic(err)
	}
}

func newWrapper(cfg Config) (*SQL, error) {
	if !isDriverSupported(cfg.Type) {
		return nil, fmt.Errorf("SQL driver not supported: %s", cfg.Type)
	}

	db, err := sqlx.Open(cfg.Type, cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error creating connection: %v", err)
	}

	s := &SQL{
		db: db,
	}
	return s, nil
}

func isDriverSupported(driver string) bool {
	for _, supported := range supportedDrivers {
		if supported == driver {
			return true
		}
	}
	return false
}

// validateTableName ensures the table name is safe to use.
func validateTableName(tableName string) error {
	if strings.ContainsAny(tableName, " ;'\"") {
		return fmt.Errorf("invalid table name")
	}
	return nil
}

func (s *SQL) Fetch(ctx context.Context, options map[string]string, filters ...filters.Filter) (items []map[string]interface{}, err error) {
	t := s.getTableName(options)
	if t == "" {
		return nil, fmt.Errorf("no table name provided")
	}
	if err := validateTableName(t); err != nil {
		return nil, err
	}

	whereClause, values, err := generateWhereClause(filters...)
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		whereClause = fmt.Sprintf("WHERE %s", whereClause)
	}

	q := s.db.Rebind(fmt.Sprintf("SELECT * FROM %s %s;", t, whereClause))
	rows, err := s.db.Queryx(q, values...)
	if err != nil {
		return nil, err
	}

	resp := make([]map[string]interface{}, 0)
	for rows.Next() {
		results := make(map[string]interface{})
		if err = rows.MapScan(results); err != nil {
			return nil, err
		}
		resp = append(resp, results)
	}

	return resp, nil
}

func (s *SQL) getTableName(options map[string]string) string {
	t, ok := options[tableOption]
	if !ok {
		t, ok = options[tableOptionCollection]
		if !ok {
			return ""
		}
	}
	return t
}

func generateWhereClause(filters ...filters.Filter) (string, []interface{}, error) {
	single := make([]string, len(filters))
	values := make([]interface{}, len(filters))
	for i, filter := range filters {
		q, err := filter.ToSQLComp()
		if err != nil {
			return "", nil, err
		}
		single[i] = q
		values[i] = filter.Comparator
	}

	return strings.Join(single, " AND "), values, nil
}

func (s *SQL) Store(ctx context.Context, item map[string]interface{}, options map[string]string) error {
	t := s.getTableName(options)
	if t == "" {
		return fmt.Errorf("no table name provided")
	}
	if err := validateTableName(t); err != nil {
		return err
	}

	keys := make([]string, 0, len(item))
	values := make([]interface{}, 0, len(item))
	placeholders := make([]string, 0, len(item))
	for key, value := range item {
		keys = append(keys, key)
		values = append(values, value)
		placeholders = append(placeholders, "?")
	}
	if len(keys) < 1 {
		return nil
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", t, strings.Join(keys, ","), strings.Join(placeholders, ","))
	query = s.db.Rebind(query)
	_, err := s.db.Exec(query, values...)
	return err
}

func (s *SQL) Update(ctx context.Context, fields map[string]interface{}, options map[string]string, filters ...filters.Filter) error {
	t := s.getTableName(options)
	if t == "" {
		return fmt.Errorf("no table name provided")
	}
	if err := validateTableName(t); err != nil {
		return err
	}

	if len(fields) < 1 {
		return nil
	}

	setStatements := make([]string, 0, len(fields))
	values := make([]interface{}, 0, len(fields))

	for key, value := range fields {
		setStatements = append(setStatements, fmt.Sprintf("%s = ?", key))
		values = append(values, value)
	}

	whereClause, whereValues, err := generateWhereClause(filters...)
	if err != nil {
		return err
	}

	values = append(values, whereValues...)

	var query string
	if whereClause != "" {
		query = fmt.Sprintf("UPDATE %s SET %s WHERE %s", t, strings.Join(setStatements, ", "), whereClause)
	} else {
		query = fmt.Sprintf("UPDATE %s SET %s", t, strings.Join(setStatements, ", "))
	}

	query = s.db.Rebind(query)
	_, err = s.db.Exec(query, values...)
	return err
}
