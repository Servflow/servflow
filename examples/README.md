# ServFlow Engine Examples

This directory contains practical examples demonstrating how to build APIs using the ServFlow Engine. Each example includes complete configurations, setup instructions, and explanations to help you get started quickly.

## Getting Started Examples

The examples in this directory are based on the [ServFlow Documentation Getting Started Guide](https://docs.servflow.io/getting-started/) and provide hands-on implementations of common API patterns.

### Available Examples

| Example | Description | Features |
|---------|-------------|----------|
| [**User Registration**](./getting-started/user-registration/) | Complete user registration API with validation and security | ✅ Input validation<br/>✅ Password hashing<br/>✅ Database storage<br/>✅ JWT tokens |
| [**Database Agent**](./getting-started/db-agent/) | AI-powered database query endpoint using natural language | ✅ Natural language queries<br/>✅ AI integration<br/>✅ Safe database access<br/>✅ Conversation history |

## Quick Start

1. **Choose an example** from the list above
2. **Navigate to the example directory** (e.g., `cd getting-started/user-registration/`)
3. **Follow the README** in that directory for setup instructions
4. **Copy configurations** to your ServFlow Engine
5. **Test the API** using the provided curl commands

## Prerequisites

Before running any examples, ensure you have:

- **ServFlow Engine** installed and running
- **Database access** (MongoDB, PostgreSQL, MySQL, etc.)
- **Required API keys** (OpenAI for AI examples)
- **Basic familiarity** with YAML configuration

## Example Structure

Each example directory contains:

- `*.yaml` - ServFlow API and integration configurations
- `README.md` - Detailed setup and usage instructions
- Example requests and responses
- Troubleshooting guide

## Configuration Patterns

### Integration Setup
All examples use placeholder integration IDs that you should replace with your own:

```yaml
integrations:
  - id: my_database          # Replace with your integration ID
    type: mongo              # or sql, qdrant, etc.
    config:
      connectionString: '{{ secret "YOUR_CONNECTION_SECRET" }}'
```

### Secret Management
Examples use the secure secret template syntax:

```yaml
connectionString: '{{ secret "MONGODB_STRING" }}'
api_key: '{{ secret "OPENAI_API_KEY" }}'
```

Set these secrets as environment variables or through your secrets management system.

### API Endpoint Patterns
Each example follows ServFlow's declarative workflow pattern:

```yaml
http:
  listenPath: /your-endpoint
  method: POST
  next: $conditional.validate

conditionals:
  validate:
    expression: '{{ your_validation_logic }}'
    validPath: $action.process
    invalidPath: $response.error

actions:
  process:
    type: action_type
    config:
      # action configuration
    next: $response.success

responses:
  success:
    code: 200
    responseObject:
      # response structure
```

## Testing Examples

### Health Check
First, verify your ServFlow Engine is running:

```bash
curl http://localhost:8080/health
```

### Example Requests
Each example includes curl commands for testing. Replace `localhost:8080` with your ServFlow Engine URL:

```bash
curl -X POST http://localhost:8080/your-endpoint \
  -H "Content-Type: application/json" \
  -d '{"your": "data"}'
```

## Common Setup Steps

### 1. Environment Configuration
Create a `.env` file or set environment variables:

```bash
export SERVFLOW_PORT=8080
export MONGODB_STRING="mongodb://user:pass@localhost:27017"
export OPENAI_API_KEY="sk-your-key-here"
```

### 2. Directory Structure
Organize your ServFlow configuration:

```
your-project/
├── configs/
│   └── apis/              # API endpoint configurations
│       └── example.yaml
├── integrations.yaml      # Integration configurations
└── .env                   # Environment variables
```

### 3. Running ServFlow
Start the engine with your configurations:

```bash
servflow start --integrations integrations.yaml configs/apis
```

## Integration Types

Examples demonstrate these integration types:

| Type | Purpose | Examples |
|------|---------|----------|
| `mongo` | MongoDB database | User storage, document queries |
| `sql` | PostgreSQL, MySQL | Relational data operations |
| `openai` | OpenAI API | AI-powered features |
| `qdrant` | Vector database | Similarity search |
| `sheets` | Google Sheets | Spreadsheet as database |

## Security Best Practices

Examples demonstrate secure patterns:

- **Secret Management**: Never hardcode sensitive values
- **Input Validation**: Validate all user inputs
- **Password Security**: Hash passwords with bcrypt
- **Access Control**: Limit database access scope
- **Error Handling**: Return appropriate error responses

## Troubleshooting

### Common Issues

**"Integration not found"**
- Check integration ID matches configuration
- Verify integration file is in correct directory

**"Secret not found"**
- Ensure environment variables are set
- Check secret name matches template usage

**"Connection failed"**
- Verify database/service is running
- Check connection strings and credentials

**"Port already in use"**
- Change `SERVFLOW_PORT` environment variable
- Kill process using the port: `lsof -ti:8080 | xargs kill`

### Debug Mode
Run ServFlow with debug logging:

```bash
SERVFLOW_ENV=debug servflow start --integrations integrations.yaml configs/apis
```

## Contributing Examples

Want to contribute more examples? Follow these guidelines:

1. **Create a new directory** under the appropriate category
2. **Include complete configurations** with placeholder IDs
3. **Write comprehensive README** with setup instructions
4. **Add test cases** with expected responses
5. **Follow security best practices**
6. **Test thoroughly** before submitting

## Learn More

- **[ServFlow Documentation](https://docs.servflow.io)** - Complete guide and reference
- **[Getting Started Guide](https://docs.servflow.io/getting-started/)** - Step-by-step tutorials
- **[API Configuration](https://docs.servflow.io/concepts/)** - Configuration concepts
- **[Available Actions](https://docs.servflow.io/concepts/actions)** - Action reference
- **[Integrations Guide](https://docs.servflow.io/concepts/integrations)** - Integration setup

## Support

- 📖 **Documentation**: [docs.servflow.io](https://docs.servflow.io)
- 🐛 **Issues**: [GitHub Issues](https://github.com/servflow/servflow/issues)
- 💬 **Community**: [GitHub Discussions](https://github.com/servflow/servflow/discussions)
- 📧 **Enterprise Support**: [support@servflow.com](mailto:support@servflow.com)

---

Happy building with ServFlow! 🚀