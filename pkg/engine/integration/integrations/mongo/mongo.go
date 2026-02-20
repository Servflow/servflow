package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/Servflow/servflow/pkg/engine/integration"
	dbfilters "github.com/Servflow/servflow/pkg/engine/integration/integrations/filters"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	ConnectionString string `json:"connectionString"`
	DBName           string `json:"dbName"`
}

type Mongo struct {
	client *mongo.Client
	dbName string
}

func (m *Mongo) ExecuteQuery(ctx context.Context, collection string, filterQuery string, projectionQuery string) ([]map[string]interface{}, error) {
	var filter bson.M
	if filterQuery != "" {
		if err := bson.UnmarshalExtJSON([]byte(filterQuery), false, &filter); err != nil {
			return nil, fmt.Errorf("error processing filter query: %v", err)
		}
	}

	var projection bson.M
	if projectionQuery != "" {
		if err := bson.UnmarshalExtJSON([]byte(projectionQuery), false, &projection); err != nil {
			return nil, fmt.Errorf("error processing projection query: %v", err)
		}
	}

	db := m.client.Database(m.dbName)
	coll := db.Collection(collection)
	opts := options.Find().SetProjection(projection)

	cur, err := coll.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, fmt.Errorf("error executing query: %v", err)
	}

	var r []bson.M
	if err := cur.All(ctx, &r); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, m := range r {
		results = append(results, m)
	}
	return results, nil
}

func (m *Mongo) Delete(ctx context.Context, options map[string]string, filters ...dbfilters.Filter) error {
	c, ok := options[collectionOption]
	if !ok {
		return fmt.Errorf("invalid collection")
	}

	bsonFilter, err := dbfilters.FiltersToBSON(filters)
	if err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}

	_, err = m.client.Database(m.dbName).Collection(c).DeleteMany(ctx, bsonFilter)
	if err != nil {
		return fmt.Errorf("error deleting items: %w", err)
	}

	return nil
}

func (m *Mongo) Type() string {
	return "mongo"
}

var (
	collectionOption = "collection"
)

func init() {
	fields := map[string]integration.FieldInfo{
		"connectionString": {
			Type:        integration.FieldTypePassword,
			Label:       "Connection String",
			Placeholder: "mongodb://localhost:27017",
			Required:    true,
		},
		"dbName": {
			Type:        integration.FieldTypeString,
			Label:       "Database Name",
			Placeholder: "mydb",
			Required:    true,
		},
	}

	if err := integration.RegisterIntegration("mongo", integration.RegistrationInfo{
		Name:        "MongoDB",
		Description: "MongoDB database integration for document storage and retrieval",
		ImageURL:    "https://d2ojax9k5fldtt.cloudfront.net/mongo.svg",
		Fields:      fields,
		Constructor: func(m map[string]any) (integration.Integration, error) {
			return newWrapper(Config{
				ConnectionString: m["connectionString"].(string),
				DBName:           m["dbName"].(string),
			})
		},
	}); err != nil {
		panic(err)
	}
}

// TODO pass context for cancellation
// TODO databases need a release function

func newWrapper(cfg Config) (*Mongo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.ConnectionString).
		SetMaxConnIdleTime(5*time.Minute).
		SetSocketTimeout(30*time.Second).
		SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("error with mongo config: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("error creating connection: %v", err)
	}

	m := &Mongo{
		dbName: cfg.DBName,
		client: client,
	}
	return m, nil
}

func (m *Mongo) Update(ctx context.Context, fields map[string]interface{}, options map[string]string, filters ...dbfilters.Filter) error {
	c, ok := options[collectionOption]
	if !ok {
		return fmt.Errorf("invalid collection")
	}
	bsonFilter, err := dbfilters.FiltersToBSON(filters)
	if err != nil {
		return fmt.Errorf("invalid filters: %w", err)
	}

	_, err = m.client.Database(m.dbName).Collection(c).UpdateOne(ctx, bsonFilter, bson.M{"$set": fields})
	if err != nil {
		return fmt.Errorf("error with update: %w", err)
	}
	return nil
}

func (m *Mongo) Fetch(ctx context.Context, options map[string]string, filters ...dbfilters.Filter) (items []map[string]interface{}, err error) {
	c, ok := options[collectionOption]
	if !ok {
		return nil, fmt.Errorf("invalid collection")
	}

	bsonFilter, err := dbfilters.FiltersToBSON(filters)
	if err != nil {
		return nil, fmt.Errorf("invalid filters: %w", err)
	}
	cursor, err := m.client.Database(m.dbName).Collection(c).Find(ctx, bsonFilter)
	if err != nil {
		return nil, fmt.Errorf("error fetching items: %w", err)
	}

	var mResults []bson.M
	if err := cursor.All(ctx, &mResults); err != nil {
		return nil, fmt.Errorf("error getting items: %w", err)
	}

	results := make([]map[string]interface{}, len(mResults))
	for i, res := range mResults {
		results[i] = res
	}

	return results, nil
}

func (m *Mongo) Store(ctx context.Context, item map[string]interface{}, options map[string]string) error {
	_, err := m.client.Database(m.dbName).Collection(options[collectionOption]).InsertOne(ctx, item)
	if err != nil {
		return fmt.Errorf("error inserting item: %w", err)
	}
	return nil
}
