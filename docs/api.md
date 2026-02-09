# Heimdex Agent API Reference

## Base URL

```
http://127.0.0.1:8787
```

## Authentication

All endpoints except `/health` require Bearer token authentication.

```
Authorization: Bearer <token>
```

The token is displayed at agent startup and stored in the database.

---

## Endpoints

### GET /health

Health check endpoint (no authentication required).

**Response**

```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime_s": 3600,
  "device_id": "abc123..."
}
```

---

### GET /status

Get current agent status.

**Response**

```json
{
  "state": "idle",
  "last_error": "",
  "sources_count": 2,
  "files_count": 150,
  "jobs_running": 0,
  "active_job": null
}
```

**States**
- `idle`: No active jobs
- `indexing`: Scan job running
- `paused`: Job runner paused
- `error`: Last job failed

---

### GET /sources

List all configured sources.

**Response**

```json
{
  "sources": [
    {
      "id": "abc123-def456-...",
      "type": "folder",
      "path": "/Users/name/Videos",
      "display_name": "My Videos",
      "drive_nickname": "",
      "present": true,
      "created_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

---

### POST /sources/folders

Add a folder source.

**Request**

```json
{
  "path": "/path/to/folder",
  "display_name": "Optional Display Name"
}
```

**Response**

```json
{
  "source_id": "abc123-def456-..."
}
```

**Errors**
- `400 BAD_REQUEST`: Path doesn't exist or isn't a directory

---

### DELETE /sources/{id}

Remove a source and all its indexed files.

**Response**

`204 No Content`

---

### GET /sources/{id}/files

List files for a specific source.

**Response**

```json
{
  "files": [
    {
      "id": "file-123-...",
      "source_id": "source-123-...",
      "path": "/Users/name/Videos/movie.mp4",
      "filename": "movie.mp4",
      "size": 1073741824,
      "fingerprint": "sha256-...",
      "created_at": "2024-01-15T11:00:00Z"
    }
  ]
}
```

---

### POST /scan

Start a scan job.

**Request**

```json
{
  "source_id": "abc123-..."
}
```

If `source_id` is omitted, scans the first source.

**Response**

```json
{
  "job_id": "job-123-..."
}
```

---

### GET /jobs

List recent jobs.

**Response**

```json
{
  "jobs": [
    {
      "id": "job-123-...",
      "type": "scan",
      "status": "completed",
      "source_id": "source-123-...",
      "progress": 100,
      "error": "",
      "created_at": "2024-01-15T11:00:00Z",
      "updated_at": "2024-01-15T11:05:00Z"
    }
  ]
}
```

**Status Values**
- `pending`: Waiting to run
- `running`: Currently executing
- `completed`: Finished successfully
- `failed`: Finished with error

---

### GET /jobs/{id}

Get a specific job.

**Response**

```json
{
  "id": "job-123-...",
  "type": "scan",
  "status": "running",
  "source_id": "source-123-...",
  "progress": 45,
  "error": "",
  "created_at": "2024-01-15T11:00:00Z",
  "updated_at": "2024-01-15T11:02:00Z"
}
```

---

### GET /playback/file

Stream a video file with HTTP Range support.

**Query Parameters**
- `file_id` (required): File ID from catalog
- `t` (optional): Start time in seconds (not implemented in v0)

**Headers**
- `Range: bytes=0-1023` (optional): Request specific byte range

**Response**

For range requests:
- Status: `206 Partial Content`
- Headers:
  - `Accept-Ranges: bytes`
  - `Content-Range: bytes 0-1023/1073741824`
  - `Content-Length: 1024`
  - `Content-Type: video/mp4`

For full file:
- Status: `200 OK`
- Headers:
  - `Accept-Ranges: bytes`
  - `Content-Length: 1073741824`
  - `Content-Type: video/mp4`

**Errors**
- `404 NOT_FOUND`: File not in catalog
- `404 DRIVE_DISCONNECTED`: Source drive is not connected
- `416 Range Not Satisfiable`: Invalid range header

---

## Error Response Format

All errors return a consistent format:

```json
{
  "error": "Human readable message",
  "code": "ERROR_CODE"
}
```

**Common Error Codes**
- `UNAUTHORIZED`: Missing or invalid token
- `BAD_REQUEST`: Invalid request parameters
- `NOT_FOUND`: Resource not found
- `INTERNAL_ERROR`: Server error
- `DRIVE_DISCONNECTED`: Source drive not available

---

## Example Usage

```bash
# Get token from startup output
TOKEN="your-64-char-token"

# Health check
curl http://127.0.0.1:8787/health

# Check status
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8787/status

# Add folder
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path": "/Users/me/Videos", "display_name": "My Videos"}' \
  http://127.0.0.1:8787/sources/folders

# Start scan
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"source_id": "abc123..."}' \
  http://127.0.0.1:8787/scan

# Check job status
curl -H "Authorization: Bearer $TOKEN" \
  http://127.0.0.1:8787/jobs

# Stream video with Range
curl -H "Authorization: Bearer $TOKEN" \
  -H "Range: bytes=0-1023" \
  "http://127.0.0.1:8787/playback/file?file_id=file123..."
```
