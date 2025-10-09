package mongo

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func startMongoContainer(t *testing.T) string {
	req := testcontainers.ContainerRequest{
		Image:        "mongo:latest",
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor:   wait.ForListeningPort("27017/tcp"),
	}

	mongoC, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %s", err)
	}

	t.Cleanup(func() {
		mongoC.Terminate(context.Background())
	})

	host, err := mongoC.Host(context.Background())
	if err != nil {
		t.Fatalf("Failed to get container host: %s", err)
	}

	port, err := mongoC.MappedPort(context.Background(), "27017")
	if err != nil {
		t.Fatalf("Failed to get container port: %s", err)
	}

	return "mongodb://" + host + ":" + port.Port()
}

// writeDataAndReturnCleanupFn writes data to the specified MongoDB collection and returns a cleanup function to delete the inserted document.
func writeDataAndReturnCleanupFn(client *mongo.Client, database, collection string, data map[string]interface{}) (id interface{}, ret func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	coll := client.Database(database).Collection(collection)

	// Add a new document with the provided data
	result, err := coll.InsertOne(ctx, data)
	if err != nil {
		log.Fatalf("Failed to add document: %v", err)
	}

	// Cleanup function
	return result.InsertedID, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := coll.DeleteOne(ctx, bson.M{"_id": result.InsertedID})
		if err != nil {
			log.Fatalf("Failed to delete document: %v", err)
		} else {
			fmt.Printf("Successfully deleted document with id: %v\n", result.InsertedID)
		}
	}
}

func TestMongo_NewWrapper(t *testing.T) {
	t.Parallel()
	uri := startMongoContainer(t)
	cfg := Config{
		ConnectionString: uri,
		DBName:           "servflow",
	}
	mng, err := newWrapper(cfg)
	assert.NoError(t, err)
	assert.Equal(t, "servflow", mng.dbName)
}

func TestMongo_ExecuteQuery(t *testing.T) {
	t.Parallel()
	runExecuteQuery := func(initialDocs []map[string]interface{}, filterQuery, projectionQuery string, expected []map[string]interface{}) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			uri := startMongoContainer(t)
			cfg := Config{
				ConnectionString: uri,
				DBName:           "servflow",
			}
			mng, err := newWrapper(cfg)
			require.NoError(t, err)

			// Insert initial documents
			for _, initialDoc := range initialDocs {
				_, cleanup := writeDataAndReturnCleanupFn(mng.client, "servflow", "users", initialDoc)
				t.Cleanup(cleanup)
			}

			// Execute the query
			results, err := mng.ExecuteQuery(context.Background(), "users", filterQuery, projectionQuery)
			require.NoError(t, err)

			require.Equal(t, len(expected), len(results))

			// Verify the results match expectations
			for i, result := range results {
				for k, v := range expected[i] {
					assert.Equal(t, v, result[k])
				}
				// If no projection is specified, ensure we don't check _id in comparison
				if projectionQuery == "" {
					for k, v := range result {
						if k != "_id" {
							assert.Equal(t, expected[i][k], v)
						}
					}
				}
			}
		}
	}

	initialDocs := []map[string]interface{}{
		{
			"name":  "john",
			"email": "john@example.com",
			"age":   int32(30),
		},
		{
			"name":  "jane",
			"email": "jane@example.com",
			"age":   int32(25),
		},
		{
			"name":  "bob",
			"email": "bob@example.com",
			"age":   int32(35),
		},
	}

	t.Run("no filter no projection", runExecuteQuery(
		initialDocs,
		"",
		"",
		initialDocs,
	))

	t.Run("with filter", runExecuteQuery(
		initialDocs,
		`{"name": "john"}`,
		"",
		[]map[string]interface{}{
			{
				"name":  "john",
				"email": "john@example.com",
				"age":   int32(30),
			},
		},
	))

	t.Run("with projection", runExecuteQuery(
		initialDocs,
		"",
		`{"name": 1, "email": 1}`,
		[]map[string]interface{}{
			{
				"name":  "john",
				"email": "john@example.com",
			},
			{
				"name":  "jane",
				"email": "jane@example.com",
			},
			{
				"name":  "bob",
				"email": "bob@example.com",
			},
		},
	))

	t.Run("with filter and projection", runExecuteQuery(
		initialDocs,
		`{"age": {"$gte": 30}}`,
		`{"name": 1, "age": 1}`,
		[]map[string]interface{}{
			{
				"name": "john",
				"age":  int32(30),
			},
			{
				"name": "bob",
				"age":  int32(35),
			},
		},
	))

	t.Run("no results", runExecuteQuery(
		initialDocs,
		`{"name": "nonexistent"}`,
		"",
		[]map[string]interface{}{},
	))

	t.Run("complex filter", runExecuteQuery(
		initialDocs,
		`{"$and": [{"age": {"$gt": 25}}, {"age": {"$lt": 35}}]}`,
		"",
		[]map[string]interface{}{
			{
				"name":  "john",
				"email": "john@example.com",
				"age":   int32(30),
			},
		},
	))
}

func TestMongo_Fetch(t *testing.T) {
	t.Parallel()
	runFetch := func(initialDocs, expected []map[string]interface{}, filters ...filters.Filter) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			uri := startMongoContainer(t)
			cfg := Config{
				ConnectionString: uri,
				DBName:           "servflow",
			}
			mng, err := newWrapper(cfg)
			require.NoError(t, err)

			for _, initialDoc := range initialDocs {
				_, cleanup := writeDataAndReturnCleanupFn(mng.client, "servflow", "users", initialDoc)
				t.Cleanup(cleanup)
			}

			fetched, err := mng.Fetch(context.Background(), map[string]string{collectionOption: "users"}, filters...)
			require.NoError(t, err)

			require.Equal(t, len(expected), len(fetched))

			for i, doc := range fetched {
				for k, v := range doc {
					if k != "_id" {
						assert.Equal(t, expected[i][k], v)
					}
				}
			}
		}
	}

	initialDocs := []map[string]interface{}{
		{
			"name":  "test",
			"email": "servflow",
		},
		{
			"name":  "test2",
			"email": "test2",
		},
	}
	t.Run("no filter", runFetch(initialDocs, initialDocs))
	t.Run("filter single", runFetch(initialDocs, []map[string]interface{}{
		{
			"name":  "test",
			"email": "servflow",
		},
	}, filters.Filter{
		Field:      "name",
		Operation:  "==",
		Comparator: "test",
	}))
	t.Run("should give empty results", runFetch(initialDocs, []map[string]interface{}{}, filters.Filter{
		Field:      "name",
		Operation:  "==",
		Comparator: "testa",
	}))
}

func TestMongo_Store(t *testing.T) {
	t.Parallel()
	runStoreTest := func(docToStore map[string]interface{}) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			uri := startMongoContainer(t)

			cfg := Config{
				ConnectionString: uri,
				DBName:           "servflow",
			}
			mng, err := newWrapper(cfg)
			require.NoError(t, err)

			err = mng.Store(context.Background(), docToStore, map[string]string{collectionOption: "users"})
			require.NoError(t, err)

			cursor, err := mng.client.Database("servflow").Collection("users").Find(context.Background(), bson.M{})
			require.NoError(t, err)

			var results []bson.M
			err = cursor.All(context.Background(), &results)
			require.NoError(t, err)

			require.Len(t, results, 1)
			for k, v := range docToStore {
				assert.Equal(t, v, results[0][k])
			}

			_, err = mng.client.Database("servflow").Collection("users").DeleteOne(context.Background(), bson.M{"_id": results[0]["_id"]})
			require.NoError(t, err)
		}
	}

	t.Run("store single item", runStoreTest(map[string]interface{}{"" +
		"name": "test",
		"email": "servflow",
	}))
}

func TestMongo_Update(t *testing.T) {
	runUpdate := func(initialDoc, expected map[string]interface{}, updateFields map[string]interface{}, filters ...filters.Filter) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			uri := startMongoContainer(t)
			cfg := Config{
				ConnectionString: uri,
				DBName:           "servflow",
			}
			mng, err := newWrapper(cfg)
			require.NoError(t, err)

			var (
				docID   interface{}
				cleanup func()
			)
			docID, cleanup = writeDataAndReturnCleanupFn(mng.client, "servflow", "users", initialDoc)
			t.Cleanup(cleanup)

			err = mng.Update(context.Background(), updateFields, map[string]string{collectionOption: "users"}, filters...)
			require.NoError(t, err)

			coll := mng.client.Database("servflow").Collection("users")
			cursor, err := coll.Find(context.Background(), bson.M{"_id": docID})
			require.NoError(t, err)

			var mResults []bson.M
			err = cursor.All(context.Background(), &mResults)
			require.NoError(t, err)

			// get first result
			gotten := map[string]interface{}(mResults[0])
			delete(gotten, "_id")
			assert.Equal(t, expected, gotten)
		}
	}

	t.Run("simple case", runUpdate(map[string]interface{}{
		"email": "test@gmail.com",
		"name":  "testName",
	}, map[string]interface{}{
		"email": "test@gmail.coms",
		"name":  "testName",
	}, map[string]interface{}{
		"email": "test@gmail.coms",
	}, filters.Filter{
		Field:      "name",
		Operation:  "==",
		Comparator: "testName",
	}))

	t.Run("should not update", runUpdate(map[string]interface{}{
		"email": "test@gmail.com",
		"name":  "testName",
	}, map[string]interface{}{
		"email": "test@gmail.com",
		"name":  "testName",
	}, map[string]interface{}{
		"email": "test@gmail.coms",
	}, filters.Filter{
		Field:      "name",
		Operation:  "!=",
		Comparator: "testName",
	}))

	t.Run("update with number", runUpdate(map[string]interface{}{
		"email": "test@gmail.com",
		"name":  "testName",
		"age":   2,
	}, map[string]interface{}{
		"email": "test@gmail.coms",
		"name":  "testName",
		"age":   int32(2),
	}, map[string]interface{}{
		"email": "test@gmail.coms",
	}, filters.Filter{
		Field:      "age",
		Operation:  ">",
		Comparator: 1,
	}))
}

func TestMongo_Delete(t *testing.T) {
	t.Parallel()
	runDelete := func(initialDocs []map[string]interface{}, deleteFilters []filters.Filter, expectedRemaining int) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			uri := startMongoContainer(t)
			cfg := Config{
				ConnectionString: uri,
				DBName:           "servflow",
			}
			mng, err := newWrapper(cfg)
			require.NoError(t, err)

			// Insert initial documents
			var cleanupFunctions []func()
			for _, initialDoc := range initialDocs {
				_, cleanup := writeDataAndReturnCleanupFn(mng.client, "servflow", "users", initialDoc)
				cleanupFunctions = append(cleanupFunctions, cleanup)
			}

			// Register cleanup functions to run after the test
			t.Cleanup(func() {
				for _, cleanup := range cleanupFunctions {
					cleanup()
				}
			})

			// Execute the delete operation
			err = mng.Delete(context.Background(), map[string]string{collectionOption: "users"}, deleteFilters...)
			require.NoError(t, err)

			// Verify the remaining documents count
			cursor, err := mng.client.Database("servflow").Collection("users").Find(context.Background(), bson.M{})
			require.NoError(t, err)

			var results []bson.M
			err = cursor.All(context.Background(), &results)
			require.NoError(t, err)

			assert.Equal(t, expectedRemaining, len(results))
		}
	}

	t.Run("delete all documents", runDelete(
		[]map[string]interface{}{
			{
				"name":  "test1",
				"email": "test1@example.com",
			},
			{
				"name":  "test2",
				"email": "test2@example.com",
			},
		},
		[]filters.Filter{},
		0,
	))

	t.Run("delete specific document", runDelete(
		[]map[string]interface{}{
			{
				"name":  "test1",
				"email": "test1@example.com",
			},
			{
				"name":  "test2",
				"email": "test2@example.com",
			},
		},
		[]filters.Filter{
			{
				Field:      "name",
				Operation:  "==",
				Comparator: "test1",
			},
		},
		1,
	))

	t.Run("delete with complex filter", runDelete(
		[]map[string]interface{}{
			{
				"name":  "test1",
				"email": "test1@example.com",
				"age":   25,
			},
			{
				"name":  "test2",
				"email": "test2@example.com",
				"age":   30,
			},
			{
				"name":  "test3",
				"email": "test3@example.com",
				"age":   35,
			},
		},
		[]filters.Filter{
			{
				Field:      "age",
				Operation:  ">",
				Comparator: 25,
			},
		},
		1,
	))

	t.Run("delete nothing", runDelete(
		[]map[string]interface{}{
			{
				"name":  "test1",
				"email": "test1@example.com",
			},
			{
				"name":  "test2",
				"email": "test2@example.com",
			},
		},
		[]filters.Filter{
			{
				Field:      "name",
				Operation:  "==",
				Comparator: "nonexistent",
			},
		},
		2,
	))
}
