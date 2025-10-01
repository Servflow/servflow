# Servflow

[![GitHub Release](https://img.shields.io/github/release/servflow/servflow.svg)](https://github.com/servflow/servflow/releases)
[![License](https://img.shields.io/github/license/servflow/servflow.svg)](LICENSE)
[![Issues](https://img.shields.io/github/issues/servflow/servflow.svg)](https://github.com/servflow/servflow/issues)

**Servflow** is a powerful no-code backend tool that helps you build robust backends and workflows without writing code. Create complex data processing pipelines, API integrations, and business logic through an intuitive visual interface.

## 🚀 Features

- **Visual Workflow Builder**: Design complex workflows with drag-and-drop simplicity
- **Pre-built Actions**: Extensive library of ready-to-use workflow actions
- **Database Integration**: Connect to PostgreSQL, MySQL, MongoDB, and more
- **API Integrations**: Seamlessly integrate with external APIs and services
- **Real-time Processing**: Handle real-time data streams and events
- **Scalable Architecture**: Built for high-performance and horizontal scaling
- **Observability**: Built-in tracing and monitoring capabilities

## 📦 Installation

### Binary Installation (Recommended)

#### Linux/macOS (via curl)
```bash
# Download and install latest version
curl -fsSL https://github.com/servflow/servflow/releases/latest/download/install.sh | bash

# Or download specific version
curl -L "https://github.com/servflow/servflow/releases/download/v1.0.0/servflow-linux-amd64" -o servflow
chmod +x servflow
sudo mv servflow /usr/local/bin/
```

#### Windows (via PowerShell)
```powershell
# Download latest release
Invoke-WebRequest -Uri "https://github.com/servflow/servflow/releases/latest/download/servflow-windows-amd64.exe" -OutFile "servflow.exe"

# Add to PATH (optional)
$env:PATH += ";$PWD"
```

#### Manual Download

Download the latest release for your platform from the [Releases page](https://github.com/servflow/servflow/releases):

- **Linux**: `servflow-linux-amd64`
- **macOS**: `servflow-darwin-amd64` (Intel) or `servflow-darwin-arm64` (Apple Silicon)
- **Windows**: `servflow-windows-amd64.exe`

### Docker Installation

```bash
# Pull the latest image
docker pull ghcr.io/servflow/servflow:latest

# Run with basic configuration
docker run -d \
  --name servflow \
  -p 8080:8080 \
  ghcr.io/servflow/servflow:latest

# Run with persistent data and custom config
docker run -d \
  --name servflow \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config:/app/config \
  -e SERVFLOW_DATABASE_URL="postgres://user:pass@host:5432/dbname" \
  ghcr.io/servflow/servflow:latest
```

### Docker Compose

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  servflow:
    image: ghcr.io/servflow/servflow:latest
    ports:
      - "8080:8080"
    environment:
      - SERVFLOW_DATABASE_URL=postgres://servflow:password@postgres:5432/servflow
      - SERVFLOW_REDIS_URL=redis://redis:6379
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=servflow
      - POSTGRES_USER=servflow
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

Then run:
```bash
docker-compose up -d
```

## 🏃‍♂️ Quick Start

1. **Start Servflow**:
   ```bash
   servflow start
   ```

2. **Open the Web Interface**:
   Navigate to `http://localhost:8080` in your browser

3. **Create Your First Workflow**:
   - Click "New Workflow"
   - Drag actions from the sidebar
   - Connect actions to create your data flow
   - Configure each action's parameters
   - Save and run your workflow

## ⚙️ Configuration

Servflow can be configured via environment variables or a configuration file.

### Environment Variables

```bash
# Server configuration
SERVFLOW_PORT=8080
SERVFLOW_HOST=0.0.0.0

# Database
SERVFLOW_DATABASE_URL=postgres://user:pass@localhost:5432/servflow

# Redis (for caching and queues)
SERVFLOW_REDIS_URL=redis://localhost:6379

# Security
SERVFLOW_JWT_SECRET=your-secret-key
SERVFLOW_CORS_ORIGINS=http://localhost:3000,https://yourdomain.com

# Observability
SERVFLOW_OTEL_ENDPOINT=http://localhost:4317
SERVFLOW_LOG_LEVEL=info
```

### Configuration File

Create a `config.yaml` file:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  url: "postgres://user:pass@localhost:5432/servflow"

redis:
  url: "redis://localhost:6379"

security:
  jwt_secret: "your-secret-key"
  cors_origins:
    - "http://localhost:3000"
    - "https://yourdomain.com"

observability:
  otel_endpoint: "http://localhost:4317"
  log_level: "info"
```

## 📚 Documentation

- **[User Guide](https://docs.servflow.io)** - Complete user documentation
- **[API Reference](https://docs.servflow.io/api)** - REST API documentation
- **[Action Library](https://docs.servflow.io/actions)** - Available workflow actions
- **[Examples](https://docs.servflow.io/examples)** - Sample workflows and use cases

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

## 🔗 Links

- **Website**: [servflow.io](https://servflow.io)
- **Documentation**: [docs.servflow.io](https://docs.servflow.io)
- **Releases**: [GitHub Releases](https://github.com/servflow/servflow/releases)
- **Docker Images**: [GitHub Container Registry](https://github.com/servflow/servflow/pkgs/container/servflow)

---

Made with ❤️ by the Servflow team. Happy workflow building! 🚀