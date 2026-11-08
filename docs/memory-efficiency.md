# Desktop memory measurements

Measured on 8 November 2026 on Windows amd64, AMD Ryzen 7 5800X, Go 1.26.3.
The baseline production code was revision `c7dd24c`; the after measurements use
the thumbnail buffer and bounded log delivery changes in this working tree.
Each measurement ran three times. Figures below are medians unless stated otherwise.

| Measurement | Before | After | Difference |
| --- | ---: | ---: | ---: |
| GDI capture allocation per frame | 198,326 bytes | 99,953 bytes | 49.6% less |
| GDI capture allocations per frame | 39 | 38 | One fewer pixel buffer |
| GPU readback retained Go heap | 131,408 bytes | 16 bytes | About 128 KiB less per capturer |
| Blocked log burst retained Go heap | 3,399,160 bytes | 62,512 bytes | 98.2% less |
| Blocked log burst total allocated bytes | 4,873,360 bytes | 1,063,360 bytes | 78.2% less |
| Blocked log burst simultaneous requests | 1,000 | 1 | Bounded delivery |

## Workloads and limits

GDI capture used the live primary display at the default 200px thumbnail size.
Capture plus JPEG encoding allocated 206,401 bytes/frame before and 116,829
after, but this comparison includes changing desktop image content and should
not be treated as an exact encoder saving. No screenshots were saved.

The GPU retention test uses the Windows D3D11 WARP software device, a synthetic
1920x1080 texture, and a 200px thumbnail. It measures Go heap after garbage
collection, with the capturer still alive, before and after ten readbacks.
The old implementation retained a 240x135 BGRA scratch buffer; the new one
converts mapped rows directly into the returned RGBA image. Per-frame GPU
allocations remain approximately 230 KB because the old scratch buffer was
already reused. Native GPU allocations were not measured by this test.

The logger test emits 1,000 distinct 1,024-byte messages while a custom HTTP
transport blocks delivery. This isolates client-side event/request retention
without real sockets or server memory. It measures after garbage collection
while requests remain blocked, then releases and drains delivery. Heap growth
ranged from 2,977,312–3,677,768 bytes before and 62,176–69,928 bytes after.
Before, 1,000 new goroutines grew allocated stack memory by 4,882,432–8,159,232
bytes. After, there was one lazily started worker (reused across repeats), and
no measured growth in allocated stack spans.

The logger improvement includes intentionally dropping overflow deliveries:
there is one worker and capacity for 64 queued events. New webhook deliveries
are dropped when the queue is full. Local listeners still receive all events.
Accepted events preserve their original URL and credentials and are attempted
in queue order; delivery errors do not stop the worker. This queue is bounded
by event count, not by message byte length.

These results measure allocation pressure and retained Go memory in specific
workloads. They do not establish the complete application's idle working set,
native driver memory, or a CPU-speed improvement.

## Reproduction

Run from the repository's `windows` directory in PowerShell:

```powershell
go test ./internal/screenthumb -run '^$' -bench 'Benchmark(Capture|CaptureAndEncode|GPU.*)$' -benchmem -count=3
$env:SATELLITE_MEMORY_BENCH = '1'
go test ./internal/logger -run '^TestLogBurstMemory$' -v -count=3
go test ./internal/screenthumb -run '^TestGPUReadbackRetainedMemory$' -v -count=3
Remove-Item Env:SATELLITE_MEMORY_BENCH
```

For baseline comparisons, use the same measurement tests with the baseline
production files. The logger baseline was measured before editing production
code. The GPU baseline used a Go `-overlay` mapping `gpu_windows.go` to its
contents at `c7dd24c`, preserving the working tree and the new measurement test.
GPU benchmarks may skip when desktop duplication is unavailable; WARP tests
exercise the conversion without requiring an interactive GPU capture session.

## Validation

`go test ./...` and `go vet ./...` passed. The full test suite was rerun outside
the restricted sandbox to permit its local HTTP/MQTT connections. New tests
cover in-place channel conversion, source ownership, queue saturation under
concurrent producers, local listener delivery, FIFO destination/credential
preservation, and continued draining after delivery errors. Existing GPU
pipeline tests cover synthetic image colours and scaling.

The race detector was not run: this environment has CGO disabled and no GCC
compiler on PATH.
