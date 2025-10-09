# Database Agent Example

AI-powered database queries using natural language - ask questions and get answers from your data.

## Quick Setup

1. **Install ServFlow Engine** (if not already installed):
   ```bash
   curl -fsSL https://raw.githubusercontent.com/servflow/servflow/main/install.sh | bash
   ```

2. **Set up your environment**:
   ```bash
   export MONGODB_STRING="mongodb://username:password@localhost:27017/yourdb"
   export OPENAI_API_KEY="sk-your-openai-api-key-here"
   ```

3. **Start ServFlow**:
   ```bash
   servflow start --integrations integrations.yaml configs/
   ```

4. **Test with natural language**:
   ```bash
   curl -X POST http://localhost:8080/db_agent \
     -H "Content-Type: application/json" \
     -d '{"query": "Show me all users named John"}'
   ```

   **Expected Response**:
   ```json
   {
     "response": "I found 2 users named John:\n\n1. John Doe (john@example.com)\n2. John Smith (john.smith@example.com)"
   }
   ```

## Prerequisites

- **MongoDB database** with a `users` collection
- **OpenAI API account** for AI capabilities
- Users collection should have fields: `name`, `email`, `id`

## What's Included

- `db-agent.yaml` - Complete AI agent API configuration
- `integrations.yaml` - MongoDB and OpenAI integration setup
- Ready-to-use configuration files

## Sample Queries to Try

```bash
# Count users
curl -X POST http://localhost:8080/db_agent \
  -d '{"query": "How many users are in the database?"}'

# Find by email domain
curl -X POST http://localhost:8080/db_agent \
  -d '{"query": "Show me users with gmail addresses"}'

# Complex queries
curl -X POST http://localhost:8080/db_agent \
  -d '{"query": "Find users created in the last week"}'
```

## What This Demonstrates

- **AI agents** - Natural language processing with OpenAI
- **Database queries** - MongoDB integration with dynamic filters
- **Conversation history** - Follow-up questions with context
- **Security** - Prevents access to sensitive fields like passwords

## Learn More

**→ [Database Agent Tutorial](https://docs.servflow.io/getting-started/building-db-agent)** - Complete step-by-step explanation

**→ [AI Agent Action Reference](https://docs.servflow.io/concepts/actions#agent)** - All configuration options

**→ [MongoDB Integration Guide](https://docs.servflow.io/concepts/integrations#mongodb)** - Database setup details

## Common Issues

**"OpenAI integration not found"**
- Verify `OPENAI_API_KEY` environment variable is set
- Check integration ID matches in configuration files

**"MongoDB connection failed"**  
- Verify `MONGODB_STRING` environment variable
- Ensure MongoDB is running and accessible

**Need more help?** Check the [complete troubleshooting guide](https://docs.servflow.io/reference/troubleshooting)

---

**Setup time: 2 minutes** ⚡