# Memory Corruption Detection System

A scientific instrument for detecting and analyzing memory corruption events, with optional heuristic attribution analysis for potential cosmic ray causation. Designed for both controlled demonstration and long-term passive monitoring.

## Background and Motivation

This project stems from documented research on high-energy particle interactions with semiconductor memory — commonly referred to as "cosmic ray induced soft errors." When galactic cosmic rays (primarily high-energy protons and heavier ions) interact with Earth's atmosphere, they produce secondary particle showers, including neutrons and muons. These secondary particles can deposit charge in DRAM cells sufficient to flip the stored bit state, producing what the industry terms a Single Event Upset (SEU).

### The Soft Error Phenomenon

Cosmic ray induced bit flips are a well-characterized problem in high-reliability computing:

- At sea level, unprotected DRAM experiences roughly one soft error per gigabyte per month under typical conditions
- Error rates increase by approximately two orders of magnitude at commercial aircraft altitudes (~35,000 ft)
- Spacecraft and high-altitude scientific instrumentation require radiation-hardened memory or aggressive ECC schemes to maintain data integrity
- At ground level, neutron flux from cosmic ray secondaries is the dominant cause of soft errors in modern DRAM

Despite being a documented physical phenomenon, casual observation is difficult. A single-GB allocation would statistically require months of continuous operation to observe a natural event, and even then, distinguishing a genuine SEU from a software fault, hardware defect, or electromagnetic interference requires significant additional analysis.

### Design Philosophy

Rather than overstating detection capability, this system is built around a clear separation of concerns:

1. **Detection**: The scanner reliably detects when memory contents deviate from a known-good reference pattern. This is a deterministic measurement.
2. **Attribution**: Heuristic analysis provides a probabilistic estimate of whether a detected event has characteristics consistent with cosmic ray causation. This is statistical inference, not proof.

This distinction is maintained throughout the codebase and reflected in the output data.

## Modes of Operation

### Demo Mode (`demo.json`)

Intended for education, presentations, and proof-of-concept demonstrations. A fault injector introduces controlled bit flips at a configurable rate, allowing the full detection and attribution pipeline to be exercised and observed in real time without waiting for natural events.

- Fault injection: enabled (configurable profile and rate)
- Memory allocation: 10% of system RAM (capped at 2 GB)
- Default duration: 15 minutes

### Listen Mode (`listen.json`)

Intended for patient, long-term passive monitoring. Fault injection is disabled; the system monitors allocated memory for naturally occurring corruption events.

- Fault injection: disabled
- Memory allocation: 25% of system RAM (capped at 8 GB)
- Default duration: 24 hours

## Quick Start

```bash
# Run a 5-minute demo
go run cmd/main.go -mode demo -demo-time 5m

# Generate and run with the demo configuration
go run cmd/main.go -generate-config demo
go run cmd/main.go -mode demo

# Long-term passive monitoring
go run cmd/main.go -generate-config listen
go run cmd/main.go -mode listen

# Run with an example configuration file
go run cmd/main.go -config configs/examples/minimal-test.json
go run cmd/main.go -config configs/examples/long-demo.json
```

### From Research to Reality
While the initial goal was pure cosmic ray detection, practical challenges led to a **dual-purpose design**:

**Research Interest**: Can we detect actual cosmic ray-induced memory corruption on consumer hardware?
- Answer: Theoretically yes, but requires months of data and statistical analysis
- Most consumer systems lack ECC telemetry for definitive attribution
- Many other sources cause identical memory corruption patterns

**Practical Solution**: Can we demonstrate the concept and educate about the phenomenon?  
- Answer: Absolutely! Demo mode with controlled fault injection provides:
  - Reliable, observable bit flips for presentations and education
  - Understanding of memory corruption detection techniques
  - Appreciation for the cosmic ray phenomenon without waiting months for natural events

### Why This Approach Works
Rather than overselling our detection capabilities, this system:
- **Separates detection from attribution**: We detect memory changes, then provide heuristic analysis
- **Offers both modes**: Demo mode for education, Listen mode for patient research  
- **Maintains scientific integrity**: Clear about what we can and cannot definitively determine
- **Balances curiosity with practicality**: Satisfies interest in cosmic rays while being genuinely useful

The result is a tool that respects both the fascinating science of cosmic ray interactions and the practical needs of demonstration and education.

## 🔬 Scientific Approach

**What this system actually does:**
- Detects when memory contents change unexpectedly
- Provides heuristic analysis to estimate likelihood of cosmic ray causation 
- Separates raw detection from attribution analysis
- Offers fault injection for reliable demonstration

**What this system does NOT do:**
- Definitively prove cosmic ray causation (requires specialized equipment)
- Detect cosmic rays directly (measures secondary effects only)
- Replace proper ECC memory or radiation detection equipment
- Provide legally or scientifically binding cosmic ray attribution



## Technical Limitations

1. **Detection latency** — corruptions are only observable at scan boundaries. A flip that occurs immediately after a scan and is corrected at the next scan persists for up to one full `scan_interval`.
2. **No hardware attribution** — without ECC telemetry or a particle detector, attribution remains a heuristic estimate and cannot be used as scientific evidence.
3. **Identical soft error signatures** — alpha particle emission from DRAM packaging, thermal noise, and row-hammer effects produce single-bit flips that are statistically indistinguishable from SEUs at the software level.
4. **Platform constraints** — `mlock` and `mprotect` behavior is OS- and privilege-dependent. The system degrades gracefully when these calls fail.
5. **Statistical requirements** — meaningful natural event rates at sea level require months of continuous data collection. Demo mode with fault injection is suitable for functional validation; it is not a substitute for natural event data.


### Use Case Warnings
- **Not for production systems**: High memory usage affects performance
- **Educational only**: Results not suitable for scientific publication without proper validation
- **Demo mode**: Fault injection creates artificial events for demonstration

# Contributing

Areas of active interest:
- Platform-specific ECC telemetry access (Linux `edac` subsystem, AMD SMN registers)
- Improved statistical attribution models
- Adaptive scan strategies based on observed event rate
- Unit test coverage (target: 80%)

```bash
git clone https://github.com/dperkins/cosmic-rays.git
cd cosmic-rays
go mod download
go test ./...
```

## Open Issues

- NOAA space weather data integration for listen mode correlation
- Resolution of `unsafe` package usage in `injector.go`
- Automatic experiment termination not functioning; output currently logs configured duration rather than actual elapsed time
- Logging granularity during experiment execution needs improvement

## License

MIT License. See LICENSE for details.