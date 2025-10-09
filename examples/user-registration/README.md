# User Registration Example

Complete user registration and authentication system with validation, password hashing, and JWT tokens.

## Quick Setup

1. **Install ServFlow Engine** (if not already installed):
   ```bash
   curl -fsSL https://raw.githubusercontent.com/servflow/servflow/main/install.sh | bash
   ```

2. **Set up your environment**:
   ```bash
   export MONGODB_STRING="mongodb://username:password@localhost:27017/yourdb"
   export JWT_SECRET="your-super-secure-jwt-secret-key"
   ```

3. **Start ServFlow**:
   ```bash
   servflow start --integrations integrations.yaml configs/
   ```

4. **Test user registration**:
   ```bash
   curl -X POST http://localhost:8080/register \
     -H "Content-Type: application/json" \
     -d '{
       "email": "john@example.com",
       "password": "securePassword123",
       "name": "John Doe"
     }'
   ```

   **Expected Response**:
   ```json
   {
     "success": true,
     "message": "User registered successfully",
     "user": {
       "id": "uuid-string",
       "email": "john@example.com", 
       "name": "John Doe"
     },
     "token": "jwt-token-here"
   }
   ```

## Prerequisites

- **MongoDB database** for user storage
- **Strong JWT secret** for token signing
- Users collection will be created automatically

## What's Included

- `configs/user-registration.yaml` - Complete registration API configuration
- `integrations.yaml` - MongoDB integration setup
- Input validation, password hashing, JWT token generation

## Sample Test Cases

```bash
# Test validation error
curl -X POST http://localhost:8080/register \
  -d '{"email": "invalid-email"}'

# Test duplicate registration
curl -X POST http://localhost:8080/register \
  -d '{"email": "john@example.com", "password": "test123", "name": "John"}'
```

## What This Demonstrates

- **Input validation** - Required fields and email format checking
- **Password security** - bcrypt hashing for secure storage
- **Database operations** - User creation and duplicate prevention
- **JWT tokens** - Secure authentication token generation
- **Error handling** - Comprehensive validation and error responses

## Learn More

**→ [User Registration Tutorial](https://docs.servflow.io/getting-started/user-registration-api)** - Complete step-by-step explanation

**→ [Authentication Patterns](https://docs.servflow.io/concepts/actions#authenticate)** - All authentication options

**→ [JWT Configuration](https://docs.servflow.io/concepts/actions#jwt)** - Token generation and validation

## Common Issues

**"MongoDB connection failed"**
- Verify `MONGODB_STRING` environment variable
- Ensure MongoDB is running and accessible

**"JWT token generation failed"**
- Check `JWT_SECRET` environment variable is set
- Use a strong, unique secret key

**Need more help?** Check the [complete troubleshooting guide](https://docs.servflow.io/reference/troubleshooting)

---

**Setup time: 3 minutes** ⚡