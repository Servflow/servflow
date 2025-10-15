![banner](https://servflow.io/images/banner.png)
<div align="center">

# 🚀 ServFlow Engine

**Free, standalone API engine - build backends with YAML instead of code**

[![GitHub Release](https://img.shields.io/github/release/servflow/servflow.svg)](https://github.com/servflow/servflow/releases)
[![License](https://img.shields.io/github/license/servflow/servflow.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/servflow/servflow.svg)](https://hub.docker.com/r/servflow/servflow)
[![Issues](https://img.shields.io/github/issues/servflow/servflow.svg)](https://github.com/servflow/servflow/issues)

📚 **[Complete Documentation & Tutorials](https://docs.servflow.io)** • 📁 **[Download Examples](#-examples)** • 💬 **[Community](https://github.com/servflow/servflow/discussions)**

---
</div>

![demo](https://servflow.io/images/demo.gif)

## What is ServFlow Engine?

**ServFlow Engine** is a free, standalone declarative API engine that transforms YAML configurations into production-ready APIs. No backend code required.

ServFlow Engine is part of the ServFlow platform. Use it standalone (free forever) or with the ServFlow Dashboard for visual development.

### ✨ Why ServFlow Engine?

- **⚡ Zero Backend Code**: Build complete APIs using only YAML configurations
- **🔗 Universal Integrations**: Connect to any database, AI service, or external API
- **🧠 AI-Powered**: Built-in support for OpenAI, Claude, and other AI services
- **📈 Infinitely Scalable**: Designed for high-performance and horizontal scaling
- **⚙️ Configuration-Driven**: Version control your entire API logic

**Example**: This YAML becomes a working API endpoint:

```yaml
# Complete example in examples/hello-world/
http:
  listenPath: /users
  method: GET
  next: $action.fetch_users

actions:
  fetch_users:
    type: mongoquery
    config:
      collection: users
      integrationID: mongo
    next: $response.success

responses:
  success:
    code: 200
    responseObject:
      fields:
        users:
          value: "{{ .variable_actions_fetch_users }}"
```

---

## 🚀 Quick Start

Get ServFlow Engine running in under 2 minutes:

### 1. Install ServFlow Engine

**macOS & Linux** (Recommended):
```bash
curl -fsSL https://raw.githubusercontent.com/servflow/servflow/main/install.sh | bash
```

**Manual Download**:
Download the latest binary from [GitHub Releases](https://github.com/servflow/servflow/releases)

**Docker**:
```bash
docker pull servflow/servflow:latest
```

### 2. Download Examples & Start

```bash
# Clone this repository for examples
git clone https://github.com/servflow/servflow.git
cd servflow/examples/hello-world

# Start with the hello-world example
servflow start --integrations integrations.yaml configs/
```

### 3. Test Your API

```bash
curl http://localhost:8080/hello
# Response: {"message": "Hello from ServFlow Engine!", "timestamp": "2024-01-15T10:30:00Z"}
```

**🎉 That's it!** You now have a running API built with just YAML configuration.

---

## 📁 Examples

Ready-to-run examples you can download and use immediately:

| 🎯 **Example** | 📋 **What it does** | ⏱️ **Setup Time** | 🔗 **Tutorial** |
|---|---|---|---|
| [**hello-world**](./examples/hello-world/) | Simple API response | 30 seconds | [Your First API](https://docs.servflow.io/getting-started/your-first-api) |
| [**db-agent**](./examples/db-agent/) | AI-powered database queries | 2 minutes | [Database Agent](https://docs.servflow.io/getting-started/building-db-agent) |
| [**user-registration**](./examples/user-registration/) | User signup with validation | 3 minutes | [User Registration](https://docs.servflow.io/getting-started/user-registration-api) |

### 🏃‍♂️ Using Examples

```bash
# 1. Clone this repository
git clone https://github.com/servflow/servflow.git
cd servflow/examples

# 2. Choose an example (e.g., db-agent)
cd db-agent

# 3. Follow the quick setup in each README
# 4. Visit the docs for complete explanations
```

Each example includes:
- ✅ Complete YAML configurations that work out-of-the-box
- ✅ Quick setup instructions (under 3 minutes)
- ✅ Sample test requests
- ✅ Links to detailed tutorials in our documentation

**→ [Complete tutorials and explanations at docs.servflow.io](https://docs.servflow.io/getting-started/)**

---

## 🔧 How It Works

ServFlow Engine uses two types of configuration files:

### 1. **Integrations** (`integrations.yaml`)
Define connections to databases, AI services, and external APIs:

```yaml
integrations:
  mongo:
    type: mongo
    config:
      connectionString: '{{ secret "MONGODB_STRING" }}'
      dbName: myapp
  openai:
    type: openai
    config:
      api_key: '{{ secret "OPENAI_API_KEY" }}'
```

### 2. **API Endpoints** (`configs/*.yaml`)
Define your API endpoints and business logic:

```yaml
id: users_api
name: Users API
http:
  listenPath: /users
  method: GET
  next: $action.fetch_users
# ... rest of configuration
```

**🔥 The Result**: A fully functional API endpoint with zero backend code!

**→ [Learn the complete configuration syntax in our docs](https://docs.servflow.io/concepts/)**

---

## 📚 Documentation & Learning

### 🎯 New to ServFlow?
- [**Installation Guide**](https://docs.servflow.io/getting-started/installation) - Complete setup instructions
- [**Your First API**](https://docs.servflow.io/getting-started/your-first-api) - Build your first endpoint in 5 minutes
- [**Example Walkthrough**](https://docs.servflow.io/getting-started/example-walkthrough) - How to use this repository's examples

### 🧠 Learn by Building
- [**Database Agent Tutorial**](https://docs.servflow.io/getting-started/building-db-agent) - Build AI-powered endpoints
- [**User Registration Tutorial**](https://docs.servflow.io/getting-started/user-registration-api) - Create secure user APIs
- [**Advanced Patterns**](https://docs.servflow.io/guides/) - Production-ready configurations

### 🚀 Reference & Advanced
- [**Actions Reference**](https://docs.servflow.io/concepts/actions) - All available actions
- [**Integrations Guide**](https://docs.servflow.io/concepts/integrations) - Connect to any service
- [**Template Functions**](https://docs.servflow.io/concepts/templates) - Dynamic data processing
- [**Production Deployment**](https://docs.servflow.io/guides/deployment) - Scale and secure your APIs

---

## 🛠️ Installation Options

### Binary Installation

#### Quick Install Script
```bash
curl -fsSL https://raw.githubusercontent.com/servflow/servflow/main/install.sh | bash
```

#### Manual Download
Download from [GitHub Releases](https://github.com/servflow/servflow/releases):
- **Linux (x64)**: `servflow-vX.X.X-linux-amd64.tar.gz`
- **macOS (Intel)**: `servflow-vX.X.X-darwin-amd64.tar.gz`  
- **macOS (Apple Silicon)**: `servflow-vX.X.X-darwin-arm64.tar.gz`
- **Windows (x64)**: `servflow-vX.X.X-windows-amd64.zip`

### Docker Installation

```bash
# Pull the latest image
docker pull servflow/servflow:latest

# Run with configuration
docker run -d \
  --name servflow \
  -p 8080:8080 \
  -v $(pwd)/integrations.yaml:/app/integrations.yaml \
  -v $(pwd)/configs:/app/configs \
  servflow/servflow:latest start --integrations /app/integrations.yaml /app/configs
```

---

## 🔧 Quick Configuration

### Environment Setup

```bash
# Create project structure
mkdir my-servflow-api && cd my-servflow-api
mkdir -p configs
touch integrations.yaml

# Set environment variables for secrets
export MONGODB_STRING="mongodb://localhost:27017/mydb"
export OPENAI_API_KEY="sk-your-api-key-here"

# Start ServFlow
servflow start --integrations integrations.yaml configs/
```

### Health Check

```bash
curl http://localhost:8080/health
# Response: ok
```

---

## 🌟 What You Can Build

### 🤖 AI-Powered APIs
- Natural language database queries
- Smart content generation  
- Automated data processing

### 📊 Data APIs
- RESTful database operations
- Complex query pipelines
- Real-time data streaming

### 🔐 Authentication Systems  
- User registration & login
- JWT token management
- Role-based permissions

### 🔗 Integration Hubs
- Multi-service orchestration
- Webhook processing
- Third-party API proxying

**→ [See all possibilities in our documentation](https://docs.servflow.io)**

---

## 🤝 Community & Support

### Get Help
- 📖 **[Documentation](https://docs.servflow.io)** - Comprehensive guides and tutorials
- 💬 **[GitHub Discussions](https://github.com/servflow/servflow/discussions)** - Community Q&A
- 🐛 **[Issues](https://github.com/servflow/servflow/issues)** - Bug reports and feature requests

### Contributing
We welcome contributions! Check out our examples and documentation for ways to help.

### Quick Links
- [🐞 Report a Bug](https://github.com/servflow/servflow/issues/new?template=bug_report.md)
- [💡 Request a Feature](https://github.com/servflow/servflow/issues/new?template=feature_request.md)
- [❓ Ask a Question](https://github.com/servflow/servflow/discussions)

---

## 🔍 Common Issues

### "servflow: command not found"
```bash
# Make sure binary is executable and in PATH
chmod +x servflow
sudo mv servflow /usr/local/bin/
```

### "Config folder for APIs must be specified"  
```bash
# Provide the correct path to your configs
servflow start --integrations integrations.yaml configs/
```

### Need more help?
Check our [complete troubleshooting guide](https://docs.servflow.io/reference/troubleshooting) or [open an issue](https://github.com/servflow/servflow/issues).

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🔗 Links

- 🌐 **Website**: [servflow.io](https://servflow.io)
- 📚 **Documentation**: [docs.servflow.io](https://docs.servflow.io)  
- 📦 **Releases**: [GitHub Releases](https://github.com/servflow/servflow/releases)
- 🐳 **Docker**: [Docker Hub](https://hub.docker.com/r/servflow/servflow)
- 💬 **Community**: [GitHub Discussions](https://github.com/servflow/servflow/discussions)

---

<div align="center">

**Made with ❤️ by the ServFlow team**

⭐ **[Star this repo](https://github.com/servflow/servflow)** if ServFlow helps you build better APIs!

[Get Started](https://docs.servflow.io/getting-started/installation) • [View Examples](./examples/) • [Read Docs](https://docs.servflow.io)

</div>
