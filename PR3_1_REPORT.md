# PR3.1 — Scenes Capability & Validation Hardening

**Branch**: `scenes-pr3-1-agent-hardening`  
**Repos**: heimdex-agent (primary), heimdex-media-pipelines (doctor change only)  
**Base**: PR3 (`scenes-pr3-agent-scenes`)  
**Stats**: 10 files changed, 923 insertions, 52 deletions (across both repos)

---

## Changes by Goal

### G1 — Capability detection sourced from doctor JSON

| File | Change |
|------|--------|
| `heimdex-media-pipelines/src/.../cli.py` | Doctor now emits `pipelines: {speech, faces, scenes}` booleans |
| `internal/pipelines/types.go` | Added `PipelinesInfo` struct, `Pipelines` field on `Capabilities` |
| `internal/pipelines/runner.go` | Extracted `DeriveCapabilities()`. RunDoctor reads `caps.Pipelines`; falls back to legacy inference if zero-value |
| `internal/pipelines/runner_test.go` | 3 new tests: `TestParseDoctorJSON_WithPipelines`, `_LegacyFallback`, `_ScenesWithoutSpeech` |

**Why**: Previously `HasScenes = ffmpeg available`, which is too weak. Now derived from Python-side introspection that checks actual pipeline availability.

### G2 — Scene output payload validation

| File | Change |
|------|--------|
| `internal/pipelines/types.go` | Added `SceneOutputPayload`, `SceneBoundary` types |
| `internal/pipelines/runner.go` | Added `ValidateSceneOutput()` to Runner interface + impl. Checks: meta fields, video_id, scenes array presence, per-scene timestamps (non-negative, end > start), scene_id regex `^.+_scene_\d+$`, monotonic ordering |
| `internal/catalog/runner.go` | Scenes step now calls `ValidateSceneOutput` instead of `ValidateOutput` |
| `internal/pipelines/runner_test.go` | 8 new tests: valid, empty scenes, missing video_id, negative timestamp, end < start, non-monotonic, invalid scene_id, missing meta fields |

**Why**: `ValidateOutput` only checked 3 meta fields. Scene output could be garbage (wrong timestamps, missing video_id) and still pass. Now payload-level invariants are enforced.

### G3 — Per-step cancellation / no goroutine leaks

| File | Change |
|------|--------|
| `internal/catalog/runner.go` | `parallelCtx, parallelCancel` wraps faces+scenes goroutines. On first error: cancel context + drain channel (no early return = no leak) |
| `internal/catalog/runner_test.go` | 2 new tests: `TestProcessIndexJob_CancelledContext`, `_FacesFailsScenesStillDrains` |

**Why**: Previously, early return on first parallel error left the other goroutine running until its subprocess timed out. Now the cancel propagates immediately via context.

### G4 — Non-blocking /status with scenes clarity

| File | Change |
|------|--------|
| `internal/pipelines/doctor.go` | Added `Peek()` — read-only, no-probe cache read |
| `internal/api/routes.go` | `statusHandler` uses `Peek()` instead of `Get()` (non-blocking) |
| `internal/api/schemas.go` | `PipelineStatusResponse` adds `has_scenes`. New `ConstraintsResponse{ScenesRequiresSpeech: true}`. `StatusResponse` adds `constraints` |
| `internal/api/routes_test.go` | 5 new tests: nil doctor, empty cache, cached caps, scenes_requires_speech, zero probed_at |

**Why**: `Get()` could trigger a subprocess probe inside the HTTP request path, causing latency spikes. `Peek()` is always instant. The `constraints` field makes the speech→scenes dependency explicit for API consumers.

---

## Verification Commands

### heimdex-agent
```bash
cd heimdex-agent
go vet ./...
go test -v -race ./...
```

### heimdex-media-pipelines
```bash
docker run --rm \
  -v $(pwd)/../heimdex-media-contracts:/contracts \
  -v $(pwd):/app -w /app python:3.13-slim bash -c "
    pip install --no-cache-dir /contracts opencv-python-headless typer pytest &&
    pip install --no-cache-dir --no-deps -e . &&
    python -m pytest tests/ -v --tb=short
  "
```

### Doctor smoke test
```bash
python -m heimdex_media_pipelines doctor --json | python -c \
  'import json,sys; d=json.load(sys.stdin); print(d["pipelines"])'
```

---

## Test Summary

| Package | Tests | New | Status |
|---------|-------|-----|--------|
| `internal/pipelines` | 28 | +11 | ✅ PASS |
| `internal/catalog` | 15 | +2 | ✅ PASS |
| `internal/api` | 5 | +5 | ✅ PASS |
| `internal/db` | 4 | 0 | ✅ PASS |
| `internal/playback` | 3 | 0 | ✅ PASS |
| **Total Go** | **55** | **+18** | ✅ |
| Python pipelines | 54 | 0 | ✅ PASS |

All tests pass with `-race` flag.

---

## Backward Compatibility

- **Doctor JSON**: New `pipelines` field is additive. Old consumers ignore it. Agent falls back to legacy inference if field is absent.
- **Runner interface**: Added `ValidateSceneOutput()` — breaking for custom implementations, but only `SubprocessRunner` and test fakes implement it.
- **Status API**: `pipelines.has_scenes` and `constraints` are new additive fields. Existing consumers ignore them.
- **Behavior when scenes disabled**: Identical to pre-PR3.1. HasScenes=false → scenes step skipped, no goroutines launched.

---

## Tradeoffs

1. **ValidateSceneOutput is a separate method** (not a parameter on ValidateOutput). This avoids changing the existing interface contract for speech/faces validation, at the cost of a second interface method. Preferred because scene validation has fundamentally different invariants.

2. **Peek() returns stale data or nil**. The status handler doesn't show capabilities until the first doctor probe runs (triggered by the first index job). This is acceptable — probing on startup or on-demand can be added later.

3. **Scene regex validation in Go is duplicated from Python contracts**. The `^.+_scene_\d+$` pattern exists in both. This is intentional — the agent should validate independently of the Python side as a defense-in-depth measure.
