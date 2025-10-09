package firestore

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func shouldSkip(t *testing.T) (serviceAcc, projectId string) {
	serviceAcc = os.Getenv("TEST_SERVICE_ACCOUNT_FILE")
	projectId = os.Getenv("TEST_GOOGLE_PROJECT_ID")
	if serviceAcc == "" || projectId == "" {
		t.Skip()
	}
	return
}

func TestFirestore_Execute(t *testing.T) {
	testServAcc, testProjectID := shouldSkip(t)

	file, err := os.OpenFile(testServAcc, os.O_RDONLY, 0644)
	require.NoError(t, err, "got error from opening file")
	contents, err := io.ReadAll(file)
	require.NoError(t, err, "got error from reading file")
	require.NotEmpty(t, contents, "contents empty")

	client, err := NewFirestoreExecutable(Config{
		ProjectID:        testProjectID,
		CollectionID:     "users",
		ServiceAccount:   string(contents),
		DocumentTemplate: `{"email":"{{ in.email }}","first_name":"{{ in.first_name }}","last_name":"{{ in.last_name }}"}`,
	})
	require.NoError(t, err, "error creating client")

	_, err = client.Execute(context.Background(), `{"email":"test_execute@gmail.com","first_name":"test","last_name":"user"}`)
	require.NoError(t, err, "error executing firestore client")

	// fetch created user
	resp, err := client.client.Collection("users").Where("email", "==", "test_execute@gmail.com").Documents(context.Background()).GetAll()
	assert.NoError(t, err, "error creating collection")
	if len(resp) < 1 {
		t.Fatal("expected created resource found none")
	}
	t.Cleanup(func() {
		_, err = resp[0].Ref.Delete(context.Background())
		assert.NoError(t, err, "error deleting")
	})
}
