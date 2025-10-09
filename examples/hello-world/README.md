# Hello World Example

The simplest possible ServFlow API - returns a JSON response with no external dependencies.

## Quick Setup

1. **Install ServFlow Engine** (if not already installed):
   ```bash
   curl -fsSL https://raw.githubusercontent.com/servflow/servflow/main/install.sh | bash
   ```

2. **Start ServFlow**:
   ```bash
   servflow start --integrations integrations.yaml configs/
   ```

3. **Test the API**:
   ```bash
   curl http://localhost:8080/hello
   ```

   **Expected Response**:
   ```json
   {
     "message": "Hello from ServFlow Engine!",
     "timestamp": "2024-01-15T10:30:00Z",
     "engine": "ServFlow"
   }
   ```

## What's Included

- `configs/hello-world.yaml` - Complete API configuration (no integrations needed)
- `integrations.yaml` - Empty integrations file
- Zero external dependencies - works immediately

## What This Demonstrates

- **Basic API structure** - HTTP endpoint definition
- **Static responses** - Returning JSON data
- **Template functions** - Using `{{ now }}` for dynamic timestamps
- **Minimal configuration** - Simplest possible ServFlow setup

## Learn More

**→ [Your First API Tutorial](https://docs.servflow.io/getting-started/your-first-api)** - Step-by-step explanation of this example

**→ [API Configuration Reference](https://docs.servflow.io/concepts/)** - Complete configuration syntax

**→ [Template Functions](https://docs.servflow.io/concepts/templates)** - All available template functions like `{{ now }}`

## Next Steps

Try these examples for more advanced functionality:
- **[Database Agent](../db-agent/)** - AI-powered database queries
- **[User Registration](../user-registration/)** - Authentication and validation

---

**Total setup time: 30 seconds** ⚡