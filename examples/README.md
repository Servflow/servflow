# ServFlow Engine Examples

Ready-to-run examples that demonstrate real-world ServFlow Engine patterns. Each example is designed to work out-of-the-box with minimal setup.

## 🚀 Quick Start

```bash
# 1. Clone this repository
git clone https://github.com/servflow/servflow.git
cd servflow/examples/getting-started

# 2. Choose an example and follow its README
cd hello-world
servflow start --integrations integrations.yaml configs/

# 3. Test your API
curl http://localhost:8080/hello
```

---

## 📁 Available Examples

### 🟢 [Hello World](./getting-started/hello-world/)
**The simplest possible ServFlow API**
- **What it does**: Returns a JSON response with message and timestamp
- **Setup time**: 30 seconds
- **Prerequisites**: None
- **Learn**: Basic API structure, static responses, template functions

```bash
cd hello-world
servflow start --integrations integrations.yaml configs/
curl http://localhost:8080/hello
```

**→ [Complete Tutorial](https://docs.servflow.io/getting-started/your-first-api)**

---

### 🔴 [User Registration](./getting-started/user-registration/)
**Complete user signup and authentication system**
- **What it does**: User registration with validation, password hashing, JWT tokens
- **Setup time**: 3 minutes
- **Prerequisites**: Database (MongoDB/PostgreSQL), email service (optional)
- **Learn**: Authentication, validation, security, JWT tokens

```bash
cd user-registration
export MONGODB_STRING="mongodb://localhost:27017/mydb"
servflow start --integrations integrations.yaml configs/
curl -X POST http://localhost:8080/register -d '{"email": "user@example.com", "password": "secure123"}'
```

**→ [Complete Tutorial](https://docs.servflow.io/getting-started/user-registration-api)**

---

## 🎯 Learning Path

### Beginner (New to ServFlow)
1. **Start with Hello World** - Understand the basics
2. **Read the tutorial** - Learn why it works
3. **Modify the example** - Practice configuration
4. **Move to intermediate examples**

### Intermediate (Ready for real features)
1. **Try Database Agent** - Add AI capabilities
2. **Experiment with queries** - Natural language processing
3. **Connect your own database** - Real data integration
4. **Explore authentication examples**

### Advanced (Building production APIs)
1. **Use User Registration** - Complete application patterns
2. **Combine multiple examples** - Build complex workflows
3. **Create custom integrations** - Connect to your services
4. **Deploy to production** - Scale your APIs

---

## 🛠️ Example Structure

Every example follows the same pattern:

```
example-name/
├── configs/              # API endpoint definitions
│   └── example.yaml     # Main API configuration
├── integrations.yaml    # Database and service connections  
├── README.md           # Quick setup guide
└── [optional files]   # Additional configs or sample data
```

### Quick Setup Pattern

All examples use this workflow:

```bash
# 1. Navigate to example
cd getting-started/[example-name]

# 2. Set environment variables (if needed)
export SECRET_NAME="your-value"

# 3. Start ServFlow
servflow start --integrations integrations.yaml configs/

# 4. Test the API
curl [example-specific-command]
```

---

## 📚 Documentation & Tutorials

### Complete Guides
Each example has a detailed tutorial that explains:
- **How it works** - Step-by-step breakdown
- **Configuration syntax** - Understanding each section
- **Customization options** - Adapting for your needs
- **Production considerations** - Security, scaling, deployment

### Quick Links
- **[Example Walkthrough](https://docs.servflow.io/getting-started/example-walkthrough)** - How to use this repository
- **[Installation Guide](https://docs.servflow.io/getting-started/installation)** - Get ServFlow Engine running
- **[Core Concepts](https://docs.servflow.io/concepts/)** - Understand ServFlow architecture
- **[Actions Reference](https://docs.servflow.io/concepts/actions)** - All available building blocks

---

## 🔧 Working with Examples

### Copy to Your Project

```bash
# Create your project
mkdir my-servflow-api
cd my-servflow-api

# Copy example files
cp -r /path/to/examples/hello-world/* .

# Customize for your needs
vi configs/hello-world.yaml
vi integrations.yaml
```

### Modify and Experiment

Examples are meant to be modified:

- **Change API paths** - Update `listenPath` values
- **Add new fields** - Extend response objects  
- **Connect different services** - Modify integrations
- **Combine examples** - Build complex APIs

### Test Your Changes

```bash
# Restart ServFlow after changes
servflow start --integrations integrations.yaml configs/

# Test with curl
curl http://localhost:8080/your-endpoint

# Check logs for errors
```

---

## 🎯 What Each Example Teaches

### Core ServFlow Concepts
- **API Configuration** - YAML structure and syntax
- **HTTP Endpoints** - Routing and methods
- **Template Functions** - Dynamic data processing
- **Response Building** - JSON structure and formatting

### Integration Patterns
- **Database Connections** - MongoDB, PostgreSQL, MySQL
- **External APIs** - HTTP requests and response processing
- **Authentication** - JWT tokens, user management

### Advanced Features
- **Conditional Logic** - Branching and decision making
- **Error Handling** - Graceful failure management
- **Security** - Input validation, secret management
- **Multi-step Workflows** - Complex business logic

---

## 🔍 Common Issues

### Environment Variables
```bash
# Check if secrets are set
echo $MONGODB_STRING

# Set missing variables
export MONGODB_STRING="your-connection-string"
```

### File Structure
```bash
# Ensure proper file layout
ls -la configs/
ls -la integrations.yaml
```

### API Testing
```bash
# Verify ServFlow is running
curl http://localhost:8080/health

# Check specific endpoint
curl http://localhost:8080/your-endpoint
```

### More Help
- **[Troubleshooting Guide](https://docs.servflow.io/reference/troubleshooting)** - Common problems and solutions
- **[GitHub Discussions](https://github.com/servflow/servflow/discussions)** - Community support
- **[Issues](https://github.com/servflow/servflow/issues)** - Bug reports and feature requests

---

## 🤝 Contributing Examples

### Want to Add an Example?

We're looking for examples that demonstrate:
- Email sending and notifications
- File upload and processing  
- Real-time WebSocket APIs
- Multi-step business workflows
- Error handling patterns
- Performance optimization

### Example Guidelines

Good examples are:
- **Self-contained** - Work without complex external setup
- **Educational** - Teach specific ServFlow concepts  
- **Realistic** - Solve real-world problems
- **Well-documented** - Clear setup and explanation

---

## 🔗 Links

- **📚 [Documentation](https://docs.servflow.io)** - Complete ServFlow Engine guide
- **🚀 [Download Engine](https://github.com/servflow/servflow/releases)** - Latest releases
- **💬 [Community](https://github.com/servflow/servflow/discussions)** - Questions and discussions
- **🐛 [Issues](https://github.com/servflow/servflow/issues)** - Bug reports and features
- **🌐 [Website](https://servflow.io)** - Main product site

---

**Total examples**: 3 (growing!)  
**Average setup time**: Under 3 minutes  
**Prerequisites**: ServFlow Engine installed  

Ready to build APIs with YAML? Start with [Hello World](./getting-started/hello-world/)! 🚀