# User Registration API Example

This example demonstrates how to build a complete user registration endpoint using ServFlow Engine. The endpoint handles user validation, password hashing, database storage, and JWT token generation - all through declarative configuration.

## What This Example Builds

A `POST /register` endpoint that:

- ✅ Validates required fields (email, password, name)
- ✅ Checks for existing users to prevent duplicates
- ✅ Securely hashes passwords using bcrypt
- ✅ Stores user data in MongoDB
- ✅ Generates JWT tokens for immediate authentication
- ✅ Returns appropriate success/error responses

## Files in This Example

- `user-registration.yaml` - Complete API endpoint configuration
- `integration.yaml` - Database and service integration setup
- `README.md` - This documentation file

## Prerequisites

Before running this example:

1. **ServFlow Engine** installed and running
2. **MongoDB database** accessible
3. **Environment secrets** configured:
   - `MONGODB_STRING` - Your MongoDB connection string
   - `JWT_SECRET` - Secret key for JWT token signing

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
# Example MongoDB connection string
export MONGODB_STRING="mongodb://username:password@localhost:27017"

# Generate a secure JWT secret
export JWT_SECRET="your-super-secure-jwt-secret-key-here"
```

### 3. Deploy the API Configuration

Copy the API configuration to your ServFlow APIs folder:

```bash
cp user-registration.yaml /path/to/servflow/configs/apis/
```

### 4. Start ServFlow Engine

```bash
servflow start --integrations /path/to/servflow/integrations.yaml /path/to/servflow/configs/apis
```

## Testing the API

### Successful Registration

```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "securepassword123",
    "name": "John Doe"
  }'
```

**Expected Response (201):**
```json
{
  "success": true,
  "message": "User registered successfully",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "email": "john@example.com",
    "name": "John Doe"
  }
}
```

### Validation Error

```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com"
  }'
```

**Expected Response (400):**
```json
{
  "success": false,
  "message": "Missing required fields: email, password, and name are required"
}
```

### Duplicate User Error

```bash
# Try to register the same email twice
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "anotherpassword",
    "name": "John Smith"
  }'
```

**Expected Response (409):**
```json
{
  "success": false,
  "message": "User with this email already exists"
}
```

## Configuration Explanation

### HTTP Endpoint
```yaml
http:
  listenPath: /register
  method: POST
  next: $conditional.validateInput
```
Defines the POST endpoint and starts the workflow with input validation.

### Input Validation
```yaml
conditionals:
  validateInput:
    expression: |
      {{ and
        (notempty (param "email") "Email" true)
        (notempty (param "password") "Password" true)
        (notempty (param "name") "Name" true)
      }}
```
Uses template functions to ensure all required fields are present and non-empty.

### Duplicate User Check
```yaml
actions:
  checkExistingUser:
    type: fetch
    config:
      integrationID: my_database
      table: users
      filters:
        - field: email
          operator: eq
          value: '{{ param "email" }}'
```
Queries the database to check if a user with the provided email already exists.

### Password Security
```yaml
actions:
  hashPassword:
    type: hash
    config:
      value: '{{ param "password" }}'
      algorithm: bcrypt
```
Securely hashes passwords using bcrypt before storage.

### Data Storage
```yaml
actions:
  createUser:
    type: store
    config:
      integrationID: my_database
      table: users
      fields:
        email: '{{ param "email" }}'
        name: '{{ param "name" }}'
        password: '{{ .variable_actions_hashPassword }}'
        created_at: '{{ now }}'
        status: active
```
Stores the new user with hashed password and metadata.

## Database Schema

This example expects a `users` collection/table with these fields:

```json
{
  "_id": "ObjectId or UUID",
  "email": "string (unique)",
  "name": "string",
  "password": "string (hashed)",
  "created_at": "timestamp",
  "status": "string"
}
```

## Security Features

- **Password Hashing**: Uses bcrypt for secure password storage
- **Duplicate Prevention**: Checks for existing users before creation
- **Input Validation**: Validates all required fields
- **JWT Tokens**: Generates secure tokens for authentication
- **Secret Management**: Uses secure secret storage for sensitive data

## Customization Options

### Different Database Types
Replace the MongoDB integration with SQL or other supported databases:

```yaml
integrations:
  - id: my_database
    type: sql
    config:
      connectionString: '{{ secret "DATABASE_URL" }}'
      driver: postgres
```

### Additional Validation
Add more sophisticated validation rules:

```yaml
conditionals:
  validateEmail:
    expression: '{{ regexMatch "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$" (param "email") }}'
    validPath: $conditional.validatePassword
    invalidPath: $response.invalidEmail
```

### Email Verification
Add email verification workflow:

```yaml
actions:
  sendVerificationEmail:
    type: email
    config:
      to: '{{ param "email" }}'
      subject: "Verify your account"
      template: verification_email
    next: $response.registrationPending
```

## Common Issues

### "Integration not found"
- Verify the `integrationID` matches your configured integration
- Check that integrations are loaded from the integrations.yaml file

### "Database connection failed"
- Verify your MongoDB connection string is correct
- Ensure the database server is running and accessible

### "JWT secret not found"
- Set the `JWT_SECRET` environment variable
- Use a strong, randomly generated secret key

## Next Steps

After getting this example working, consider:

1. **Building a Login Endpoint** - Authenticate existing users
2. **Adding Email Verification** - Verify user email addresses
3. **Password Reset Flow** - Allow users to reset forgotten passwords
4. **User Profile Management** - Update user information
5. **Role-Based Access Control** - Add user roles and permissions

## Learn More

- [ServFlow Documentation](https://docs.servflow.io)
- [API Configuration Guide](https://docs.servflow.io/concepts)
- [Available Actions Reference](https://docs.servflow.io/concepts/actions)
- [Integration Setup Guide](https://docs.servflow.io/concepts/integrations)