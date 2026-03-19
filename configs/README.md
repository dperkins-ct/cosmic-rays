## Configuration

### Memory Management

```json
{
  "memory_size": "1MB",
  "memory_size_auto": "10%",
  "use_locked_memory": false,
  "use_protected_memory": false
}
```

`use_locked_memory` calls `mlock(2)` to prevent the monitored region from being paged to disk. If the call fails (e.g., due to RLIMIT_MEMLOCK), the system degrades gracefully rather than aborting.

`use_protected_memory` calls `mprotect(2)` to mark pages read-only. This can detect writes that bypass the scanner, but requires the protection to be temporarily lifted during fault injection.

### Scanning and Detection

```json
{
  "duration": "30m",
  "scan_interval": "1s",
  "scan_strategy": "full",
  "sample_rate": 1.0,
  "patterns_to_use": ["alternating", "checksum", "random", "known"]
}
```

`scan_strategy` options:
- `full` — scan the entire allocated region on every cycle. Suitable for smaller allocations.
- `sampled` — scan a random subset of size `sample_rate * total_size` per cycle. Reduces CPU overhead at the cost of reduced detection sensitivity.
- `adaptive` — reserved for future implementation; currently defaults to full scan.

### Memory Patterns

The scanner initializes each memory block with a known pattern and regenerates the reference on each scan for comparison. Four patterns are supported:

| Pattern | Description |
|---|---|
| `alternating` | Alternating `0xAA` / `0x55` bytes (10101010 / 01010101) |
| `checksum` | Each byte equals its index modulo 256 |
| `random` | Deterministic pseudorandom sequence (fixed seed) |
| `known` | Repeating 4-byte sequence `0x42 0xEF 0xBE 0xAD` |

Using multiple patterns across separate memory blocks provides coverage against pattern-dependent detection gaps. Because each block uses a distinct pattern, a single flip produces one event per block type — the repair mechanism ensures this counts as one event total, not one per scan.

### Detection and Attribution

```json
{
  "enable_attribution": true,
  "attribution_threshold": 0.95,
  "enable_ecc_telemetry": false
}
```

Attribution analysis assigns a `cosmic_ray_likelihood` score (0.0–1.0) to each detected event based on the following heuristic factors:

- **Hamming distance** — single-bit flips score higher; multi-bit changes are less characteristic of SEUs
- **Injection correlation** — events matching a known injection point in time and offset are flagged as injected, not attributed to cosmic rays
- **Altitude** — if location data is provided and altitude exceeds 1,000 m, the likelihood score is adjusted upward
- **ECC telemetry** — if `enable_ecc_telemetry` is true, the system will attempt to correlate with hardware ECC counters where available (platform-dependent; most consumer systems do not expose this)

Attribution results are reported as `low` (<50%), `medium` (50–80%), or `high` (>80%) confidence.

### Fault Injection

```json
{
  "injection": {
    "enabled": true,
    "profile": "mixed",
    "rate": 2.0,
    "burst_size": 3,
    "burst_interval": "45s",
    "random_seed": 0
  }
}
```

Fault injection is used in demo mode to produce observable events for validation and presentation. Four injection profiles are available:

| Profile | Behavior |
|---|---|
| `single` | One bit flip per interval, computed from `rate` |
| `multi` | Two to four bit flips at different offsets per interval |
| `burst` | A cluster of `burst_size` flips separated by short delays, repeated at `burst_interval` |
| `mixed` | Randomly selects among the above profiles per interval |

Setting `random_seed` to `0` uses a seed derived from `crypto/rand`. A fixed non-zero seed produces a reproducible injection sequence.

Note: injected events are distinguishable from genuine soft errors in the attribution analysis — the system compares each detected offset and timestamp against its own injection history and marks matching events as `is_injected: true` with `cosmic_ray_likelihood: 0.0`.
