# Database Agent API Example

This example demonstrates how to build an AI-powered database agent endpoint using ServFlow Engine. The agent allows users to interact with your database using natural language queries, making data access intuitive and conversational.

## What This Example Builds

A `POST /db_agent` endpoint that:

- ✅ Accepts natural language queries from users
- ✅ Uses AI to understand intent and generate database queries
- ✅ Safely queries your MongoDB database with appropriate filters
- ✅ Returns results in a conversational format
- ✅ Maintains conversation history for context
- ✅ Prevents access to sensitive fields like passwords

## Files in This Example

- `db-agent.yaml` - Complete AI agent endpoint configuration
- `integration.yaml` - Database and OpenAI integration setup
- `README.md` - This documentation file

## Prerequisites

Before running this example:

1. **ServFlow Engine** installed and running
2. **MongoDB database** with a `users` collection
3. **OpenAI API account** for AI capabilities
4. **Environment secrets** configured:
   - `MONGODB_STRING` - Your MongoDB connection string
   - `OPENAI_API_KEY` - Your OpenAI API key

## Setup Instructions

### 1. Configure Your Integrations

Copy the integration configuration to your ServFlow integrations file:

```bash
# If you don't have an integrations.yaml file, create one:
cp integration.yaml /path/to/servflow/integrations.yaml

# If you already have an integrations.yaml file, merge the content
cat integration.yaml >> /path/to/servflow/integrations.yaml
```

### 2. Set Environment Secrets

Configure the required secrets for your ServFlow Engine:

```bash
# MongoDB connection string
export MONGODB_STRING="mongodb://username:password@localhost:27017"

# OpenAI API key
export OPENAI_API_KEY="sk-your-openai-api-key-here"
```

### 3. Prepare Your Database

Ensure you have a `users` collection in your MongoDB database with documents like:

```json
{
  "_id": "ObjectId or UUID",
  "name": "John Doe",
  "email": "john@example.com",
  "id": "uuid-string",
  "created_at": "2024-01-15T10:30:00Z",
  "status": "active"
}
```

### 4. Deploy the API Configuration

Copy the API configuration to your ServFlow APIs folder:

```bash
cp db-agent.yaml /path/to/servflow/configs/apis/
```

### 5. Start ServFlow Engine

```bash
servflow start --integrations /path/to/servflow/integrations.yaml /path/to/servflow/configs/apis
```

## Testing the API

### Example 1: Finding Users by Name

```bash
curl -X POST http://localhost:8080/db_agent \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Find all users with the name John"
  }'
```

**Expected AI Response:**
```json
{
  "response": "I found 2 users named John in the database:\n\n1. John Doe (john.doe@example.com)\n2. John Smith (john.smith@example.com)"
}
```

### Example 2: Getting User Count

```bash
curl -X POST http://localhost:8080/db_agent \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How many users are in the database?"
  }'
```

**Expected AI Response:**
```json
{
  "response": "There are currently 150 users in the database."
}
```

### Example 3: Complex Query

```bash
curl -X POST http://localhost:8080/db_agent \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Show me users whose email contains gmail.com, but only return their names"
  }'
```

**Expected AI Response:**
```json
{
  "response": "Here are the users with Gmail addresses:\n\n1. John Doe\n2. Jane Smith\n3. Mike Johnson\n\nI found 3 users total with Gmail addresses."
}
```

### Example 4: Error Case - Missing Query

```bash
curl -X POST http://localhost:8080/db_agent \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Expected Response (400):**
```json
{
  "success": false,
  "message": "Query parameter is required",
  "errors": ["Missing or empty 'query' parameter"]
}
```

## Configuration Explanation

### AI Agent Configuration

```yaml
actions:
  agent:
    type: agent
    config:
      integrationID: my_openai_service
      userPrompt: '{{ param "query" }}'
      systemPrompt: |
        You are a helpful agent to assist the user interact and query data from a mongodb database.
        You have access to a mongodb querier tool that allows you fetch data using filter and projection queries on the collection "users".
        The collection has the fields name, email, id (uuid).
        *IMPORTANT* Never include password queries in your responses
```

Key components:
- **`integrationID`**: References your OpenAI integration
- **`userPrompt`**: The actual user query
- **`systemPrompt`**: Instructions that define agent behavior and security constraints
- **`history: true`**: Maintains conversation context

### Database Query Tool

```yaml
actions:
  queryDB:
    type: mongoquery
    config:
      collection: users
      filter: '{{ tool_param "filter" | stringescape }}'
      projection: '{{ tool_param "projection" | stringescape }}'
      integrationID: "my_database"
```

The agent uses this tool to execute actual database queries based on the AI-generated parameters.

## How the Agent Works

### 1. Natural Language Processing
The AI agent receives your natural language query and understands the intent using OpenAI's language models.

### 2. Query Planning
The agent determines what database operations are needed and plans the appropriate MongoDB queries.

### 3. Tool Execution
The agent calls the `mongodbquerier` tool with the appropriate filter and projection parameters.

### 4. Result Formatting
The agent receives the raw database results and formats them into a natural, conversational response.

### 5. Context Retention
With `history: true`, the agent remembers previous queries in the conversation for follow-up questions.

## Advanced Use Cases

### Multi-turn Conversations
Thanks to conversation history, you can have follow-up interactions:

1. **First query**: "Show me all users named John"
2. **Follow-up**: "What are their email addresses?"
3. **Follow-up**: "How many of them have gmail addresses?"

### Complex Filters
The agent can understand and generate complex MongoDB queries:
- "Find users created in the last week"
- "Show me users whose names start with 'A' and have yahoo email addresses"
- "Count users by email domain"

## Security Features

### Field Restrictions
The system prompt prevents access to sensitive fields like passwords. The agent will refuse queries that attempt to access restricted data.

### Collection Scope
The tool is configured to only access the `users` collection, preventing unauthorized access to other database collections.

### Input Sanitization
All parameters are properly escaped using `stringescape` to prevent injection attacks.

### API Key Security
OpenAI API keys are stored securely using the secrets management system.

## Customization Options

### Different Database Types

Replace MongoDB with SQL databases:

```yaml
actions:
  queryDB:
    type: sqlquery
    config:
      query: '{{ tool_param "query" | stringescape }}'
      integrationID: "my_postgres_db"
```

### Additional Collections

Expand the agent to work with multiple collections:

```yaml
systemPrompt: |
  You have access to multiple collections: users, orders, products.
  Available fields:
  - users: name, email, id, created_at, status
  - orders: id, user_id, total, created_at, status
  - products: name, price, category, stock
```

### Enhanced Security

Add user authentication before allowing database queries:

```yaml
conditionals:
  authenticateUser:
    expression: '{{ validateJWT (header "Authorization") }}'
    validPath: $action.agent
    invalidPath: $response.unauthorized
```

## Common Issues

### "OpenAI integration not found"
- Verify your `my_openai_service` integration is properly configured
- Check that your `OPENAI_API_KEY` secret is set
- Ensure the integration ID matches exactly

### "MongoDB connection failed"
- Verify your `my_database` integration is working
- Test the connection with a simple query first
- Check your `MONGODB_STRING` secret

### "Agent doesn't understand my query"
- Try rephrasing your question more clearly
- Be specific about what data you want to see
- Check that you're asking about fields that exist (name, email, id)

### "Tool execution failed"
- Verify the `queryDB` action configuration
- Check that the collection name matches your database
- Ensure proper JSON formatting in tool parameters

## Database Schema

This example expects a `users` collection with these fields:

```json
{
  "_id": "ObjectId",
  "name": "string",
  "email": "string",
  "id": "string (UUID)",
  "created_at": "timestamp",
  "status": "string"
}
```

## Next Steps

After getting this example working, consider these enhancements:

### 🔍 Expand Database Access
- Add more collections to the tool configuration
- Include additional fields (while maintaining security)
- Support for different database operations (updates, deletes)

### 🛡️ Enhanced Security
- Add user authentication before allowing database queries
- Implement rate limiting for API calls
- Add audit logging for database queries

### 💬 Improved Conversational Features
- Add support for data visualization requests
- Include export functionality (CSV, JSON)
- Add natural language explanations of query results

### 🔗 Integration with Other Services
- Connect to multiple databases
- Add vector search capabilities for semantic queries
- Include external API data in responses

## Learn More

- [ServFlow Documentation](https://docs.servflow.io)
- [API Configuration Guide](https://docs.servflow.io/concepts)
- [Available Actions Reference](https://docs.servflow.io/concepts/actions)
- [Agent Configuration Guide](https://docs.servflow.io/concepts/actions#agent)
- [Integration Setup Guide](https://docs.servflow.io/concepts/integrations)

Ready to build more AI-powered endpoints? This database agent pattern can be adapted for many different use cases including customer support, data analytics, and content management systems.