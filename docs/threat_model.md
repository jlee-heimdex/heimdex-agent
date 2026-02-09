# Heimdex Agent Threat Model

## Overview

This document outlines the security model for Heimdex Agent, a localhost-only desktop application that indexes and streams video files.

## Security Principles

1. **Localhost Only**: Never bind to external interfaces
2. **Authentication Required**: All sensitive endpoints require Bearer token
3. **No Secrets in Logs**: Tokens are sanitized before logging
4. **Minimal Permissions**: Request only necessary file system access
5. **Defense in Depth**: Multiple layers of protection

## Attack Surface

### Network

| Component | Binding | Risk | Mitigation |
|-----------|---------|------|------------|
| HTTP API | 127.0.0.1:8787 | Local process access | Bearer token auth |
| Database | File-based | File system access | User permissions |

### Attack Vectors

#### 1. Local Process Access
**Threat**: Other applications on the same machine can access the API.

**Mitigations**:
- Bearer token authentication required
- Token stored securely in SQLite (file permissions)
- Token is 64 characters of cryptographic randomness

#### 2. Token Theft
**Threat**: Attacker obtains the auth token.

**Mitigations**:
- Token only stored in database file
- Database has user-only permissions (0600)
- Token is never logged (sanitized)
- Token displayed only at startup

#### 3. Path Traversal
**Threat**: Attacker tries to access files outside allowed sources.

**Mitigations**:
- Playback only serves files in the catalog
- File paths validated against source paths
- No direct path input to playback endpoint (file_id only)

#### 4. File System Access
**Threat**: Attacker adds malicious source paths.

**Mitigations**:
- AddFolder validates path exists and is directory
- Bearer token required for AddFolder
- Sources tracked in database

## Data at Rest

| Data | Location | Protection |
|------|----------|------------|
| Database | ~/.heimdex/heimdex.db | File permissions (0600) |
| Auth Token | In database | Same as database |
| Video Files | User-specified paths | User file permissions |

## Data in Transit

| Channel | Protection |
|---------|------------|
| localhost HTTP | Plaintext (acceptable for localhost) |
| Cloud sync (future) | TLS required |

## Authentication Flow

```
1. First Run:
   - Generate 64-byte random token
   - Store in database config table
   - Display to user

2. API Request:
   - Client sends: Authorization: Bearer <token>
   - Server retrieves stored token
   - Constant-time comparison
   - Reject with 401 if mismatch
```

## Logging Policy

### Logged
- Request method and path
- Response status code
- Request duration
- Job IDs and status
- File operations (path sanitized)

### Never Logged
- Full authentication tokens
- Full file paths (home dir replaced with ~)
- File contents

## Incident Response

### Token Compromise
1. Delete database file
2. Restart agent (new token generated)
3. Update all clients with new token

### Malicious Source Added
1. Remove source via API: DELETE /sources/{id}
2. This also removes all indexed files for that source

## Future Security Enhancements

1. **Token Rotation**: Periodic token regeneration
2. **Rate Limiting**: Prevent brute force attempts
3. **Audit Logging**: Track all sensitive operations
4. **Encryption at Rest**: Encrypt database file
5. **mTLS**: For cloud communication
