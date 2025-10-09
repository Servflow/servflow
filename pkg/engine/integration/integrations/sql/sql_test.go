package sql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	// Package level variables for container reuse
	postgresContainer testcontainers.Container
	baseConnString    string
	containerInit     sync.Once
)

// TestMain sets up the shared Postgres container once before all tests
func TestMain(m *testing.M) {
	// TestMain will be called by the test runner
	ctx := context.Background()

	// Set up the container once
	containerInit.Do(func() {
		req := testcontainers.ContainerRequest{
			Image:        "postgres:14",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpassword",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections"),
		}

		var err error
		postgresContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			panic(fmt.Sprintf("Failed to start container: %s", err))
		}

		host, err := postgresContainer.Host(ctx)
		if err != nil {
			panic(fmt.Sprintf("Failed to get container host: %s", err))
		}

		port, err := postgresContainer.MappedPort(ctx, "5432")
		if err != nil {
			panic(fmt.Sprintf("Failed to get container port: %s", err))
		}

		baseConnString = fmt.Sprintf("postgres://testuser:testpassword@%s:%s/testdb?sslmode=disable", host, port.Port())

		// Connect to base DB to ensure it's ready
		db, err := sql.Open("postgres", baseConnString)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to database: %s", err))
		}
		defer db.Close()

		for i := 0; i < 10; i++ {
			err = db.Ping()
			if err == nil {
				break
			}
			time.Sleep(time.Second)
		}
		if err != nil {
			panic(fmt.Sprintf("Failed to ping database after retries: %s", err))
		}
	})

	// Run the tests
	code := m.Run()

	// Clean up the container after all tests have run
	if postgresContainer != nil {
		postgresContainer.Terminate(ctx)
	}

	// Exit with the test result code
	os.Exit(code)
}

// newDB creates a new database for each test within the shared container
func newDB(t *testing.T) string {
	// Ensure container is initialized
	containerInit.Do(func() {
		t.Fatal("Container not initialized. TestMain should have been called before tests.")
	})

	// Create a unique database name for this test
	dbName := fmt.Sprintf("testdb_%d", time.Now().UnixNano())

	// Connect to the base database
	db, err := sql.Open("postgres", baseConnString)
	if err != nil {
		t.Fatalf("Failed to connect to database: %s", err)
	}
	defer db.Close()

	// Create a new database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		t.Fatalf("Failed to create test database: %s", err)
	}

	// Register cleanup to drop the database after the test
	t.Cleanup(func() {
		// Create a new connection to PostgreSQL to clean up
		cleanupDB, err := sql.Open("postgres", baseConnString)
		if err != nil {
			t.Logf("Warning: Failed to open cleanup connection: %s", err)
			return
		}
		defer cleanupDB.Close()

		// Need to disconnect all active connections before dropping the database
		connKiller := fmt.Sprintf(`
			SELECT pg_terminate_backend(pg_stat_activity.pid)
			FROM pg_stat_activity
			WHERE pg_stat_activity.datname = '%s'
			AND pid <> pg_backend_pid()
		`, dbName)

		_, err = cleanupDB.Exec(connKiller)
		if err != nil {
			t.Logf("Warning: Failed to terminate connections: %s", err)
		}

		_, err = cleanupDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
		if err != nil {
			t.Logf("Warning: Failed to drop test database: %s", err)
		}
	})

	// Modify the connection string to use the new database
	parts := strings.Split(baseConnString, "/")
	parts[len(parts)-1] = strings.Replace(parts[len(parts)-1], "testdb", dbName, 1)
	connString := strings.Join(parts, "/")

	t.Logf("Created new test database: %s", dbName)
	return connString
}

func setupTestDB(t *testing.T, s *SQL, tableName string) {
	schemaRaw := `
	CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		email VARCHAR(255) NOT NULL,
		password VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);`
	schema := fmt.Sprintf(schemaRaw, tableName)

	_, err := s.db.Exec(schema)
	require.NoError(t, err, "Error creating test table")

	t.Cleanup(func() {
		_, err = s.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
		assert.NoError(t, err)
	})
}

func Test_generateWhereClause(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		filters        []filters.Filter
		expected       string
		expectedValues []interface{}
		wantErr        bool
	}{
		{
			name: "single filter",
			filters: []filters.Filter{
				{
					Operation:  filters.Equals,
					Field:      "name",
					Comparator: "test",
				},
			},
			expected:       "name = ?",
			expectedValues: []interface{}{"test"},
		},
		{
			name:           "empty filters",
			filters:        []filters.Filter{},
			expected:       "",
			expectedValues: []interface{}{},
		},
		{
			name: "multiple filters",
			filters: []filters.Filter{
				{
					Operation:  filters.Equals,
					Field:      "name",
					Comparator: "test",
				},
				{
					Operation:  filters.GreaterThan,
					Field:      "age",
					Comparator: 18,
				},
			},
			expected:       "name = ? AND age > ?",
			expectedValues: []interface{}{"test", 18},
		},
		{
			name: "different operators",
			filters: []filters.Filter{
				{
					Operation:  filters.NotEquals,
					Field:      "status",
					Comparator: "inactive",
				},
				{
					Operation:  filters.LessThanEqual,
					Field:      "price",
					Comparator: 100,
				},
			},
			expected:       "status != ? AND price <= ?",
			expectedValues: []interface{}{"inactive", 100},
		},
		{
			name: "invalid operator",
			filters: []filters.Filter{
				{
					Operation:  "invalid",
					Field:      "name",
					Comparator: "test",
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotten, values, err := generateWhereClause(tc.filters...)
			if (err != nil) != tc.wantErr {
				t.Errorf("generateWhereClause() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				assert.Equal(t, tc.expected, gotten)
				assert.Equal(t, tc.expectedValues, values)
			}
		})
	}
}

func TestSQL_NewWrapper(t *testing.T) {
	// t.Parallel() - removed to ensure container is initialized

	testCases := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid postgres config",
			config: Config{
				Type:             "postgres",
				ConnectionString: newDB(t),
			},
			wantErr: false,
		},
		{
			name: "invalid driver",
			config: Config{
				Type:             "invalid_driver",
				ConnectionString: "invalid_connection_string",
			},
			wantErr: true,
		},
		{
			name: "invalid connection string",
			config: Config{
				Type:             "postgres",
				ConnectionString: "invalid_connection_string",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, err := newWrapper(tc.config)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, s)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, s)
				assert.NotNil(t, s.db)
				s.db.Close()
			}
		})
	}
}

func TestSQL_Fetch(t *testing.T) {
	// t.Parallel() - removed to ensure proper container handling
	sqlConnectionString := newDB(t)

	cfg := Config{
		Type:             "postgres",
		ConnectionString: sqlConnectionString,
	}
	s, err := newWrapper(cfg)
	require.NoError(t, err)

	type testUser struct {
		name     string
		email    string
		password string
	}

	testCases := []struct {
		name          string
		initialUsers  []testUser
		filters       []filters.Filter
		options       map[string]string
		expectedCount int
		tableName     string
		wantErr       bool
		setupFn       func(*testing.T, *SQL)
		checkFn       func(*testing.T, []map[string]interface{})
	}{
		{
			name: "fetch all users",
			initialUsers: []testUser{
				{"Test User 1", "test1@test.com", "password123"},
				{"Test User 2", "test2@test.com", "password456"},
			},
			filters:   []filters.Filter{},
			tableName: "users_first",
			options: map[string]string{
				"table": "users_first",
			},
			expectedCount: 2,
			wantErr:       false,
			checkFn: func(t *testing.T, items []map[string]interface{}) {
				assert.Equal(t, "Test User 1", items[0]["name"])
				assert.Equal(t, "test1@test.com", items[0]["email"])
				assert.Equal(t, "Test User 2", items[1]["name"])
				assert.Equal(t, "test2@test.com", items[1]["email"])
			},
		},
		{
			name: "fetch with filter",
			initialUsers: []testUser{
				{"Test User 1", "test1@test.com", "password123"},
				{"Test User 2", "test2@test.com", "password456"},
				{"Test User 3", "test3@test.com", "password789"},
			},
			filters: []filters.Filter{
				{
					Field:      "name",
					Operation:  "==",
					Comparator: "Test User 2",
				},
			},
			tableName: "users_second",
			options: map[string]string{
				"table": "users_second",
			},
			expectedCount: 1,
			wantErr:       false,
			checkFn: func(t *testing.T, items []map[string]interface{}) {
				assert.Equal(t, "Test User 2", items[0]["name"])
				assert.Equal(t, "test2@test.com", items[0]["email"])
			},
		},
		{
			name:         "fetch empty table",
			initialUsers: []testUser{},
			filters:      []filters.Filter{},
			tableName:    "users_empty",
			options: map[string]string{
				"table": "users_empty",
			},
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name:         "fetch with invalid table",
			initialUsers: []testUser{},
			filters:      []filters.Filter{},
			tableName:    "users_invalid",
			options: map[string]string{
				"table": "users; DROP TABLE users;--",
			},
			expectedCount: 0,
			wantErr:       true,
		},
		{
			name:          "fetch without table option",
			initialUsers:  []testUser{},
			filters:       []filters.Filter{},
			options:       map[string]string{},
			expectedCount: 0,
			wantErr:       true,
		},
		{
			name: "fetch with complex filter",
			initialUsers: []testUser{
				{"Young User", "young@test.com", "password123"},
				{"Old User", "old@test.com", "password456"},
			},
			setupFn: func(t *testing.T, s *SQL) {
				// Add age column for complex filtering
				_, err := s.db.Exec("ALTER TABLE users_complex ADD COLUMN IF NOT EXISTS age INTEGER")
				require.NoError(t, err)

				// Set ages
				_, err = s.db.Exec("UPDATE users_complex SET age = 25 WHERE name = 'Young User'")
				require.NoError(t, err)
				_, err = s.db.Exec("UPDATE users_complex SET age = 65 WHERE name = 'Old User'")
				require.NoError(t, err)
			},
			filters: []filters.Filter{
				{
					Field:      "age",
					Operation:  ">",
					Comparator: 30,
				},
			},
			tableName: "users_complex",
			options: map[string]string{
				"table": "users_complex",
			},
			expectedCount: 1,
			wantErr:       false,
			checkFn: func(t *testing.T, items []map[string]interface{}) {
				assert.Equal(t, "Old User", items[0]["name"])
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.tableName != "" {
				setupTestDB(t, s, tc.tableName)
			} else {
				setupTestDB(t, s, "default_users")
			}

			// Insert initial users
			for _, user := range tc.initialUsers {
				insertQuery := `
									INSERT INTO %s (name, email, password)
									VALUES ($1, $2, $3)
								`
				insertQuery = fmt.Sprintf(insertQuery, tc.tableName)
				_, err := s.db.Exec(insertQuery, user.name, user.email, user.password)
				require.NoError(t, err)
			}

			// Run any additional setup
			if tc.setupFn != nil {
				tc.setupFn(t, s)
			}

			// Execute the fetch operation
			items, err := s.Fetch(context.Background(), tc.options, tc.filters...)

			// Check results
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, items, tc.expectedCount)

				// Run any additional checks
				if tc.checkFn != nil && len(items) > 0 {
					tc.checkFn(t, items)
				}
			}
		})
	}
}

func TestSQL_Store(t *testing.T) {
	// t.Parallel() - removed to ensure proper container handling

	sqlConnectionString := newDB(t)

	cfg := Config{
		Type:             "postgres",
		ConnectionString: sqlConnectionString,
	}
	s, err := newWrapper(cfg)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		item      map[string]interface{}
		tableName string
		options   map[string]string
		wantErr   bool
		checkFn   func(*testing.T, *SQL, map[string]interface{})
	}{
		{
			name: "store single record",
			item: map[string]interface{}{
				"name":     "Test User",
				"email":    "test@test.com",
				"password": "password123",
			},
			tableName: "users_single",
			options: map[string]string{
				"table": "users_single",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL, item map[string]interface{}) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_single WHERE name = $1 AND email = $2",
					item["name"], item["email"]).Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			tableName: "users_empty",
			name:      "store empty fields",
			item:      map[string]interface{}{},
			options: map[string]string{
				"table": "users_empty",
			},
			wantErr: false,
		},
		{
			tableName: "users_invalid",
			name:      "store with invalid table",
			item: map[string]interface{}{
				"name":     "Test User",
				"email":    "test@test.com",
				"password": "password123",
			},
			options: map[string]string{
				"table": "users; DROP TABLE users;--",
			},
			wantErr: true,
			checkFn: func(t *testing.T, s *SQL, item map[string]interface{}) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_invalid").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			tableName: "users_no",
			name:      "store without table option",
			item: map[string]interface{}{
				"name":     "Test User",
				"email":    "test@test.com",
				"password": "password123",
			},
			options: map[string]string{},
			wantErr: true,
		},
		{
			tableName: "users_collection",
			name:      "store with collection option instead of table",
			item: map[string]interface{}{
				"name":     "Collection User",
				"email":    "collection@test.com",
				"password": "password123",
			},
			options: map[string]string{
				"collection": "users_collection",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL, item map[string]interface{}) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_collection WHERE name = $1",
					item["name"]).Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.tableName != "" {
				setupTestDB(t, s, tc.tableName)
			} else {
				setupTestDB(t, s, "default_users")
			}

			// Execute the store operation
			err := s.Store(context.Background(), tc.item, tc.options)

			// Check results
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Run any additional checks
			if tc.checkFn != nil {
				tc.checkFn(t, s, tc.item)
			}
		})
	}
}

func TestSQL_Update(t *testing.T) {
	// t.Parallel() - removed to ensure proper container handling

	sqlConnectionString := newDB(t)

	cfg := Config{
		Type:             "postgres",
		ConnectionString: sqlConnectionString,
	}
	s, err := newWrapper(cfg)
	require.NoError(t, err)

	type updateTestCase struct {
		name         string
		initialSetup func(*testing.T, *SQL)
		fields       map[string]interface{}
		options      map[string]string
		filters      []filters.Filter
		wantErr      bool
		tableName    string
		checkFn      func(*testing.T, *SQL)
	}

	testCases := []updateTestCase{
		{
			name:      "update single record",
			tableName: "users",
			initialSetup: func(t *testing.T, s *SQL) {
				_, err := s.db.Exec(`
					INSERT INTO users (name, email, password)
					VALUES ($1, $2, $3)
				`, "Test User", "test@test.com", "password123")
				require.NoError(t, err)
			},
			fields: map[string]interface{}{
				"name":     "Updated User",
				"password": "newpassword",
			},
			options: map[string]string{
				"table": "users",
			},
			filters: []filters.Filter{
				{
					Operation:  filters.Equals,
					Field:      "email",
					Comparator: "test@test.com",
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				var name, password string
				err := s.db.QueryRow("SELECT name, password FROM users WHERE email = $1",
					"test@test.com").Scan(&name, &password)
				assert.NoError(t, err)
				assert.Equal(t, "Updated User", name)
				assert.Equal(t, "newpassword", password)
			},
		},
		{
			name:      "update with empty fields",
			tableName: "users_empty",
			initialSetup: func(t *testing.T, s *SQL) {
				_, err := s.db.Exec(`
					INSERT INTO users_empty (name, email, password)
					VALUES ($1, $2, $3)
				`, "Test User", "test@test.com", "password123")
				require.NoError(t, err)
			},
			fields: map[string]interface{}{},
			options: map[string]string{
				"table": "users",
			},
			filters: []filters.Filter{
				{
					Operation:  filters.Equals,
					Field:      "email",
					Comparator: "test@test.com",
				},
			},
			wantErr: false,
		},
		{
			tableName: "users_invalid",
			name:      "update with invalid table",
			initialSetup: func(t *testing.T, s *SQL) {
				_, err := s.db.Exec(`
					INSERT INTO users_invalid (name, email, password)
					VALUES ($1, $2, $3)
				`, "Test User", "test@test.com", "password123")
				require.NoError(t, err)
			},
			fields: map[string]interface{}{
				"name": "Updated User",
			},
			options: map[string]string{
				"table": "users; DROP TABLE users;--",
			},
			wantErr: true,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_invalid").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			tableName: "users_none",
			name:      "update without table option",
			initialSetup: func(t *testing.T, s *SQL) {
				_, err := s.db.Exec(`
					INSERT INTO users_none (name, email, password)
					VALUES ($1, $2, $3)
				`, "Test User", "test@test.com", "password123")
				require.NoError(t, err)
			},
			fields: map[string]interface{}{
				"name": "Updated User",
			},
			options: map[string]string{},
			wantErr: true,
		},
		{
			tableName: "users_multiple_filter",
			name:      "update multiple records with filter",
			initialSetup: func(t *testing.T, s *SQL) {
				// Insert multiple users with different ages
				_, err := s.db.Exec("ALTER TABLE users_multiple_filter ADD COLUMN IF NOT EXISTS age INTEGER")
				require.NoError(t, err)

				users := []struct {
					name     string
					email    string
					password string
					age      int
				}{
					{"User 1", "user1@test.com", "pass1", 20},
					{"User 2", "user2@test.com", "pass2", 30},
					{"User 3", "user3@test.com", "pass3", 40},
				}

				for _, u := range users {
					_, err := s.db.Exec(`
						INSERT INTO users_multiple_filter (name, email, password, age)
						VALUES ($1, $2, $3, $4)
					`, u.name, u.email, u.password, u.age)
					require.NoError(t, err)
				}
			},
			fields: map[string]interface{}{
				"name": "Senior User",
			},
			options: map[string]string{
				"table": "users_multiple_filter",
			},
			filters: []filters.Filter{
				{
					Operation:  ">",
					Field:      "age",
					Comparator: 25,
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				rows, err := s.db.Query("SELECT name, age FROM users_multiple_filter ORDER BY age")
				require.NoError(t, err)
				defer rows.Close()

				var results []struct {
					name string
					age  int
				}

				for rows.Next() {
					var name string
					var age int
					err := rows.Scan(&name, &age)
					require.NoError(t, err)
					results = append(results, struct {
						name string
						age  int
					}{name, age})
				}

				require.Len(t, results, 3)
				assert.Equal(t, "User 1", results[0].name)
				assert.Equal(t, "Senior User", results[1].name)
				assert.Equal(t, "Senior User", results[2].name)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.tableName != "" {
				setupTestDB(t, s, tc.tableName)
			} else {
				setupTestDB(t, s, "default")
			}

			// Setup initial data
			if tc.initialSetup != nil {
				tc.initialSetup(t, s)
			}

			// Execute the update operation
			err := s.Update(context.Background(), tc.fields, tc.options, tc.filters...)

			// Check results
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Run any additional checks
			if tc.checkFn != nil {
				tc.checkFn(t, s)
			}
		})
	}
}

func TestSQL_Delete(t *testing.T) {
	// t.Parallel() - removed to ensure proper container handling

	sqlConnectionString := newDB(t)

	cfg := Config{
		Type:             "postgres",
		ConnectionString: sqlConnectionString,
	}
	s, err := newWrapper(cfg)
	require.NoError(t, err)

	type testUser struct {
		name     string
		email    string
		password string
	}

	testCases := []struct {
		name         string
		initialUsers []testUser
		filters      []filters.Filter
		options      map[string]string
		tableName    string
		wantErr      bool
		checkFn      func(*testing.T, *SQL)
	}{
		{
			name: "delete all records",
			initialUsers: []testUser{
				{"User 1", "user1@test.com", "pass1"},
				{"User 2", "user2@test.com", "pass2"},
				{"User 3", "user3@test.com", "pass3"},
			},
			filters:   []filters.Filter{},
			tableName: "users",
			options: map[string]string{
				"table": "users",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
		{
			name:      "delete specific record",
			tableName: "users_specific",
			initialUsers: []testUser{
				{"User 1", "user1@test.com", "pass1"},
				{"User 2", "user2@test.com", "pass2"},
				{"User 3", "user3@test.com", "pass3"},
			},
			filters: []filters.Filter{
				{
					Field:      "name",
					Operation:  "==",
					Comparator: "User 2",
				},
			},
			options: map[string]string{
				"table": "users_specific",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_specific").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 2, count)

				var exists int
				err = s.db.QueryRow("SELECT COUNT(*) FROM users_specific WHERE name = $1", "User 2").Scan(&exists)
				assert.NoError(t, err)
				assert.Equal(t, 0, exists)
			},
		},
		{
			name:      "delete with complex filter",
			tableName: "users_complex",
			initialUsers: []testUser{
				{"User 1", "user1@test.com", "pass1"},
				{"User 2", "user2@test.com", "pass2"},
				{"User 3", "user3@test.com", "pass3"},
			},
			filters: []filters.Filter{
				{
					Field:      "email",
					Operation:  "!=",
					Comparator: "user2@test.com",
				},
			},
			options: map[string]string{
				"table": "users_complex",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_complex").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 1, count)

				var email string
				err = s.db.QueryRow("SELECT email FROM users_complex").Scan(&email)
				assert.NoError(t, err)
				assert.Equal(t, "user2@test.com", email)
			},
		},
		{
			name:      "delete with multiple filters",
			tableName: "users_multiple",
			initialUsers: []testUser{
				{"Young User", "young@test.com", "pass1"},
				{"Old User", "old@test.com", "pass2"},
				{"Middle User", "middle@test.com", "pass3"},
			},
			filters: []filters.Filter{
				{
					Field:      "email",
					Operation:  "==",
					Comparator: "old@test.com",
				},
				{
					Field:      "name",
					Operation:  "==",
					Comparator: "Old User",
				},
			},
			options: map[string]string{
				"table": "users_multiple",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_multiple").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 2, count)

				var exists int
				err = s.db.QueryRow("SELECT COUNT(*) FROM users_multiple WHERE name = $1", "Old User").Scan(&exists)
				assert.NoError(t, err)
				assert.Equal(t, 0, exists)
			},
		},
		{
			name:      "delete with invalid table",
			tableName: "users_invalid",
			initialUsers: []testUser{
				{"User 1", "user1@test.com", "pass1"},
			},
			options: map[string]string{
				"table": "users; DROP TABLE users;--",
			},
			wantErr: true,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_invalid").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			name:      "delete without table option",
			tableName: "users_no",
			initialUsers: []testUser{
				{"User 1", "user1@test.com", "pass1"},
			},
			options: map[string]string{},
			wantErr: true,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_no").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 1, count)
			},
		},
		{
			tableName: "users_collection",
			name:      "delete with collection option instead of table",
			initialUsers: []testUser{
				{"User 1", "user1@test.com", "pass1"},
			},
			options: map[string]string{
				"collection": "users_collection",
			},
			wantErr: false,
			checkFn: func(t *testing.T, s *SQL) {
				var count int
				err := s.db.QueryRow("SELECT COUNT(*) FROM users_collection").Scan(&count)
				assert.NoError(t, err)
				assert.Equal(t, 0, count)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture range variable for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.tableName != "" {
				setupTestDB(t, s, tc.tableName)
			} else {
				setupTestDB(t, s, "default_users")
			}
			// Insert initial users
			for _, user := range tc.initialUsers {
				query := `INSERT INTO %s (name, email, password)
						VALUES ($1, $2, $3)
								`
				query = fmt.Sprintf(query, tc.tableName)
				_, err := s.db.Exec(query, user.name, user.email, user.password)
				require.NoError(t, err)
			}

			// Execute the delete operation
			err := s.Delete(context.Background(), tc.options, tc.filters...)

			// Check results
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Run any additional checks
			if tc.checkFn != nil {
				tc.checkFn(t, s)
			}
		})
	}
}
