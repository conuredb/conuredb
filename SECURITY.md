# Security Policy

## Supported Versions

We provide security updates for the following versions of ConureDB:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability in ConureDB, please follow these steps:

### Private Disclosure

**Do not create a public GitHub issue for security vulnerabilities.**

Instead, please email security findings to: **`hi@neelanjan.dev`**

Include the following information:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact assessment
- Any suggested fixes or mitigations

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your report within 48 hours
- **Initial Assessment**: We will provide an initial assessment within 5 business days
- **Updates**: We will keep you informed of our progress throughout the process
- **Resolution**: We aim to resolve critical vulnerabilities within 30 days

### Disclosure Timeline

1. **Day 0**: Vulnerability reported privately
2. **Day 1-2**: Acknowledgment sent
3. **Day 3-7**: Initial assessment and validation
4. **Day 7-30**: Development and testing of fix
5. **Day 30+**: Public disclosure after fix is released

## Security Best Practices

### Deployment Security

- **Network Security**: Run ConureDB behind a firewall, restrict access to necessary ports only
- **Authentication**: Implement network-level authentication (VPN, mutual TLS, etc.)
- **Encryption**: Use TLS for all network communication in production
- **Access Control**: Limit access to ConureDB HTTP API and data directories

### Kubernetes Security

- **RBAC**: Use proper Kubernetes RBAC for service accounts
- **Network Policies**: Implement network policies to restrict pod-to-pod communication
- **Pod Security**: Run containers as non-root users (enabled by default in our charts)
- **Secrets Management**: Use Kubernetes secrets for sensitive configuration

### Data Security

- **Encryption at Rest**: Consider encrypting persistent volumes
- **Backup Security**: Secure backup files and transmission
- **Data Access**: Monitor and audit data access patterns
- **Key Management**: Implement proper key rotation for any encryption keys

## Known Security Considerations

### Current Limitations

1. **No Built-in Authentication**: ConureDB currently relies on network-level security
2. **No Encryption at Rest**: Data is stored unencrypted on disk
3. **No Audit Logging**: No built-in audit trail for data access

### Planned Security Features

- Built-in authentication and authorization
- Encryption at rest support
- Audit logging capabilities
- TLS support for Raft communication

## Vulnerability Response

### Classification

We classify vulnerabilities using the following severity levels:

- **Critical**: Remote code execution, data corruption, authentication bypass
- **High**: Information disclosure, privilege escalation, DoS attacks
- **Medium**: Limited information disclosure, minor privilege issues
- **Low**: Limited impact on security posture

### Response Times

- **Critical**: 24-48 hours acknowledgment, 7-14 days for fix
- **High**: 48-72 hours acknowledgment, 14-30 days for fix
- **Medium**: 3-5 days acknowledgment, 30-60 days for fix
- **Low**: 5-7 days acknowledgment, 60-90 days for fix

## Security Updates

Security updates will be:

- Released as patch versions (e.g., 1.0.1)
- Announced in release notes with severity classification
- Communicated through GitHub security advisories
- Documented with migration instructions if needed

## Community Security

### Bug Bounty

We currently do not have a formal bug bounty program, but we:

- Acknowledge security researchers in release notes
- Provide detailed feedback on reported vulnerabilities
- Consider implementing bounty programs as the project grows

### Security Research

We welcome security research on ConureDB. Please

- Test only against your own instances
- Respect user privacy and data
- Follow responsible disclosure practices
- Do not attempt to access production systems

## Contact Information

- **Security Email**: `hi@neelanjan.dev`
- **General Issues**: [GitHub Issues](https://github.com/conuredb/conuredb/issues)
- **Project Maintainers**: [GitHub Team](https://github.com/orgs/conure-db/teams)

---

Thank you for helping keep ConureDB and our community safe!
