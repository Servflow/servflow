# Security Policy

## Supported Versions

We provide security updates for the following versions of Servflow:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

The Servflow team takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose your findings.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report security vulnerabilities by emailing us at:

📧 **Contact@servflow.com**

### What to Include

Please include the following information in your report:

- **Description**: A clear description of the vulnerability
- **Impact**: What could an attacker accomplish by exploiting this vulnerability?
- **Reproduction Steps**: Step-by-step instructions to reproduce the issue
- **Environment**: Servflow version, installation method, OS, etc.
- **Supporting Material**: Screenshots, logs, or proof-of-concept code (if applicable)

### What to Expect

After you submit a report, here's what happens:

1. **Acknowledgment** (within 48 hours): We'll acknowledge receipt of your report
2. **Initial Assessment** (within 5 business days): We'll provide an initial assessment of the report
3. **Investigation** (timeline varies): We'll investigate and work on a fix
4. **Resolution** (timeline varies): We'll release a security update and coordinate disclosure

### Disclosure Timeline

- We aim to resolve critical vulnerabilities within 90 days
- We'll work with you to coordinate public disclosure
- We may request additional time for complex issues
- We'll credit you in the security advisory (unless you prefer to remain anonymous)

### Scope

This security policy applies to:

- **Servflow Core**: The main binary and server components
- **Web Interface**: The browser-based user interface
- **API Endpoints**: All REST API endpoints
- **Docker Images**: Official Docker containers
- **Installation Scripts**: Official installation and setup scripts

### Out of Scope

The following are generally considered out of scope:

- Issues in third-party dependencies (please report to the respective maintainers)
- Social engineering attacks
- Physical attacks
- Denial of service attacks requiring excessive resources
- Issues requiring physical access to the server

### Bounty Program

While we don't currently have a formal bug bounty program, we recognize and appreciate security researchers who help us improve Servflow's security:

- **Recognition**: Credit in security advisories and release notes
- **Swag**: Servflow merchandise for significant findings
- **Direct Communication**: Direct line to our security team for future reports

### Security Best Practices

To help keep your Servflow installation secure:

#### For Administrators

- **Keep Updated**: Always run the latest version of Servflow
- **Network Security**: Use firewalls and network segmentation
- **Access Control**: Implement proper authentication and authorization
- **Monitoring**: Enable logging and monitoring for security events
- **Backups**: Maintain secure, tested backups of your data

#### For Developers

- **Secure Configuration**: Follow security configuration guides
- **Input Validation**: Validate all user inputs in custom actions
- **Secrets Management**: Use proper secret management for API keys and credentials
- **HTTPS**: Always use HTTPS in production
- **Regular Audits**: Regularly review your workflows and permissions

### Security Updates

Security updates are delivered through:

- **GitHub Releases**: Security patches are released as new versions
- **Security Advisories**: Published on GitHub Security Advisories
- **Documentation Updates**: Security guides are updated as needed
- **Notifications**: Critical updates may be announced via multiple channels

### Contact Information

For security-related questions or concerns:

- **Email**: security@servflow.com
- **Response Time**: We aim to respond within 48 hours
- **PGP Key**: Available upon request for sensitive communications

### Legal

- We will not pursue legal action against researchers who report vulnerabilities in good faith
- We ask that you don't access or modify data that doesn't belong to you
- Please don't perform testing that could harm our services or other users

## Security Features

Servflow includes several built-in security features:

### Authentication & Authorization

- JWT-based authentication
- Role-based access control (RBAC)
- API key management
- Session management

### Data Protection

- Encryption at rest (configurable)
- Encryption in transit (TLS/HTTPS)
- Secure credential storage
- Input sanitization

### Network Security

- CORS configuration
- Rate limiting
- Request validation
- Secure headers

### Monitoring & Auditing

- Access logging
- Audit trails for sensitive operations
- Security event monitoring
- Trace ID tracking

---

Thank you for helping keep Servflow and our community safe! 🔒
