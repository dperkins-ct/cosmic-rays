# Memory Corruption Detection System

A system for detecting memory corruption events with optional heuristic cosmic ray attribution. This tool is designed for both demonstration purposes and serious long-term monitoring of memory corruption events.

## 🌌 Background & Motivation

This experiment originated from fascination with **cosmic rays causing spontaneous bit flips in computer memory**—a documented phenomenon where high-energy particles from space can alter individual bits in RAM, potentially causing software crashes, data corruption, or unexpected behavior.

### The Cosmic Ray Phenomenon
Cosmic rays are high-energy particles (primarily protons) that constantly bombard Earth from outer space. When these particles strike computer memory, they can:
- Flip individual bits from 0→1 or 1→0
- Cause "soft errors" that don't damage hardware permanently  
- Affect systems at sea level (~1 error per GB per month)
- Increase dramatically with altitude (aircraft, satellites)
- Potentially influence election results, cryptocurrency mining, and scientific computing

### From Research to Reality
While the initial goal was pure cosmic ray detection, practical challenges led to a **dual-purpose design**:

**🔬 Research Interest**: Can we detect actual cosmic ray-induced memory corruption on consumer hardware?
- Answer: Theoretically yes, but requires months of data and statistical analysis
- Most consumer systems lack ECC telemetry for definitive attribution
- Many other sources cause identical memory corruption patterns

**🎯 Practical Solution**: Can we demonstrate the concept and educate about the phenomenon?  
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

## 🎯 Use Cases

### Demo Mode (`demo.json`)
- **Purpose**: Educational demonstrations and proof-of-concept
- **Duration**: 15 minutes (configurable)
- **Memory**: 10% of system memory (capped at 2GB for laptop safety)  
- **Fault Injection**: ENABLED by default (5 events/minute)
- **Attribution**: Heuristic analysis with 70% confidence threshold
- **Best For**: Presentations, education, testing

### Listen Mode (`listen.json`)
- **Purpose**: Long-term passive monitoring for research
- **Duration**: 24 hours (configurable)
- **Memory**: 25% of system memory (capped at 8GB for laptop safety)
- **Fault Injection**: DISABLED by default (natural events only)
- **Attribution**: High-confidence analysis (80% threshold)
- **Best For**: Research, actual cosmic ray monitoring, system testing

## 🚀 Quick Start

### Run a 5-minute demo:
```bash
go run cmd/main.go -mode demo -demo-time 5m
```

### Generate and run with demo configuration:
```bash
go run cmd/main.go -generate-config demo     # Creates configs/demo.json
go run cmd/main.go -mode demo                # Uses configs/demo.json
```

### Long-term monitoring:
```bash
go run cmd/main.go -generate-config listen   # Creates configs/listen.json  
go run cmd/main.go -mode listen              # Uses configs/listen.json
```

### Use example configurations:
```bash
# Quick 5-second test with minimal memory
go run cmd/main.go -config configs/examples/minimal-test.json

# Extended demo session  
go run cmd/main.go -config configs/examples/long-demo.json
```

## 🛠️ Installation

### Prerequisites
- Go 1.19+ 
- Linux or macOS (memory locking support)
- Sufficient RAM for your chosen memory allocation

### Build
```bash
git clone https://github.com/dperkins/cosmic-rays.git
cd cosmic-rays
go mod download
go build -o memorytest cmd/main.go
```

## ⚙️ Configuration

### Key Configuration Sections

#### Memory Management
```json
{
  "memory_size": 0,              // 0 = auto-size
  "memory_size_auto": "10%",     // Use 10% of system memory
  "use_locked_memory": true,     // Prevent swapping (graceful degradation)
  "use_protected_memory": true   // Read-only protection (graceful degradation)
}
```

#### Detection vs Attribution
```json
{
  "enable_attribution": true,        // Enable heuristic cosmic ray analysis  
  "attribution_threshold": 0.7,      // Confidence threshold (0.0-1.0)
  "enable_ecc_telemetry": false      // Try to access real ECC data
}
```

#### Fault Injection (Demo Mode)
```json
{
  "injection": {
    "enabled": true,               // Enable fault injection
    "profile": "mixed",           // "single", "multi", "burst", "mixed"
    "rate": 5.0,                  // Events per minute
    "burst_size": 3,              // Size of burst events
    "random_seed": 0              // 0 = truly random
  }
}
```

## 📊 Understanding Results

### Event Detection
The system detects when memory contents change from expected patterns:
- **Raw Events**: Memory corruption detected (neutral observation)
- **Attribution**: Heuristic analysis of likely causes

### Attribution Confidence Levels
- **High (>80%)**: Strong heuristic indicators of cosmic ray characteristics
- **Medium (>50%)**: Some cosmic ray indicators present  
- **Low (<50%)**: Weak or conflicting indicators

### Attribution Factors
- Single vs. multi-bit flips (cosmic rays typically cause single-bit flips)
- Pattern type sensitivity
- Timing analysis vs. known injections
- Geographic/altitude correlation (if enabled)
- ECC telemetry correlation (if available)

## 🔧 Advanced Usage

### Custom Configuration
```bash
# Create custom configuration from templates
cp configs/demo.json configs/my-config.json
# Edit configs/my-config.json as needed
go run cmd/main.go -config configs/my-config.json

# Use example configurations
go run cmd/main.go -config configs/examples/minimal-test.json
go run cmd/main.go -config configs/examples/long-demo.json
```

### Command Line Options
```bash
Usage: ./memorytest [options]

  -config string
        Configuration file path (default: auto-select configs/demo.json or configs/listen.json)
  -generate-config string
        Generate configuration file: 'demo' or 'listen' (saves to configs/ directory)
  -mode string
        Experiment mode: 'demo' or 'listen' (default "demo")
  -demo-time string
        Demo duration override (e.g. '5m', '30s')
  -quiet
        Suppress banner and non-essential output
  -version
        Show version and exit
```

## 📈 Output and Logging

### Console Output
- Real-time experiment progress
- Detection event summaries  
- Attribution analysis results
- Performance statistics

### File Output 
- `./output/` directory (configurable)
- JSON structured logs
- Event history and statistics
- Performance metrics

### Sample Output
```
🔬 Memory Corruption Detection Experiment
==========================================
Mode: demo
Memory allocated: 512.0 MB
Duration: 15m0s
Scan strategy: full
Patterns: [alternating checksum]
Attribution enabled: true
Fault injection: ENABLED (mixed profile, 5.0 events/min)

🚀 Starting experiment...

📊 Experiment Summary
===================
Total runtime: 15m2s
Total scans: 902
Events detected: 73
Scans per minute: 59.9
Events per minute: 4.8
Total injections: 76
Injection rate: 5.1/min

🎯 Detection Results:
• Events analyzed with heuristic attribution  
• Attribution threshold: 70%
```

## 🧪 Technical Implementation

### Architecture 
- **Detection**: Memory scanning and change detection (pkg/detector)
- **Attribution**: Heuristic analysis engine (separate from detection)
- **Injection**: Controlled fault injection for demos (pkg/injection)
- **Memory Management**: Safe allocation with protection (pkg/memory)
- **Configuration**: Mode-based config system (internal/config)

### Memory Protection
- **mlock()**: Prevents memory from being swapped to disk
- **mprotect()**: Read-only protection to detect unauthorized writes
- **Graceful Degradation**: System continues if protection fails

### Scanning Strategies
- **Full**: Complete memory scan (small allocations)
- **Sampled**: Statistical sampling (large allocations)  
- **Adaptive**: Adjusts based on system load (future enhancement)

## ⚠️ Limitations and Disclaimers

### Scientific Limitations
1. **Not a cosmic ray detector**: Detects memory corruption, not cosmic rays directly
2. **Heuristic attribution only**: Cannot definitively prove cosmic ray causation
3. **Many other causes**: Software bugs, hardware errors, electromagnetic interference
4. **No statistical significance**: Would require months/years of data for meaningful results

### Technical Limitations
1. **ECC Memory**: Most consumer systems lack accessible ECC telemetry
2. **Platform Dependent**: Memory protection may not work on all systems  
3. **False Positives**: Software bugs can cause memory corruption
4. **Resource Usage**: Uses significant RAM and CPU for scanning

### Use Case Warnings
- **Not for production systems**: High memory usage affects performance
- **Educational only**: Results not suitable for scientific publication without proper validation
- **Demo mode**: Fault injection creates artificial events for demonstration

## 🤝 Contributing

### Development Setup
```bash
git clone https://github.com/dperkins/cosmic-rays.git
cd cosmic-rays
go mod download
go test ./...
```

### Areas for Contribution
- Platform-specific ECC telemetry access
- Advanced attribution algorithms  
- Statistical analysis tools
- Performance optimizations
- Additional memory patterns

## 📜 License

MIT License - See LICENSE file for details.

## 🙏 Acknowledgments

- Inspired by real cosmic ray detection research
- Memory corruption detection techniques from systems research
- Statistical methods from particle physics

---

**Remember**: This tool demonstrates memory corruption detection techniques and provides educational insights into cosmic ray effects on computer memory. For serious cosmic ray research, use proper scientific equipment and statistical analysis methods.

## TODOs
- Integrate NOAA Data for cosmic events for listening mode
- Add unit tests for 80% code coverage
- Resolve issues with usage of `unsafe` pkg for the `injectior.go` script
- Alter experiements to only run test type at a time to resolve injection calculation inaccuracies
- Increase logging along the way as experiment runs
- Update to include the time that it actually ran rather than the time that the config says it will run since the automatic exit is not functioning