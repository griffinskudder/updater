# Secure Deployment Guide

## Overview

This guide provides comprehensive instructions for deploying the updater service securely across different environments. It covers configuration, best practices, and security considerations for production deployments.

## Quick Start Security Checklist

Before deploying to production, ensure you have completed these critical security steps:

- [ ] **TLS/HTTPS enabled** with valid certificates
- [ ] **API keys generated** with cryptographically secure random values (32+ characters)
- [ ] **Environment variables configured** for all sensitive values
- [ ] **Rate limiting enabled** with appropriate limits for your use case
- [ ] **CORS configured** with specific allowed origins (no wildcards in production)
- [ ] **Monitoring and logging** configured for security events
- [ ] **Regular key rotation** schedule established
- [ ] **Backup recovery plan** tested and documented

## Environment Configurations

### Development Environment

For local development and testing:

```yaml
# Use the development configuration
cp configs/security-examples.yaml config.yaml

# Set environment variables
export UPDATER_CONFIG_SECTION="development"
export UPDATER_LOG_LEVEL="debug"

# Generate development API keys
openssl rand -base64 32  # Use output for dev-admin-key
openssl rand -base64 32  # Use output for dev-read-key
```

**Security Notes for Development:**
- Use strong API keys even in development
- Enable authentication to test security flows
- Use lenient rate limiting for testing
- TLS can be disabled for localhost testing

### Staging Environment

For staging/pre-production testing:

```bash
# Set environment variables
export UPDATER_CONFIG_SECTION="staging"
export STAGING_ADMIN_API_KEY="$(openssl rand -base64 32)"
export STAGING_RELEASE_API_KEY="$(openssl rand -base64 32)"
export STAGING_READONLY_API_KEY="$(openssl rand -base64 32)"

# Configure TLS certificates
sudo mkdir -p /etc/ssl/certs /etc/ssl/private
sudo cp staging.crt /etc/ssl/certs/staging.pem
sudo cp staging.key /etc/ssl/private/staging.key
sudo chmod 644 /etc/ssl/certs/staging.pem
sudo chmod 600 /etc/ssl/private/staging.key
```

### Production Environment

For production deployments:

```bash
# Set environment variables (use your secrets management system)
export UPDATER_CONFIG_SECTION="production"
export PRODUCTION_ADMIN_API_KEY="YOUR_SECURE_ADMIN_KEY"
export PRODUCTION_RELEASE_API_KEY="YOUR_SECURE_RELEASE_KEY"
export PRODUCTION_MONITORING_API_KEY="YOUR_SECURE_MONITORING_KEY"
export DATABASE_URL="your_database_connection_string"

# Configure production TLS certificates
sudo cp production.crt /etc/ssl/certs/production.pem
sudo cp production.key /etc/ssl/private/production.key
sudo chmod 644 /etc/ssl/certs/production.pem
sudo chmod 600 /etc/ssl/private/production.key
```

## API Key Management

### Generating Secure API Keys

**Recommended Method:**
```bash
# Generate a 64-character (512-bit) API key
openssl rand -base64 48 | tr -d '\n'; echo

# Alternative using system random
head -c 32 /dev/urandom | base64 | tr -d '\n'; echo
```

**Key Requirements:**
- Minimum 32 characters (256 bits of entropy)
- Use cryptographically secure random generation
- Avoid predictable patterns or dictionary words
- Store in secure environment variables or secrets management

### Key Rotation Strategy

**Recommended Rotation Schedule:**
- **Production:** Every 90 days
- **Staging:** Every 6 months
- **Development:** As needed

**Zero-Downtime Rotation Process:**
1. Generate new API key
2. Add new key to configuration (keeping old key enabled)
3. Update client applications to use new key
4. Verify new key works in logs
5. Disable old key in configuration
6. Remove old key after grace period

**Example Rotation Configuration:**
```yaml
api_keys:
  # New key
  - key: "${NEW_ADMIN_API_KEY}"
    name: "Admin Key v2"
    permissions: ["admin"]
    enabled: true

  # Old key (grace period)
  - key: "${OLD_ADMIN_API_KEY}"
    name: "Admin Key v1 (deprecated)"
    permissions: ["admin"]
    enabled: true  # Disable after client migration
```

## TLS/HTTPS Configuration

### Certificate Requirements

**Production Requirements:**
- Valid SSL certificate from trusted CA
- Support for modern TLS versions (1.2+)
- Strong cipher suites
- HSTS headers configured

**Certificate Sources:**
- Let's Encrypt (free, automated)
- Commercial CA (extended validation)
- Internal CA (for private networks)

### Let's Encrypt Setup

```bash
# Install certbot
sudo apt-get install certbot

# Generate certificate
sudo certbot certonly --standalone -d api.yourdomain.com

# Configure automatic renewal
sudo crontab -e
# Add: 0 12 * * * /usr/bin/certbot renew --quiet

# Update configuration paths
export TLS_CERT_FILE="/etc/letsencrypt/live/api.yourdomain.com/fullchain.pem"
export TLS_KEY_FILE="/etc/letsencrypt/live/api.yourdomain.com/privkey.pem"
```

### Load Balancer TLS Termination

When using a load balancer (recommended for production):

```yaml
# Application configuration (TLS terminated at load balancer)
server:
  tls_enabled: false  # TLS handled by load balancer
  port: 8080

security:
  # Configure trusted proxy networks
  trusted_proxies:
    - "10.0.0.0/8"      # Load balancer subnet
    - "172.16.0.0/12"   # Internal networks
```

## Cloud Deployment Examples

### AWS Deployment

**ECS Configuration:**
```json
{
  "family": "updater-service",
  "taskDefinition": {
    "secrets": [
      {
        "name": "PRODUCTION_ADMIN_API_KEY",
        "valueFrom": "arn:aws:secretsmanager:region:account:secret:updater/admin-key"
      },
      {
        "name": "DATABASE_URL",
        "valueFrom": "arn:aws:secretsmanager:region:account:secret:updater/db-connection"
      }
    ],
    "environment": [
      {
        "name": "UPDATER_CONFIG_SECTION",
        "value": "aws_production"
      }
    ]
  }
}
```

**Application Load Balancer Security Groups:**
```yaml
# ALB Security Group
Type: AWS::EC2::SecurityGroup
Properties:
  GroupDescription: Security group for updater ALB
  SecurityGroupIngress:
    - IpProtocol: tcp
      FromPort: 443
      ToPort: 443
      CidrIp: 0.0.0.0/0  # Restrict to your IP ranges
    - IpProtocol: tcp
      FromPort: 80
      ToPort: 80
      CidrIp: 0.0.0.0/0  # For HTTP -> HTTPS redirect
```

### Google Cloud Deployment

**Cloud Run Configuration:**
```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: updater-service
  annotations:
    run.googleapis.com/ingress: all
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/maxScale: "100"
    spec:
      containerConcurrency: 1000
      containers:
      - image: gcr.io/PROJECT/updater:latest
        env:
        - name: UPDATER_CONFIG_SECTION
          value: "gcp_production"
        - name: GCP_ADMIN_API_KEY
          valueFrom:
            secretKeyRef:
              name: updater-secrets
              key: admin-api-key
```

### Kubernetes Deployment

**Deployment with Secrets:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: updater-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: updater-service
  template:
    metadata:
      labels:
        app: updater-service
    spec:
      containers:
      - name: updater
        image: updater:latest
        ports:
        - containerPort: 8080
        env:
        - name: UPDATER_CONFIG_SECTION
          value: "k8s_production"
        - name: K8S_ADMIN_API_KEY
          valueFrom:
            secretKeyRef:
              name: updater-secrets
              key: admin-api-key
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: updater-secrets
              key: database-url
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Secret
metadata:
  name: updater-secrets
type: Opaque
stringData:
  admin-api-key: "YOUR_SECURE_ADMIN_KEY_HERE"
  database-url: "your_database_connection_string"
---
apiVersion: v1
kind: Service
metadata:
  name: updater-service
spec:
  selector:
    app: updater-service
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: updater-ingress
  annotations:
    kubernetes.io/ingress.class: "nginx"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - api.yourdomain.com
    secretName: updater-tls
  rules:
  - host: api.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: updater-service
            port:
              number: 80
```

## Monitoring and Alerting

### Security Monitoring Setup

**Key Security Metrics to Monitor:**
- Authentication failure rate
- Permission denied attempts
- Rate limit violations
- Unusual API usage patterns
- Admin operations outside business hours

**Example Monitoring Configuration:**
```yaml
# Prometheus metrics (implement in application)
- name: auth_failures_total
  help: Total authentication failures
  type: counter
  labels: [endpoint, client_ip]

- name: permission_denials_total
  help: Total permission denials
  type: counter
  labels: [endpoint, required_permission, api_key_name]

- name: rate_limit_violations_total
  help: Total rate limit violations
  type: counter
  labels: [client_ip]
```

**Alert Rules:**
```yaml
# Example Prometheus alert rules
groups:
- name: updater_security_alerts
  rules:
  - alert: HighAuthFailureRate
    expr: rate(auth_failures_total[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High authentication failure rate detected"

  - alert: AdminOperationOutsideBusinessHours
    expr: increase(admin_operations_total[1h]) > 0 unless (hour() >= 9 and hour() <= 17)
    labels:
      severity: critical
    annotations:
      summary: "Admin operation detected outside business hours"
```

### Log Analysis

**Security Events to Log:**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "WARN",
  "event": "permission_denied",
  "message": "Insufficient permissions for endpoint",
  "client_ip": "192.168.1.100",
  "api_key_name": "Read Only Key",
  "endpoint": "/api/v1/updates/myapp/register",
  "required_permission": "write",
  "user_permissions": ["read"],
  "request_id": "req_123456"
}
```

## Security Testing

### Automated Security Testing

Create a security test script to validate your deployment:

```bash
#!/bin/bash
# security-validation.sh

SERVER_URL="https://your-api.domain.com"
VALID_KEY="your-test-api-key"

echo "🔒 Security Validation Test"
echo "=========================="

# Test 1: HTTPS enforcement
echo "Testing HTTPS enforcement..."
http_response=$(curl -s -w "%{http_code}" -o /dev/null "http://your-api.domain.com/health")
if [[ $http_response -eq 301 ]] || [[ $http_response -eq 302 ]]; then
    echo "✅ HTTPS redirect working"
else
    echo "❌ HTTPS redirect not working: $http_response"
fi

# Test 2: Authentication required
echo "Testing authentication requirements..."
no_auth_response=$(curl -s -w "%{http_code}" -o /dev/null "$SERVER_URL/api/v1/updates/test/register" -X POST)
if [[ $no_auth_response -eq 401 ]]; then
    echo "✅ Authentication required"
else
    echo "❌ Authentication not required: $no_auth_response"
fi

# Test 3: Valid API key works
echo "Testing valid API key..."
valid_response=$(curl -s -w "%{http_code}" -o /dev/null "$SERVER_URL/health" -H "Authorization: Bearer $VALID_KEY")
if [[ $valid_response -eq 200 ]]; then
    echo "✅ Valid API key accepted"
else
    echo "❌ Valid API key rejected: $valid_response"
fi

# Test 4: Invalid API key rejected
echo "Testing invalid API key rejection..."
invalid_response=$(curl -s -w "%{http_code}" -o /dev/null "$SERVER_URL/api/v1/updates/test/register" -X POST -H "Authorization: Bearer invalid-key")
if [[ $invalid_response -eq 401 ]]; then
    echo "✅ Invalid API key rejected"
else
    echo "❌ Invalid API key not rejected: $invalid_response"
fi

echo ""
echo "Security validation complete!"
```

### Penetration Testing Checklist

**Manual Security Testing:**
- [ ] Test SQL injection attempts on all parameters
- [ ] Test path traversal attempts in URL parameters
- [ ] Test header injection attacks
- [ ] Test large payload handling
- [ ] Test rate limiting effectiveness
- [ ] Test CORS policy enforcement
- [ ] Test TLS configuration and cipher suites
- [ ] Test API key brute force protection

## Incident Response

### Security Incident Response Plan

**Immediate Response (within 15 minutes):**
1. Identify compromised API keys from logs
2. Disable compromised keys immediately
3. Check for unauthorized data access
4. Document incident timeline

**Short-term Response (within 1 hour):**
1. Generate new API keys for affected systems
2. Update client applications with new keys
3. Review all recent admin operations
4. Increase monitoring and logging

**Long-term Response (within 24 hours):**
1. Conduct full security audit
2. Update security procedures
3. Implement additional monitoring
4. Review and update incident response plan

### Emergency Key Revocation

**Quick Key Disable:**
```bash
# Method 1: Environment variable
export EMERGENCY_DISABLE_KEY="compromised-key-value"

# Method 2: Configuration update
kubectl patch configmap updater-config \
  -p '{"data":{"security.api_keys[0].enabled":"false"}}'

# Method 3: Service restart with new config
docker stop updater-service
# Update configuration file
docker start updater-service
```

## Compliance Considerations

### GDPR Compliance

**Data Minimization:**
- Log only necessary operational data
- Avoid logging personally identifiable information
- Implement log retention policies
- Provide data deletion capabilities

### SOC 2 Compliance

**Access Controls:**
- Implement role-based access control
- Maintain access logs and audit trails
- Regular access reviews and key rotation
- Secure key storage and management

### HIPAA Compliance (if handling health data)

**Additional Requirements:**
- End-to-end encryption
- Business Associate Agreements (BAAs)
- Enhanced access logging
- Regular risk assessments

## Security Maintenance

### Regular Security Tasks

**Weekly:**
- Review security logs for anomalies
- Monitor rate limiting effectiveness
- Check certificate expiration dates

**Monthly:**
- Rotate development/staging API keys
- Review and update CORS policies
- Update dependencies and security patches

**Quarterly:**
- Rotate production API keys
- Conduct security assessment
- Review and update security documentation
- Test incident response procedures

### Security Updates

**Staying Current:**
- Subscribe to Go security announcements
- Monitor dependency vulnerabilities
- Follow security best practices updates
- Regular security training for team members

**Update Process:**
1. Test security updates in staging
2. Schedule maintenance windows
3. Apply updates with rollback plan
4. Validate security posture post-update
5. Document changes and lessons learned

This comprehensive deployment guide ensures that your updater service is deployed securely and maintains strong security posture throughout its lifecycle.