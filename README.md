# Servflow Engine

[![GitHub Release](https://img.shields.io/github/release/servflow/servflow.svg)](https://github.com/servflow/servflow/releases)
[![License](https://img.shields.io/github/license/servflow/servflow.svg)](LICENSE)
[![Issues](https://img.shields.io/github/issues/servflow/servflow.svg)](https://github.com/servflow/servflow/issues)

**Servflow Engine** is a powerful declarative API engine that allows you to build robust backend APIs without traditional backend code. The engine processes API configurations and handles data processing pipelines, database integrations, and business logic execution.

## 🚀 Features

- **Declarative API Engine**: Process API endpoint definitions without traditional backend coding
- **Database Integration**: Connect to PostgreSQL, MySQL, MongoDB, and more
- **Configuration-Driven**: Define APIs and integrations through YAML configurations
- **API Integrations**: Seamlessly integrate with external APIs and services
- **Real-time Processing**: Handle real-time data streams and events
- **Scalable Architecture**: Built for high-performance and horizontal scaling
- **Observability**: Built-in tracing and monitoring capabilities

## 📦 Installation

### Binary Installation (Recommended)

#### Quick Install (Linux/macOS)
```bash
curl -fsSL https://github.com/servflow/servflow/releases/latest/download/install.sh | bash
```

#### Manual Download

Download the latest release archive for your platform from the [Releases page](https://github.com/servflow/servflow/releases):

- **Linux (x64)**: `servflow-vX.X.X-linux-amd64.tar.gz`
- **Linux (ARM64)**: `servflow-vX.X.X-linux-arm64.tar.gz`
- **macOS (Intel)**: `servflow-vX.X.X-darwin-amd64.tar.gz`
- **macOS (Apple Silicon)**: `servflow-vX.X.X-darwin-arm64.tar.gz`
- **Windows (x64)**: `servflow-vX.X.X-windows-amd64.zip`

#### Linux/macOS Manual Installation
```bash
# Download and extract (example for Linux x64 - replace with your platform's archive)
wget https://github.com/servflow/servflow/releases/latest/download/servflow-vX.X.X-linux-amd64.tar.gz
tar -xzf servflow-vX.X.X-linux-amd64.tar.gz
chmod +x servflow
sudo mv servflow /usr/local/bin/
```

#### Windows Manual Installation
```powershell
# Download and extract
Invoke-WebRequest -Uri "https://github.com/servflow/servflow/releases/latest/download/servflow-vX.X.X-windows-amd64.zip" -OutFile "servflow.zip"
Expand-Archive -Path "servflow.zip" -DestinationPath "."
# Add servflow.exe to your PATH
```

#### Verifying Downloads (Recommended)

For security, verify the integrity of your download using the provided checksums:

```bash
# Download the checksums file
wget https://github.com/servflow/servflow/releases/latest/download/checksums.txt

# Verify your download (Linux/macOS example)
sha256sum -c checksums.txt --ignore-missing

# Or verify individual file
echo "EXPECTED_SHA256  servflow-vX.X.X-linux-amd64.tar.gz" | sha256sum -c
```

On Windows:
```powershell
# Download checksums.txt and verify
$expectedHash = "EXPECTED_SHA256_HERE"
$actualHash = (Get-FileHash -Path "servflow-vX.X.X-windows-amd64.zip" -Algorithm SHA256).Hash
if ($expectedHash -eq $actualHash) { Write-Host "✓ Checksum verified" } else { Write-Host "✗ Checksum mismatch" }
```

### Docker Installation

```bash
# Pull the latest image
docker pull servflow/servflow:latest

# Run with proper configuration folders
docker run -d \
  --name servflow \
  -p 8080:8080 \
  -v $(pwd)/integrations.yaml:/app/integrations.yaml \
  -v $(pwd)/configs/apis:/app/configs \
  -e SERVFLOW_PORT=8080 \
  servflow/servflow:latest start --integrations /app/integrations.yaml /app/configs
```



## 🏃‍♂️ Quick Start

### Prerequisites

Before running the Servflow Engine, ensure you have the following (optional but recommended):

- **Database** - PostgreSQL, MySQL, or MongoDB for data persistence

### Configuration Setup

The Servflow Engine requires configuration folders for APIs and integrations:

```bash
# Create required directories
mkdir -p configs/apis
touch integrations.yaml
```



### Running the Engine

1. **Start the Servflow Engine**:
   ```bash
   servflow start --integrations integrations.yaml configs/apis
   ```

2. **Verify Engine is Running**:
   ```bash
   curl http://localhost:8080/health
   ```
   
   Expected response:
   ```json
   {
     "status": "healthy",
     "timestamp": "2024-01-15T10:30:00Z"
   }
   ```

3. **Create Your First Integration and API**:
   - Set up database connections in `integrations.yaml`
   - Define API endpoints in `configs/apis/`
   - Test your endpoints with HTTP requests

### Directory Structure

After setup, your project should look like this:

```
your-project/
├── servflow                    # The Servflow Engine binary
├── .env                       # Environment configuration
├── integrations.yaml          # Integration configurations
├── configs/
│   └── apis/                  # API endpoint definitions
│       └── your-api.yaml      # Your API configurations
└── docker-compose.yml         # Optional Docker setup
```

## 📁 Examples

Ready-to-use examples are available in the [`examples/`](./examples/) directory. These examples demonstrate common API patterns and are based on our [Getting Started Guide](https://docs.servflow.io/getting-started/).

### Available Examples

| Example | Description | Features |
|---------|-------------|----------|
| [**User Registration**](./examples/getting-started/user-registration/) | Complete user registration API with validation and security | Input validation, password hashing, database storage, JWT tokens |
| [**Database Agent**](./examples/getting-started/db-agent/) | AI-powered database query endpoint using natural language | Natural language queries, AI integration, safe database access, conversation history |

### Quick Start with Examples

1. **Choose an example** from the [`examples/`](./examples/) directory
2. **Navigate to the example** (e.g., `cd examples/getting-started/user-registration/`)
3. **Follow the README** in that directory for detailed setup instructions
4. **Copy configurations** to your ServFlow Engine folders
5. **Test the API** using the provided curl commands

Each example includes:
- Complete YAML configurations
- Integration setup instructions
- Test requests and expected responses
- Troubleshooting guides

## ⚙️ Configuration

Servflow can be configured via environment variables if needed:

```bash
# Optional server configuration (defaults shown)
SERVFLOW_PORT=8080
SERVFLOW_ENV=debug
```

### Configuration Files

Create integration configurations in `integrations.yaml`:

```yaml
# Integration Configuration
integrations:
  - id: user_database
    type: mongo
    config:
      connectionString: "{{ secret 'MONGODB_CONNECTION_STRING' }}"
      dbName: myapp

  - id: openai_service
    type: openai
    config:
      api_key: "{{ secret 'OPENAI_API_KEY' }}"
```

Create API configurations in `configs/apis/example.yaml`:

```yaml
# API Endpoint Configuration
id: example_api
name: Example API

http:
  listenPath: /users
  method: GET
  next: $action.fetchUsers

actions:
  fetchUsers:
    type: fetch
    config:
      integrationID: user_database
      table: users
      filters:
        - field: status
          operator: eq
          value: "active"
    next: $response.success

responses:
  success:
    code: 200
    responseObject:
      fields:
        status:
          value: "success"
        data:
          value: "{{ .variable_actions_fetchUsers }}"
```

## 🔧 Troubleshooting

### Common Issues

**"Config folder for APIs must be specified"**
- Ensure you've provided the API config folder as a command line argument
- Check that the folder path exists and is accessible

**"Integration file must be specified"**  
- Use the `--integrations` flag with the path to your integrations.yaml file when starting the Servflow Engine

**"Permission denied" when running binary**
- Make the binary executable: `chmod +x servflow`

**Port already in use**
- Change the port in your `.env` file: `SERVFLOW_PORT=8081`
- Or kill the process using port 8080: `lsof -ti:8080 | xargs kill`

## 📚 Documentation

- **[Getting Started](https://docs.servflow.io/getting-started/installation)** - Installation and setup guide
- **[Database Agent Tutorial](https://docs.servflow.io/getting-started/db-agent)** - Build an AI-powered database query endpoint
- **[User Registration Tutorial](https://docs.servflow.io/getting-started/user-registration)** - Create a complete user registration API
- **[API Configuration Concepts](https://docs.servflow.io/concepts/)** - Learn how to structure API workflows
- **[Actions Reference](https://docs.servflow.io/concepts/actions)** - Complete guide to available actions
- **[Integrations Guide](https://docs.servflow.io/concepts/integrations)** - Connect to databases and external services

## 🐛 Bug Reports & Feature Requests

Found a bug or have a feature request? Please check our [issue tracker](https://github.com/servflow/servflow/issues) and create a new issue if needed.

### Quick Links
- [🐞 Report a Bug](https://github.com/servflow/servflow/issues/new?template=bug_report.md)
- [💡 Request a Feature](https://github.com/servflow/servflow/issues/new?template=feature_request.md)
- [🔄 Request a New Action](https://github.com/servflow/servflow/issues/new?template=action_request.md)
- [💬 General Feedback](https://github.com/servflow/servflow/issues/new?template=general_feedback.md)

## 🤝 Support

- **Community Support**: [GitHub Issues](https://github.com/servflow/servflow/issues)
- **Documentation**: [docs.servflow.io](https://docs.servflow.io)
- **Enterprise Support**: [support@servflow.com](mailto:support@servflow.com)

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📝 Important Notes

**About Servflow Engine**: This repository contains the Servflow Engine - the backend API processing engine that executes your API configurations through declarative YAML files.

## 🔗 Links

- **Website**: [servflow.io](https://servflow.io)
- **Documentation**: [docs.servflow.io](https://docs.servflow.io)
- **Releases**: [GitHub Releases](https://github.com/servflow/servflow/releases)
- **Docker Images**: [Docker Hub](https://hub.docker.com/r/servflow/servflow)

---

Made with ❤️ by the Servflow team. Happy API building! 🚀