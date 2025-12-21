# Memory Leak Fixes - December 2024

## Issue
Memory usage was steadily increasing over time:
- `Sys` memory grew from 1203MB → 1492MB → 1605MB in one hour (400MB leak)
- Indicates memory allocated but not released back to OS

## Root Causes Identified

### 1. **Goroutine Leak in StartPeriodicIndexing**
**Problem:** The periodic indexing goroutine wasn't properly exiting after data fetch completed. It would wait on the ticker indefinitely.

**Fix:** Added proper exit check before each ticker wait:
- Check `fetchInProgress` status before waiting
- Exit immediately if fetch is complete
- Added defer to log goroutine exit

**File:** `orchestrator.go` - `StartPeriodicIndexing()`

### 2. **Scheduler Goroutine Not Cleaned Up**
**Problem:** Scheduler goroutine and timer continued running even after orchestrator cleanup, preventing garbage collection.

**Fix:**
- Added `scheduler` field to Orchestrator struct to track scheduler instance
- Modified `Stop()` method to use channel close instead of send (prevents panic)
- Added `stopped` flag to prevent double-close
- Call `scheduler.Stop()` in `Cleanup()` method

**Files:** 
- `orchestrator.go` - Added scheduler field and cleanup
- `internal/scheduler.go` - Fixed Stop() method

### 3. **HTTP Connection Leaks**
**Problem:** HTTP Transport connections weren't being closed, leading to connection leaks and memory retention.

**Fix:**
- Added `transport` field to `APIClient` struct
- Reduced `MaxIdleConns` from 10 to 5
- Reduced `IdleConnTimeout` from 90s to 30s
- Added `Close()` method to `APIClient` to close idle connections
- Added `defer apiClient.Close()` in all functions that create APIClient:
  - `LoadAllTrackDataWithCallback()`
  - `PerformIncrementalRefresh()`

**Files:**
- `internal/api.go` - Added Close() method
- `internal/loader.go` - Added defer Close()
- `internal/refresh.go` - Added defer Close()

### 4. **Inefficient Garbage Collection**
**Problem:** Default GOGC=100 was too lenient, allowing memory to accumulate between GC cycles.

**Fix:**
- Set `GOGC=50` for more aggressive garbage collection (2x more frequent)
- Added periodic memory monitoring every 15 minutes that:
  - Forces `runtime.GC()`
  - Logs memory stats for monitoring
- Properly stops on context cancellation

**File:** `main.go` - Added GC tuning and periodicMemoryMonitoring()

## Expected Results

After these fixes, the application should:

1. **Stable Memory Usage**: `Sys` memory should stabilize and not continuously grow
2. **Better GC**: More frequent garbage collection prevents memory accumulation
3. **No Goroutine Leaks**: All goroutines properly exit when no longer needed
4. **No Connection Leaks**: HTTP connections properly cleaned up after use

## Monitoring

Watch for these metrics in logs:
```
💾 Periodic GC: Alloc=XXXMB, Sys=XXXMB, NumGC=XXX
```

Expected behavior:
- `Alloc` should stay around 1000-1100MB (heap in use)
- `Sys` should stabilize around 1200-1400MB (not grow indefinitely)
- `NumGC` should increase regularly (GC is running)

## Testing Recommendations

1. **Monitor for 2-3 hours** to ensure Sys memory doesn't continuously grow
2. **Watch for goroutine exits** in logs:
   - `⏹️ Periodic indexing goroutine exiting`
   - `📅 Scheduler goroutine exiting`
   - `⏹️ Memory monitoring stopped`
3. **Check after scheduled refresh** (4:00 AM) that memory returns to baseline
4. **Verify on shutdown** that all cleanup messages appear

## Additional Notes

- Memory limit via `MEMORY_LIMIT_MB` env var still available for hard caps
- Periodic GC every 15 minutes provides monitoring without excessive overhead
- All fixes are backward compatible with existing functionality
