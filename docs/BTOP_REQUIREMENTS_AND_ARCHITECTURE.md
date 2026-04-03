# btop++ Requirements and Architecture Documentation

> **Reference document — written by the dtop authors, not derived from btop++ documentation.**
> This is a reverse-engineered specification of btop++ (https://github.com/aristocratos/btop),
> produced by studying its behaviour and public source code. It was used as a design reference
> when building dtop. Do not treat C++-specific APIs or build flags as dtop implementation
> requirements; dtop is an independent Go reimplementation and must remain language- and
> platform-portable.
>
> btop++ is copyright 2021 Aristocratos (jakob@qvantnet.com), Apache License 2.0.

## Executive Summary

btop++ is a cross-platform terminal-based resource monitor written in C++23 that displays real-time system information including CPU, memory, disk, network, and GPU usage. It is a continuation of bashtop (Bash) and bpytop (Python), rewritten in C++ for performance and portability.

**Version**: 1.4.6  
**Language**: C++23 (requires GCC 14+ or Clang 19+)  
**License**: Apache 2.0  
**Platforms**: Linux, macOS, FreeBSD, OpenBSD, NetBSD

---

## 1. Functional Requirements

### 1.1 Core Features

#### System Monitoring
- **CPU Monitoring**
  - Real-time CPU utilization (total and per-core)
  - CPU frequency monitoring
  - Temperature monitoring (from system sensors)
  - Power consumption monitoring (watts) - requires elevated privileges
  - Load averages (1, 5, 15 minutes)
  - Multiple graph representations (braille, block, tty symbols)
  - Support for multiple CPU stats: user, nice, system, idle, iowait, irq, softirq, steal, guest, guest_nice
  - Container detection (Docker, LXC, etc.)
  - Core mapping support for correct temperature assignment

- **Memory Monitoring**
  - RAM usage (used, available, cached, free)
  - Swap usage monitoring
  - Disk I/O monitoring (read/write speeds and activity)
  - Multiple disk support with filtering
  - ZFS ARC cache support
  - Disk usage percentages and graphs
  - Support for /etc/fstab disk enumeration
  - Base-10 or base-1024 size calculations

- **Network Monitoring**
  - Network interface selection
  - Upload/download bandwidth monitoring
  - Real-time bandwidth graphs with auto-scaling
  - IPv4 and IPv6 address display
  - Connection status
  - Total data transferred tracking
  - Manual or automatic graph scale adjustment
  - Multiple graph speed presets

- **Process Monitoring**
  - Process list with sorting options: pid, name, command, threads, user, memory, CPU (lazy/direct)
  - Tree view of processes showing parent-child relationships
  - Process filtering by name/command
  - Detailed process information view including:
    - Memory usage (with optional /proc/[pid]/smaps detail)
    - CPU usage percentage
    - Process state
    - Nice value
    - I/O read/write
    - Elapsed time
    - Parent process
  - Process control:
    - Send signals (SIGTERM, SIGKILL, SIGHUP, etc.)
    - Change process priority (renice)
    - Pause process list updates
    - Follow specific process
  - Process aggregation in tree view
  - Preserve dead process stats when paused
  - Mini CPU graphs per process

- **GPU Monitoring** (Linux x86_64 only)
  - Support for NVIDIA, AMD, and Intel GPUs
  - Up to 6 GPUs simultaneously
  - Per-GPU metrics:
    - GPU utilization percentage
    - Memory utilization
    - Temperature and max temperature
    - Power usage and max power
    - Clock speeds (GPU and memory)
    - PCIe throughput (TX/RX)
    - Encoder/decoder utilization
    - VRAM usage
  - GPU stats can be displayed in CPU box or dedicated GPU boxes
  - Toggled with keys 5, 6, 7, 0 for GPU1-4

### 1.2 User Interface

#### Display Features
- **Terminal Requirements**
  - 24-bit truecolor support (preferred)
  - 256-color fallback mode
  - 16-color TTY mode
  - UTF-8 locale required
  - Wide character support
  - Unicode blocks: Braille Patterns (U+2800-U+28FF), Geometric Shapes (U+25A0-U+25FF), Box Drawing and Block Elements (U+2500-U+259F)

- **UI Layout**
  - Modular box system: CPU, Memory/Disk, Network, Process, GPU (0-5)
  - Flexible box positioning and visibility
  - Rounded or square corners (configurable)
  - Color-coded elements
  - Responsive to terminal resize
  - Minimum size requirements enforced
  - Banner display with version info
  - Clock display with custom formatting

- **Themes**
  - 40+ built-in themes
  - Custom theme support
  - Theme file format compatible with bpytop/bashtop
  - Configurable colors for:
    - Main background/foreground
    - Box outlines (CPU, memory, network, process)
    - Graphs (temperature, CPU, memory, swap, network)
    - Process list (selected, inactive, misc)
    - Titles, highlights, dividers
    - Process following and pause indicators
  - Theme search paths:
    - System: `../share/btop/themes`, `/usr/local/share/btop/themes`, `/usr/share/btop/themes`
    - User: `$XDG_CONFIG_HOME/btop/themes` or `$HOME/.config/btop/themes`
    - Custom via `--themes-dir` command line option

- **Graph Symbols**
  - Braille: highest resolution (Unicode braille patterns)
  - Block: medium resolution (Unicode block elements)
  - TTY: lowest resolution (ASCII-compatible, 3 symbols)
  - Per-box graph symbol selection

#### Interactive Controls

- **Keyboard Navigation**
  - Arrow keys: Navigate process list
  - Page Up/Down: Scroll process list
  - Home/End: Jump to top/bottom of list
  - Enter: View detailed process info
  - Backspace/ESC: Exit menus/details
  - Tab: Cycle through sorting options
  - Space: Pause/unpause process updates
  - F2: Show options menu
  - F1/h: Show help menu
  - F3/f: Filter processes
  - F5/t: Toggle tree view
  - F6: Select sort column
  - F9/k: Send signal to process
  - 1-9: Toggle boxes or select presets
  - +/-: Increment/decrement sorting
  - Optional vim-style navigation (h, j, k, l, g, G)

- **Mouse Support**
  - Full mouse support for all UI elements
  - Click on boxes to toggle visibility
  - Click on buttons and menu items
  - Scroll in process list and menus
  - Mouse position tracking

### 1.3 Configuration

#### Configuration File (`btop.conf`)
- Auto-generated on first run
- Location: `$XDG_CONFIG_HOME/btop/btop.conf` or `$HOME/.config/btop/btop.conf`
- All options editable via UI menu
- Options include:
  - **Appearance**: color_theme, theme_background, truecolor, force_tty, rounded_corners
  - **Graph symbols**: graph_symbol (global), graph_symbol_cpu/mem/net/proc (per-box)
  - **Box visibility**: shown_boxes (space-separated list)
  - **Update rate**: update_ms (milliseconds, recommended ≥2000)
  - **Process options**: sorting, reversed, tree view, colors, gradient, per_core, mem_bytes, cpu_graphs, info_smaps, left positioning, filter_kernel, follow_detailed, aggregate, keep_dead_proc_usage
  - **CPU options**: graph_upper/lower stats, invert_lower, single_graph, bottom positioning, show_uptime, show_cpu_watts, check_temp, cpu_sensor, show_coretemp, cpu_core_map, temp_scale (celsius/fahrenheit/kelvin/rankine), show_cpu_freq, freq_mode, custom_cpu_name
  - **Memory options**: mem_graphs, mem_below_net, zfs_arc_cached, show_swap, swap_disk, show_disks, only_physical, use_fstab, zfs_hide_datasets, disk_free_priv, show_io_stat, io_mode, io_graph_combined, io_graph_speeds, disks_filter
  - **Network options**: net_download/upload speeds, net_auto, net_sync, net_iface, base_10_bitrate
  - **Battery**: show_battery, selected_battery, show_battery_watts
  - **Presets**: up to 9 custom presets with box configuration
  - **Other**: vim_keys, terminal_sync, base_10_sizes, clock_format, background_update, log_level

#### Command-Line Options
```
-c, --config <file>       Path to custom config file
-d, --debug               Debug mode with verbose logging
-f, --filter <filter>     Initial process filter
    --force-utf           Override UTF locale detection
-l, --low-color           256-color mode
-p, --preset <id>         Start with preset (0-9)
-t, --tty                 Force TTY mode (16 colors)
    --no-tty              Force disable TTY mode
-u, --update <ms>         Initial update rate
    --default-config      Print default config to stdout
    --themes-dir <dir>    Custom themes directory
-h, --help                Show help
-V, --version             Show version
```

### 1.4 Data Collection Requirements

#### Platform-Specific Implementation
Each platform must implement collection functions for:
- `Cpu::collect()` - CPU stats and temperatures
- `Mem::collect()` - Memory and disk stats
- `Net::collect()` - Network stats
- `Proc::collect()` - Process information
- `Gpu::collect()` - GPU information (Linux only)

#### Linux Data Sources
- `/proc/stat` - CPU statistics
- `/proc/cpuinfo` - CPU information
- `/proc/meminfo` - Memory statistics
- `/proc/[pid]/*` - Process information
- `/proc/net/dev` - Network statistics
- `/sys/class/hwmon/` - Temperature sensors
- `/sys/class/power_supply/` - Battery information
- `/sys/devices/` - Various device information
- NVML library (NVIDIA GPUs)
- ROCm SMI library (AMD GPUs)
- Intel GPU Top (Intel integrated GPUs)

#### macOS Data Sources
- `sysctl` calls for system information
- IOKit framework for hardware monitoring
- SMC (System Management Controller) for temperatures
- `kvm` library for process information

#### BSD Data Sources
- `sysctl` calls for system statistics
- `kvm` library for kernel access
- `devstat` library (FreeBSD) for disk statistics

### 1.5 Performance Requirements
- Update interval: configurable from 100ms to multiple seconds (recommended: 2000ms+)
- Minimal CPU overhead when idle
- Efficient memory usage
- Smooth graph updates without flickering
- Sub-second input response time
- Terminal synchronized output for flicker reduction
- Non-blocking I/O operations
- Threaded data collection

---

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         btop++ Application                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌───────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │  Main Thread  │  │ Runner Thread│  │   Input Handler      │  │
│  │               │  │              │  │                      │  │
│  │ • UI Render   │  │ • Data       │  │ • Keyboard Events   │  │
│  │ • Event Loop  │  │   Collection │  │ • Mouse Events      │  │
│  │ • Menu System │  │ • Periodic   │  │ • Signal Handling   │  │
│  └───────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
│          │                 │                     │              │
│          └─────────────────┴─────────────────────┘              │
│                            │                                     │
│  ┌─────────────────────────┴──────────────────────────────────┐ │
│  │                   Shared State (Atomics)                    │ │
│  │  • Global flags  • Runner flags  • Resize events           │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                  Core Modules                              │  │
│  │  ┌──────────┐ ┌───────────┐ ┌────────┐ ┌──────────────┐  │  │
│  │  │  Config  │ │   Theme   │ │  Draw  │ │     Menu     │  │  │
│  │  └──────────┘ └───────────┘ └────────┘ └──────────────┘  │  │
│  │  ┌──────────┐ ┌───────────┐ ┌────────┐ ┌──────────────┐  │  │
│  │  │   Input  │ │   Tools   │ │   Log  │ │     CLI      │  │  │
│  │  └──────────┘ └───────────┘ └────────┘ └──────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │              Data Collection Modules                       │  │
│  │  ┌─────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌─────────────────┐  │  │
│  │  │ CPU │ │ Mem  │ │ Net  │ │ Proc │ │ GPU (optional)  │  │  │
│  │  └─────┘ └──────┘ └──────┘ └──────┘ └─────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                            │                                     │
│  ┌─────────────────────────┴──────────────────────────────────┐ │
│  │        Platform-Specific Collectors (btop_collect.cpp)     │ │
│  │  ┌───────┐ ┌────────┐ ┌─────────┐ ┌─────────┐ ┌────────┐ │ │
│  │  │ Linux │ │ macOS  │ │ FreeBSD │ │ OpenBSD │ │ NetBSD │ │ │
│  │  └───────┘ └────────┘ └─────────┘ └─────────┘ └────────┘ │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                            │                                     │
│  ┌─────────────────────────┴──────────────────────────────────┐ │
│  │              System APIs / Kernel Interfaces                │ │
│  │  /proc • sysctl • IOKit • kvm • NVML • ROCm SMI • igt      │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Threading Model

#### Main Thread
- **Responsibility**: UI rendering, input processing, menu system
- **Operations**:
  - Initialize terminal
  - Enter main event loop
  - Process keyboard/mouse input
  - Render UI boxes
  - Handle menu interactions
  - Manage terminal resize events
  - Coordinate with runner thread

#### Runner Thread
- **Responsibility**: Periodic data collection
- **Operations**:
  - Sleep until next update cycle
  - Call `Cpu::collect()`
  - Call `Mem::collect()`
  - Call `Net::collect()`
  - Call `Proc::collect()`
  - Call `Gpu::collect()` (if enabled)
  - Set redraw flags
  - Handle stop signals

#### Synchronization
- Atomic flags for thread communication:
  - `Runner::active` - Runner thread is active
  - `Runner::reading` - Runner is currently reading data
  - `Runner::stopping` - Runner should stop
  - `Runner::redraw` - UI needs redraw
  - `Global::resized` - Terminal was resized
  - `Global::quitting` - Application is quitting
  - `Proc::resized` - Process box was resized
- No mutexes required due to atomic operations and single-writer pattern
- Input interrupt mechanism using `ppoll()` with signal mask

### 2.3 Module Descriptions

#### btop.cpp / btop.hpp
- **Purpose**: Main application logic and entry point
- **Key Functions**:
  - `btop_main()` - Application entry point
  - `clean_quit()` - Cleanup and exit handler
  - `term_resize()` - Terminal resize handler
  - Signal handlers for SIGINT, SIGTSTP, SIGCONT, SIGWINCH, SIGSEGV, SIGABRT
  - Main event loop coordination
- **Contains**:
  - Global namespace: version, banner, exit handling, atomic flags
  - Runner namespace: thread management

#### btop_shared.cpp / btop_shared.hpp
- **Purpose**: Shared interfaces and data structures for all modules
- **Defines**:
  - Namespace interfaces: `Cpu`, `Mem`, `Net`, `Proc`, `Gpu`
  - Data structures: `cpu_info`, `mem_info`, `net_info`, `proc_info`, `gpu_info`, `disk_info`
  - Function signatures for `collect()` and `draw()` per module
  - Shared constants and enums (process states, sort options, etc.)
  - Platform detection helpers
- **Key Concepts**:
  - Each module has standardized `collect()` and `draw()` functions
  - Data containers use `std::unordered_map` with `deque<long long>` for historical graph data
  - Process tree structures for hierarchical display

#### btop_config.cpp / btop_config.hpp
- **Purpose**: Configuration file parsing and management
- **Data Structures**:
  - `strings` - String configuration values
  - `bools` - Boolean configuration values  
  - `ints` - Integer configuration values
  - Temporary storage: `stringsTmp`, `boolsTmp`, `intsTmp` (for locked config)
- **Key Functions**:
  - `load()` - Parse config file from disk
  - `write()` - Write current config to disk
  - `set()` - Set configuration value (with lock support)
  - `getS()`, `getB()`, `getI()` - Get configuration values
  - `lock()` / `unlock()` - Temporarily lock config changes
  - `set_boxes()` - Validate and set visible boxes
  - `toggle_box()` - Toggle box visibility
  - `apply_preset()` - Apply saved preset configuration
  - `current_config()` - Generate in-memory config string
- **Config Location**: `$XDG_CONFIG_HOME/btop/btop.conf` or `$HOME/.config/btop/btop.conf`

#### btop_theme.cpp / btop_theme.hpp
- **Purpose**: Theme loading and color management
- **Data Structures**:
  - `colors` - Map of color names to escape sequences
  - `rgbs` - Map of color names to RGB values
  - `gradients` - Map of gradient names to 101-element arrays of color strings
- **Key Functions**:
  - `setTheme()` - Load and apply theme from file
  - `updateThemes()` - Scan filesystem for available themes
  - `hex_to_color()` - Convert hex color to escape sequence
  - `dec_to_color()` - Convert RGB to escape sequence
  - `c()` - Get color escape sequence by name
  - `g()` - Get gradient array by name
  - `dec()` - Get RGB array by name
- **Theme Format**:
  - Key-value pairs: `theme[color_name]="#RRGGBB"` or `"R G B"`
  - Gradients: `_start`, `_mid`, `_end` suffixes
  - Supports 24-bit RGB, 256-color palette, and 16-color fallback
  - Compatible with bpytop/bashtop themes

#### btop_draw.cpp / btop_draw.hpp
- **Purpose**: UI rendering primitives and layout calculation
- **Classes**:
  - `Graph` - Renders time-series graphs with various symbol sets
    - Supports braille, block, and TTY symbols
    - Handles data smoothing and scaling
    - Maintains two graph representations for braille sub-pixel rendering
  - `Meter` - Renders percentage meters/gauges
    - Pre-cached for performance
    - Gradient support
    - Invert support
  - `TextEdit` - Editable text field for filtering
    - Cursor positioning
    - Insert/delete operations
    - Numeric mode
- **Key Functions**:
  - `calcSizes()` - Calculate box dimensions based on terminal size
  - `createBox()` - Draw box with borders and title
  - `banner_gen()` - Generate ASCII art banner
  - `update_clock()` - Update clock display
- **Layout System**:
  - Responsive grid layout
  - Minimum size enforcement per box
  - Box visibility toggling
  - Per-box position overrides

#### btop_input.cpp / btop_input.hpp
- **Purpose**: Keyboard and mouse input handling
- **Key Functions**:
  - `poll()` - Non-blocking input check with timeout
  - `get()` - Read key or mouse action
  - `wait()` - Blocking wait for input
  - `interrupt()` - Wake up blocked input (for thread coordination)
  - `process()` - Process input action (key bindings)
  - `clear()` - Clear input history
- **Input Processing**:
  - Escape sequence parsing for special keys
  - Mouse SGR mode support (position and button tracking)
  - Signal mask during poll to allow interrupts
  - Key history for debugging
- **Mouse Mappings**: Tracks clickable regions with `Mouse_loc` structures

#### btop_menu.cpp / btop_menu.hpp
- **Purpose**: Interactive menu system
- **Menu Types** (enum `Menus`):
  - `Main` - Main menu with options
  - `Help` - Keyboard shortcuts help
  - `Options` - Configuration options editor
  - `SignalChoose` - Signal selection for process
  - `SignalSend` - Confirm signal send
  - `SignalReturn` - Signal result
  - `Renice` - Change process priority
  - `SizeError` - Terminal too small warning
- **Class `msgBox`**:
  - Modal message boxes
  - Button types: OK, YES/NO
  - Keyboard navigation
  - Returns action taken
- **Key Functions**:
  - `show()` - Display menu
  - `process()` - Handle menu input
  - Active menu tracking with `menuMask` bitset

#### btop_tools.cpp / btop_tools.hpp
- **Purpose**: Utility functions for string manipulation, time, and terminal control
- **Namespaces**:
  - `Fx` - Text formatting escape codes (bold, italic, underline, colors, reset)
  - `Mv` - Cursor movement escape codes (to, up, down, left, right, save, restore)
  - `Term` - Terminal control (init, restore, resize, alt screen, mouse on/off, sync output)
- **Key Utility Functions**:
  - `ulen()` / `wide_ulen()` - UTF-8 string length (character count vs. display width)
  - `uresize()` / `luresize()` - Resize UTF-8 strings
  - `s_replace()` - String replacement
  - `capitalize()`, `str_to_upper()`, `str_to_lower()` - Case conversion
  - `v_contains()`, `v_index()`, `is_in()` - Vector/value utilities
  - `time_s()`, `time_ms()`, `time_micros()` - Current time
  - `stobool()`, `isint()` - String validation and conversion
  - `ltrim()`, `rtrim()`, `trim()` - String trimming
  - `ssplit()` - String splitting (C++20 ranges)
  - `sleep_ms()`, `sleep_micros()` - Thread sleep
  - `ljust()`, `rjust()`, `cjust()` - String justification
  - `sec_to_dhms()` - Time formatting
  - `floating_humanizer()` - Human-readable byte sizes (KB, MB, GB, etc.)
  - `safeVal()` - Safe map/vector access with fallback
  - `readfile()` - Read complete file to string
  - `celsius_to()` - Temperature scale conversion
  - `hostname()`, `username()` - System information
  - `atomic_wait()`, `atomic_wait_for()` - Atomic synchronization helpers
- **Class `atomic_lock`**: RAII atomic flag setter
- **Class `DebugTimer`**: Performance timing for debugging

#### btop_log.cpp / btop_log.hpp
- **Purpose**: Logging system with multiple severity levels
- **Log Levels**: ERROR, WARNING, INFO, DEBUG
- **Log Location**: `~/.local/state/btop/btop.log` or `$XDG_STATE_HOME/btop/btop.log`
- **Key Functions**:
  - `Logger::error()`, `Logger::warning()`, `Logger::info()`, `Logger::debug()`
  - Timestamp prefix
  - File rotation
  - Thread-safe

#### btop_cli.cpp / btop_cli.hpp
- **Purpose**: Command-line argument parsing
- **Key Functions**:
  - `parse_arguments()` - Parse command line options
  - `print_help()` - Display usage information
  - `print_version()` - Display version and build information
  - `print_default_config()` - Output default configuration

#### Platform-Specific Collectors (btop_collect.cpp)

Each platform has its own implementation in:
- `src/linux/btop_collect.cpp`
- `src/osx/btop_collect.cpp` (+ `sensors.cpp`, `smc.cpp`)
- `src/freebsd/btop_collect.cpp`
- `src/openbsd/btop_collect.cpp` (+ `sysctlbyname.cpp`)
- `src/netbsd/btop_collect.cpp`

**Common Structure**:
Each file implements the same interface defined in `btop_shared.hpp`:

```cpp
namespace Cpu {
    auto collect(bool no_update = false) -> cpu_info&;
    auto get_cpuHz() -> string;
    auto get_battery() -> tuple<int, float, long, string>;
    // Platform-specific helpers
}

namespace Mem {
    auto collect(bool no_update = false) -> mem_info&;
    uint64_t get_totalMem();
    // Platform-specific helpers
}

namespace Net {
    auto collect(bool no_update = false) -> net_info&;
    // Platform-specific helpers
}

namespace Proc {
    auto collect(bool no_update = false) -> vector<proc_info>&;
    // Platform-specific helpers
}

namespace Gpu {
    auto collect(bool no_update = false) -> vector<gpu_info>&;
    // Platform-specific helpers (Linux only)
}
```

**Linux Implementation Highlights**:
- **CPU**: 
  - Parse `/proc/stat` for CPU times
  - Read `/sys/class/hwmon/` for temperatures
  - Read `/sys/devices/system/cpu/cpu*/cpufreq/` for frequencies
  - Read `/sys/class/powercap/intel-rapl/` for power usage
  - Container detection via `/proc/1/cgroup` or environment variables
- **Memory**: 
  - Parse `/proc/meminfo`
  - Read `/proc/diskstats` for I/O
  - Parse `/proc/mounts` and query with `statvfs()`
  - ZFS ARC from `/proc/spl/kstat/zfs/arcstats`
- **Network**: 
  - Parse `/proc/net/dev` for interface stats
  - Use `getifaddrs()` for IP addresses
- **Process**: 
  - Enumerate `/proc/[pid]/` directories
  - Parse `/proc/[pid]/stat` for process info
  - Parse `/proc/[pid]/status` for additional data
  - Parse `/proc/[pid]/cmdline` for command
  - Optional `/proc/[pid]/smaps` for detailed memory
  - User lookup via `getpwuid()` (unless static glibc build)
- **GPU**:
  - **NVIDIA**: Dynamic loading of `libnvidia-ml.so` (NVML)
  - **AMD**: Dynamic loading of `librocm_smi64.so` (ROCm SMI) or static linking
  - **Intel**: Integrated Intel GPU Top utility (C code in `src/linux/intel_gpu_top/`)

**macOS Implementation Highlights**:
- **CPU**: `sysctlbyname()` for CPU info, IOKit for temperatures, SMC for sensors
- **Memory**: `vm_statistics64()` and `statfs()`
- **Network**: `getifaddrs()` and interface counters
- **Process**: `kvm` library for process enumeration
- **No GPU support**

**BSD Implementation Highlights**:
- **CPU**: `sysctlbyname()` for CPU stats
- **Memory**: `kvm` library for memory info, `devstat` (FreeBSD) for disk I/O
- **Network**: `getifaddrs()` and interface statistics
- **Process**: `kvm` library for process information
- **No GPU support**

### 2.4 Data Flow

#### Typical Update Cycle
```
1. Timer expires (update_ms interval)
2. Runner thread wakes up
3. Runner::active set to true
4. For each enabled box:
   a. Call [Module]::collect()
   b. Platform-specific data collection
   c. Update internal data structures
   d. Append to historical deques
   e. Set redraw flag
5. Runner::active set to false
6. Runner sleeps until next cycle

Main Thread (parallel):
1. Check input with poll()
2. Process keyboard/mouse events
3. If redraw flags set:
   a. Clear terminal regions
   b. Call [Module]::draw() for each box
   c. Compose output strings
   d. Write to terminal
   e. Flush output
4. Update clock if needed
5. Loop
```

#### Data Collection to Display Pipeline
```
System APIs
    ↓
Platform-specific collect() functions
    ↓
Parse and calculate deltas/percentages
    ↓
Update data structures (cpu_info, mem_info, etc.)
    ↓
Append to historical deques (for graphs)
    ↓
Set redraw flags
    ↓
Main thread detects redraw flags
    ↓
Call draw() functions
    ↓
Generate strings with:
    - Box outlines
    - Text data
    - Graphs (via Graph class)
    - Meters (via Meter class)
    - Color escape sequences
    ↓
Write to terminal with optional sync
```

### 2.5 Abstractions and Interfaces (language-neutral)

Use these interface contracts regardless of implementation language. A “tick” is one runner cycle.

```pseudocode path=null start=null
interface Collector { collect(now) -> DataSet | Error }
interface Drawer    { draw(State, Capabilities) -> Frame }
interface Input     { poll(timeout_ms) -> Event[] }
interface Config    { get(key) -> value; set(key, value); persist(); reload(); on_change(cb) }
interface Logger    { log(level, message, context) }
interface Clock     { now() -> instant; sleep_until(instant) }
```

Collectors per domain must expose:
- cpu.collect(): returns CPU load %, per-core %, temps (°C), freq (MHz), load averages, optional watts.
- mem.collect(): returns RAM+swap used/free, cached/buffered, disks list with capacity and I/O deltas.
- net.collect(): returns bandwidth per direction, peaks, totals, IPv4/IPv6, link up/down.
- proc.collect(): returns list with pid, ppid, state, user, cpu%, mem bytes, cmd, nice, threads, start time; may include tree metadata and optional smaps-derived fields.
- gpu.collect(): optional; returns per GPU utilization %, memory, clocks, temps, power, PCIe tx/rx, encoder/decoder %, supported feature flags.

Drawers consume immutable snapshots produced by collectors plus UI state (visible boxes, sorting, filters) and produce a string/buffer ready for terminal write.

### 2.6 State & Data Schemas (language-neutral)

All numeric rates are per second unless stated; history buffers are bounded to current graph width (trim oldest when over capacity).

- CPU snapshot
  - total_percent: deque<number>
  - per_core_percent: list<deque<number>>
  - temps_c: list<deque<number>>; temp_max_c:number
  - load_avg: [1m,5m,15m] floats
  - watts: float | null
  - active_cpus: list<int> | null (for containers)
- Memory snapshot
  - stats: map<string, uint64> (mem_total, mem_used, mem_free, swap_* etc.)
  - disks: map<string, Disk>
  - disk_order: list<string>
  - percent_history: map<string, deque<number>> (mem, swap, zfs_arc, etc.)
  - Disk:
    - name, fs_type, device_path, stat_path
    - total_bytes, used_bytes, free_bytes
    - used_percent, free_percent
    - io_read, io_write, io_activity: deque<number>
    - old_io_counters: tuple<int64,int64,int64>
- Network snapshot
  - bandwidth: {download: deque<number>, upload: deque<number>}
  - iface_stats: map<string, {speed, peak, total, last, offset, rollovers}>
  - ipv4, ipv6: string | null
  - connected: bool
- Process snapshot
  - entries: list<Proc>
  - Proc fields: pid, ppid, user, state, cmd_full, cmd_short, threads, nice, cpu_pct, cpu_time, mem_bytes, start_time, depth, tree_prefix, collapsed, filtered, death_time|null
  - detail_cache (optional): last_pid, formatted strings (elapsed, parent, status, io_read, io_write, memory), first_mem, history (cpu_percent deque, mem_bytes deque), skip_smaps flag
- GPU snapshot (optional)
  - per_gpu: list<Gpu>
  - Gpu fields: utilization_percent deque, mem_util_percent deque, mem_total, mem_used, temp deque, temp_max, gpu_clock_mhz, mem_clock_mhz, power_usage, power_cap, pcie_tx_kbs, pcie_rx_kbs, encoder_pct, decoder_pct, supported_flags

### 2.7 Concurrency & Timing Contract

- Runner thread wakes every update_ms; skips a cycle if a collection is still in progress (no overlapping collects).
- Shared state is single-writer (runner) and single-reader (UI). Use lock-free or message-passing; avoid partial writes by swapping whole snapshots.
- Flags/events:
  - runner_active, runner_reading, runner_stopping, needs_redraw (set by runner)
  - resized, quitting (set by main/input)
- Shutdown: main sets runner_stopping, interrupts any poll, waits for runner to acknowledge, then restores terminal.
- Missed frame behavior: if UI can’t render before next tick, keep latest snapshot and drop intermediate ones.

### 2.8 Rendering & Layout Rules

- Layout: grid of independent boxes (CPU, Mem/Disk, Net, Proc, GPU1-6); enforce per-box minimums; hide boxes that don’t fit and surface a size warning.
- Graph history length equals current drawn width; on resize, truncate or pad with last value.
- Scaling:
  - Net graphs auto-scale but never below 10 KiB/s; manual scale overrides auto.
  - CPU/mem graphs scale to 0–100% unless watts/temps are shown (then use respective max).
- Color downgrade order: truecolor → 256 → 16-color; all themes must define fallbacks.
- Text: UTF-8 only; measure display width (wcwidth); truncate with ellipsis when exceeding column.
- Render order: clear region → draw borders → fill text/graphs → apply highlights → flush; keep the terminal in alt-screen when available; enable/disable sync output when supported.

### 2.9 Input & Event Handling

- Poll loop is non-blocking with timeout <= update_ms.
- Key binding precedence: modal menus > focused box actions > global actions.
- Mouse: SGR mode; map clicks to hit-tested regions; scroll affects the focused list/graph.
- Typing in filters: cursor movement, insert/delete, numeric mode when required; max length must be enforced to avoid layout overflow.
- When input arrives during render, queue it and process after frame flush to avoid tearing.

### 2.10 Error Handling & Fallback Matrix

- Missing data source (e.g., temps, watts, GPU lib): degrade feature, mark field “N/A”, log at INFO once and DEBUG thereafter; do not crash.
- Permission errors (setcap/setuid absent): disable affected metrics; show inline hint in options/help.
- Terminal lacks UTF-8 or colors: force TTY mode (ASCII graphs, 16 colors).
- Collector failure in a tick: keep last good snapshot and flag warning; avoid propagating partial data.
- Config parse errors: revert to defaults for bad keys; preserve unknown keys for forward compatibility.

### 2.11 Performance & Acceptance Targets

- Input latency: <100 ms at 2s update interval.
- Runner overhead: <2% single-core CPU on a mid-range machine at 2s interval with 1k processes.
- Memory footprint target: <60 MB without smaps; <120 MB with smaps enabled.
- Render time budget: <30 ms per frame at 120 columns.
- Startup: <500 ms to first frame on Linux.

### 2.12 Portability Guidance (Go / Rust focus)

- Isolate platform code behind Collector interfaces; each OS lives in its own package/module with the same contract.
- Avoid global mutable state; pass immutable snapshots to the renderer.
- Choose UTF-8 width lib with zero allocations per call; cache theme color conversions.
- Provide FFI shims for GPU libraries (NVML/ROCm) behind an optional build tag/feature flag.
- Use structured logging with levels; make logger injectable to simplify testing.
- Make configuration a first-class object with live reload and validation; persist atomically.
- For Go: prefer context-based cancellation for runner/input; for Rust: use channels (crossbeam/tokio) for snapshot handoff.

### 2.13 Build System (language-specific reference)

#### Makefile (Primary)
- **Auto-detection**: Platform (Linux, macOS, FreeBSD, OpenBSD, NetBSD) and architecture via `uname` and compiler
- **Compiler Requirements**: GCC 14+ or Clang 19+ with C++23 support
- **Flags**:
  - `STATIC=true` - Static linking (musl or glibc)
  - `GPU_SUPPORT=true|false` - Enable GPU monitoring (auto-enabled on Linux x86_64)
  - `RSMI_STATIC=true` - Statically link ROCm SMI library for AMD GPUs
  - `DEBUG=true` - Debug build with `-O0 -g` and verbose logging
  - `VERBOSE=true` - Show full compiler commands
  - `ARCH=<arch>` - Manually set architecture
  - `CXX=<compiler>` - Specify compiler
  - `ADDFLAGS=<flags>` - Additional compiler/linker flags
  - `PREFIX=/path` - Installation prefix (default: `/usr/local`)
- **Targets**:
  - `make` - Build btop
  - `make install` - Install btop and themes
  - `make setcap` - Set capabilities (for Intel GPU and CPU wattage)
  - `make setuid` - Set SUID bit
  - `make uninstall` - Remove installed files
  - `make clean` - Remove object files
  - `make distclean` - Remove objects and binaries
- **Platform-Specific**:
  - Automatic library linking: `-pthread` (all), `-lkvm -ldevstat` (FreeBSD), `-framework IOKit -framework CoreFoundation` (macOS), `-lkvm` (OpenBSD), `-lkvm -lprop` (NetBSD)
  - GNU make detection and enforcement on BSD
  - Thread count detection for parallel builds

#### CMake (Community Maintained, Alternative)
- **Minimum Version**: 3.25
- **Options**:
  - `BTOP_STATIC=ON|OFF` - Static linking
  - `BTOP_LTO=ON|OFF` - Link-time optimization (default: ON for Release builds)
  - `BTOP_GPU=ON|OFF` - GPU support
  - `BTOP_RSMI_STATIC=ON|OFF` - Static ROCm SMI
  - `BUILD_TESTING=ON|OFF` - Build tests (requires GoogleTest)
  - `CMAKE_INSTALL_PREFIX=<path>` - Installation prefix
- **Targets**:
  - `cmake -B build -G Ninja`
  - `cmake --build build`
  - `cmake --install build`
  - `ctest --test-dir build` (if tests enabled)
- **Features**:
  - Automatic platform detection
  - C++23 feature checks (optional monads, expected, ranges, string::contains)
  - Security flags (stack protector, clash protection, CFI)
  - Build info generation (git commit, compiler version)
  - LTO support with compiler detection

### 2.14 Dependencies

#### Required
- **C++ Standard Library**: Full C++23 support
  - `std::ranges`, `std::expected`, `std::optional::and_then`, `std::string::contains`, `std::ranges::to`
- **pthread**: POSIX threads
- **Terminal**: ANSI escape sequence support

#### Bundled (Header-Only)
- **fmt**: String formatting library (included in `include/fmt/`)
- **widechar_width.hpp**: UTF-8 wide character width calculation

#### Platform-Specific
- **Linux**: None required, optional for GPU:
  - `libnvidia-ml.so` (NVIDIA driver package)
  - `librocm_smi64.so` (ROCm, or statically linked with `RSMI_STATIC=true`)
  - Intel GPU Top (bundled C code, requires C compiler)
- **macOS**: IOKit, CoreFoundation frameworks
- **FreeBSD**: `libkvm`, `libdevstat`, `libelf` (static builds)
- **OpenBSD**: `libkvm`
- **NetBSD**: `libkvm`, `libprop`

#### Build-Time Only
- **GNU Coreutils**: `sed`, `date`, `cut`, `grep` (for Makefile)
- **lowdown**: Man page generation (optional)
- **git**: For version information (optional)

### 2.15 Security Considerations

#### Privilege Escalation
- **setcap** (preferred): `CAP_SYS_NICE` for signal sending, `CAP_PERFMON` + `CAP_DAC_READ_SEARCH` for Intel GPU and CPU wattage
- **setuid**: Run as root user (less secure, used when setcap unavailable)
- **Rationale**: 
  - Signal sending to any process without root
  - Reading SYSFS/RAPL power interfaces (Intel GPU, CPU wattage)
  - Accessing certain /proc files

#### Hardening
- Stack protectors: `-fstack-protector`, `-fstack-clash-protection`
- Control-flow integrity: `-fcf-protection`
- Assertions: `-D_GLIBCXX_ASSERTIONS`, `-D_LIBCPP_HARDENING_MODE=_LIBCPP_HARDENING_MODE_DEBUG`
- No network operations
- Limited file system access (read-only except for config/log)
- Input validation on all external data

#### Sensitive Information
- Never log passwords or secrets
- User enumeration limited to visible processes
- LDAP username issue in static glibc builds (shows UID instead of username)

### 2.16 Terminal Compatibility

#### Required Terminal Features
- **Escape Sequences**: ANSI/VT100 control sequences
- **Colors**: 
  - Best: 24-bit truecolor (`ESC[38;2;R;G;Bm`)
  - Good: 256-color palette (`ESC[38;5;Nm`)
  - Minimal: 16-color ANSI
- **Unicode**: UTF-8 encoding with wide character support
- **Cursor Control**: Position, visibility, save/restore
- **Alternative Screen**: Optional but recommended
- **Synchronized Output**: Optional but reduces flicker (`ESC[?2026h/l`)
- **Mouse Tracking**: SGR mode (`ESC[?1002h`, `ESC[?1006h`)

#### Tested Terminals
- Kitty, Alacritty, WezTerm, iTerm2, GNOME Terminal, Konsole, Yakuake, Windows Terminal, etc.
- TTY/console mode for systems without graphical terminals

#### Known Issues
- Braille characters require specific font support
- Some web-based terminals have wide character rendering issues
- Konsole/Yakuake may need "Bi-Directional text rendering" disabled

---

## 3. Detailed Requirements for Reproduction

### 3.1 Project Setup

#### Directory Structure
```
btop/
├── src/
│   ├── btop.cpp, btop.hpp              # Main application
│   ├── btop_shared.cpp, btop_shared.hpp # Shared interfaces
│   ├── btop_config.cpp, btop_config.hpp # Configuration
│   ├── btop_theme.cpp, btop_theme.hpp   # Theme system
│   ├── btop_draw.cpp, btop_draw.hpp     # Drawing primitives
│   ├── btop_input.cpp, btop_input.hpp   # Input handling
│   ├── btop_menu.cpp, btop_menu.hpp     # Menu system
│   ├── btop_tools.cpp, btop_tools.hpp   # Utilities
│   ├── btop_log.cpp, btop_log.hpp       # Logging
│   ├── btop_cli.cpp, btop_cli.hpp       # CLI parsing
│   ├── main.cpp                         # Entry point wrapper
│   ├── config.h.in                      # CMake template
│   ├── linux/
│   │   ├── btop_collect.cpp             # Linux data collection
│   │   └── intel_gpu_top/               # Intel GPU monitoring
│   │       ├── intel_gpu_top.c, .h
│   │       ├── igt_perf.c, .h
│   │       ├── intel_device_info.c, .h
│   │       └── CMakeLists.txt
│   ├── osx/
│   │   ├── btop_collect.cpp             # macOS data collection
│   │   ├── sensors.cpp, sensors.hpp
│   │   └── smc.cpp, smc.hpp
│   ├── freebsd/
│   │   └── btop_collect.cpp
│   ├── openbsd/
│   │   ├── btop_collect.cpp
│   │   └── sysctlbyname.cpp
│   └── netbsd/
│       └── btop_collect.cpp
├── include/
│   ├── fmt/                             # fmt library (header-only)
│   └── widechar_width.hpp
├── themes/
│   ├── nord.theme
│   ├── dracula.theme
│   └── ... (40+ themes)
├── Img/                                 # Screenshots and icons
├── cmake/                               # CMake find modules
│   ├── Finddevstat.cmake
│   ├── Findelf.cmake
│   ├── Findkvm.cmake
│   └── Findproplib.cmake
├── tests/                               # Unit tests (GoogleTest)
│   └── tools.cpp
├── snap/                                # Snap packaging
│   └── snapcraft.yaml
├── .github/                             # GitHub Actions CI/CD
│   └── workflows/
│       ├── continuous-build-linux.yml
│       ├── continuous-build-macos.yml
│       ├── continuous-build-freebsd.yml
│       ├── continuous-build-openbsd.yml
│       └── continuous-build-netbsd.yml
├── Makefile                             # Primary build system
├── CMakeLists.txt                       # Alternative build system
├── btop.desktop                         # Desktop entry file
├── manpage.md                           # Man page source
├── README.md
├── CHANGELOG.md
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── LICENSE                              # Apache 2.0
└── AGENTS.md                            # AI agent guidance
```

### 3.2 Code Style and Standards

#### Formatting
- **Indentation**: Tabs (not spaces)
- **Tab Size**: 4 spaces
- **Standard**: C++23
- **Line Width**: No strict limit, but keep reasonable
- **Braces**: Opening `{` on same line as statement
- **Operators**: Use alternative operators: `and`, `or`, `not` instead of `&&`, `||`, `!`

#### Naming Conventions
- **Namespaces**: PascalCase (`Cpu`, `Config`, `Tools`)
- **Classes**: PascalCase (`TextEdit`, `Graph`, `Meter`)
- **Functions**: snake_case (`collect`, `get_cpuHz`, `system_uptime`)
- **Variables**: snake_case (`cpu_info`, `update_ms`, `shown_boxes`)
- **Constants**: snake_case with descriptive names
- **Atomics**: snake_case (`active`, `quitting`, `resized`)

#### Comments
- Use `//` for single-line comments
- Use `/**/` for multi-line comments
- Doxygen-style comments for public APIs: `//*` or `//? `
- License header at top of each file

#### RAII Principles
- Use smart pointers where applicable (`std::unique_ptr`, `std::shared_ptr`)
- RAII classes for resource management (`atomic_lock`, `DebugTimer`, `KvmPtr`, `IfAddrsPtr`)
- Avoid manual memory management

#### STL Usage
- Prefer `std::ranges` algorithms over raw loops
- Use `std::string_view` for read-only string parameters
- Use `std::optional` for nullable values
- Use `std::expected` for error handling (C++23)
- Use `std::format` or `fmt::format` for string formatting

### 3.3 Build Process

#### Compilation Flags
```bash
# Required
-std=c++23
-pthread
-DFMT_HEADER_ONLY
-D_GLIBCXX_ASSERTIONS
-D_LIBCPP_HARDENING_MODE=_LIBCPP_HARDENING_MODE_DEBUG
-D_FILE_OFFSET_BITS=64

# Optimization
-O2                         # Release
-flto=thin (Clang) or -flto=<threads> (GCC)

# Warnings
-Wall -Wextra -pedantic

# Hardening
-fstack-protector
-fstack-clash-protection
-fcf-protection

# Optional
-DBTOP_DEBUG                # Debug logging
-DGPU_SUPPORT               # GPU monitoring
-DSTATIC_BUILD              # Static linking marker
```

#### Platform-Specific Compilation
```bash
# Linux
g++ -std=c++23 src/*.cpp src/linux/*.cpp -o btop -pthread ...

# macOS
clang++ -std=c++23 src/*.cpp src/osx/*.cpp -o btop -framework IOKit -framework CoreFoundation ...

# FreeBSD
g++14 -std=c++23 src/*.cpp src/freebsd/*.cpp -o btop -lkvm -ldevstat ...

# OpenBSD
clang++ -std=c++23 src/*.cpp src/openbsd/*.cpp -o btop -lkvm -static-libstdc++ ...

# NetBSD
g++14 -std=c++23 src/*.cpp src/netbsd/*.cpp -o btop -lkvm -lprop ...
```

#### GPU Support Compilation (Linux)
```bash
# Compile with GPU support
make GPU_SUPPORT=true

# Static link ROCm SMI (AMD)
make GPU_SUPPORT=true RSMI_STATIC=true

# Note: Requires ROCm SMI source in lib/rocm_smi_lib/
git clone https://github.com/rocm/rocm_smi_lib.git --depth 1 -b rocm-5.6.x lib/rocm_smi_lib
```

### 3.4 Implementation Sequence Recommendation

To reproduce btop++, implement in this order:

1. **Foundation** (Week 1-2)
   - Set up project structure and build system
   - Implement `btop_tools.cpp` (utilities first)
   - Implement `btop_log.cpp` (logging)
   - Implement terminal initialization (`Term::init()`, `Term::restore()`)
   - Basic signal handling

2. **Configuration** (Week 2-3)
   - Implement `btop_config.cpp` (parsing, validation)
   - Create default configuration template
   - Implement config file I/O

3. **Theme System** (Week 3)
   - Implement `btop_theme.cpp` (color parsing, gradients)
   - Port or create 3-5 basic themes
   - Theme loading and application

4. **Drawing Primitives** (Week 3-4)
   - Implement `btop_draw.cpp` base classes:
     - `Meter` class
     - `Graph` class (start with block symbols)
     - `TextEdit` class
   - Implement `createBox()`, `calcSizes()`
   - Banner generation

5. **Input System** (Week 4)
   - Implement `btop_input.cpp` (polling, keyboard, mouse)
   - Key mapping and processing
   - Mouse click region tracking

6. **Data Collection - Stage 1** (Week 5-6)
   - Implement Linux `Cpu::collect()` (basic /proc/stat parsing)
   - Implement Linux `Mem::collect()` (basic /proc/meminfo parsing)
   - Implement Linux `Net::collect()` (basic /proc/net/dev parsing)
   - Implement Linux `Proc::collect()` (basic process enumeration)
   - Create data structures and historical deques

7. **Drawing Modules** (Week 6-7)
   - Implement `Cpu::draw()` (box, text, basic graphs)
   - Implement `Mem::draw()` (box, meters, disk list)
   - Implement `Net::draw()` (box, graphs, interface info)
   - Implement `Proc::draw()` (box, process list)

8. **Main Application Loop** (Week 7-8)
   - Implement `btop_main()` entry point
   - Implement main event loop
   - Implement runner thread (`Runner::run()`)
   - Coordinate data collection and drawing
   - Terminal resize handling

9. **Menu System** (Week 8-9)
   - Implement `btop_menu.cpp` (base msgBox class)
   - Implement help menu
   - Implement options menu
   - Implement signal/renice menus

10. **Advanced Features - CPU** (Week 9-10)
    - Temperature sensors scanning
    - Frequency monitoring
    - Battery support
    - Power consumption (RAPL)
    - Per-core graphs

11. **Advanced Features - Memory** (Week 10-11)
    - Disk I/O statistics
    - ZFS support
    - Disk filtering
    - I/O mode with graphs

12. **Advanced Features - Process** (Week 11-12)
    - Tree view implementation
    - Process filtering
    - Detailed process info
    - Signal sending
    - Process priority changing
    - Process following

13. **GPU Support** (Week 13-14, Linux only)
    - NVIDIA NVML integration
    - AMD ROCm SMI integration
    - Intel GPU Top integration
    - GPU draw functions

14. **Platform Ports** (Week 15-20)
    - macOS support (Cpu, Mem, Net, Proc)
    - FreeBSD support
    - OpenBSD support
    - NetBSD support

15. **Polish and Testing** (Week 21-24)
    - Performance optimization
    - Memory leak detection
    - Edge case handling
    - Cross-platform testing
    - Documentation
    - Packaging

### 3.5 Critical Implementation Details

#### Terminal Initialization
```cpp
// Must set non-canonical mode with VMIN=VTIME=0
struct termios term;
tcgetattr(STDIN_FILENO, &term);
term.c_lflag &= ~(ICANON | ECHO);
term.c_cc[VMIN] = 0;
term.c_cc[VTIME] = 0;
tcsetattr(STDIN_FILENO, TCSANOW, &term);
```

#### Thread Synchronization
```cpp
// Use atomics with proper memory ordering
// No mutexes needed - single-writer, single-reader pattern
atomic<bool> Runner::active{false};
atomic<bool> Runner::reading{false};

// In runner thread
void Runner::run() {
    while (not stopping) {
        active = true;
        reading = true;
        
        // Collect data
        auto& cpu = Cpu::collect();
        auto& mem = Mem::collect();
        // ...
        
        reading = false;
        active = false;
        redraw = true;
        
        sleep_until(next_update_time);
    }
}
```

#### Graph Data Management
```cpp
// Maintain fixed-size deques for historical data
// Size determined by graph width
deque<long long> cpu_total;
const size_t max_size = graph_width;

// On collect
cpu_total.push_back(current_percentage);
if (cpu_total.size() > max_size) {
    cpu_total.pop_front();
}

// Graph rendering uses this deque
graph(cpu_total);
```

#### Platform Detection Pattern
```cpp
#if defined(__linux__)
    // Linux-specific code
#elif defined(__APPLE__)
    // macOS-specific code
#elif defined(__FreeBSD__)
    // FreeBSD-specific code
#elif defined(__OpenBSD__)
    // OpenBSD-specific code
#elif defined(__NetBSD__)
    // NetBSD-specific code
#else
    #error "Unsupported platform"
#endif
```

#### Color Escape Sequence Generation
```cpp
// 24-bit truecolor
string fg = fmt::format("\x1b[38;2;{};{};{}m", r, g, b);
string bg = fmt::format("\x1b[48;2;{};{};{}m", r, g, b);

// 256-color fallback
int color_256 = 16 + 36 * (r / 51) + 6 * (g / 51) + (b / 51);
string fg_256 = fmt::format("\x1b[38;5;{}m", color_256);

// 16-color fallback (approximate to ANSI colors)
```

#### Process Tree Construction
```cpp
// Build parent-child relationships
unordered_map<pid_t, vector<proc_info*>> children;
for (auto& proc : proc_list) {
    children[proc.ppid].push_back(&proc);
}

// Recursive tree generation
void build_tree(proc_info* parent, int depth) {
    parent->depth = depth;
    parent->prefix = generate_prefix(depth);
    for (auto* child : children[parent->pid]) {
        build_tree(child, depth + 1);
    }
}
```

#### Mouse Event Parsing
```cpp
// SGR mouse format: ESC[<b;x;y;M or m
// b = button, x = column, y = row, M = press, m = release
regex mouse_regex("\033\\[<(\\d+);(\\d+);(\\d+);([Mm])");
smatch match;
if (regex_search(input, match, mouse_regex)) {
    int button = stoi(match[1]);
    int x = stoi(match[2]);
    int y = stoi(match[3]);
    bool pressed = (match[4] == "M");
    
    // Check mouse mappings for clicked region
    for (auto& [name, region] : mouse_mappings) {
        if (x >= region.col && x < region.col + region.width &&
            y >= region.line && y < region.line + region.height) {
            // Handle click on this region
        }
    }
}
```

---

## 4. Testing Strategy

### 4.1 Unit Tests
- Test utility functions in `btop_tools.cpp`
- Test configuration parsing edge cases
- Test theme color parsing
- Test string manipulation (UTF-8, wide characters)
- Framework: GoogleTest (CMake builds only)

### 4.2 Integration Tests
- Test data collection on each platform
- Test UI rendering at various terminal sizes
- Test input processing (keyboard and mouse)
- Test menu navigation
- Test process tree generation

### 4.3 Performance Tests
- Measure CPU overhead at various update intervals
- Measure memory footprint
- Test with thousands of processes
- Test with large /proc filesystems

### 4.4 Platform Tests
- Continuous integration on Linux, macOS, FreeBSD, OpenBSD, NetBSD
- Test on various terminal emulators
- Test in TTY mode
- Test with different locales

### 4.5 Stress Tests
- Rapid terminal resizing
- Rapid box toggling
- Process list filtering with many processes
- Network graphs with high throughput
- Long-running sessions (memory leaks)

---

## 5. Documentation Requirements

### 5.1 User Documentation
- README with features, installation, compilation
- Man page with command-line options
- Help menu with keyboard shortcuts
- In-app options menu with descriptions

### 5.2 Developer Documentation
- CONTRIBUTING.md with coding standards
- AGENTS.md for AI assistance guidance
- Code comments explaining non-obvious logic
- Architecture overview (this document)

### 5.3 Operational Documentation
- Config file comments
- Theme file format specification
- Log file location and format
- Troubleshooting guide (in README)

---

## 6. Deployment and Distribution

### 6.1 Packaging
- **Source tarballs**: Released on GitHub
- **Binary releases**: Statically compiled for multiple architectures (x86_64, i686, ARM, RISC-V)
- **Package managers**: 
  - Debian/Ubuntu: PPA or .deb
  - Fedora/RHEL: RPM
  - Arch: AUR
  - FreeBSD: ports
  - macOS: Homebrew
  - Snap: btop and btop-desktop
- **Desktop integration**: .desktop file for graphical launchers

### 6.2 Installation Paths
- **Binary**: `$PREFIX/bin/btop` (default: `/usr/local/bin/btop`)
- **Themes**: `$PREFIX/share/btop/themes/`
- **Config**: `$XDG_CONFIG_HOME/btop/` or `$HOME/.config/btop/`
- **Log**: `$XDG_STATE_HOME/btop/` or `$HOME/.local/state/btop/`
- **Man page**: `$PREFIX/share/man/man1/btop.1`
- **Desktop file**: `$PREFIX/share/applications/btop.desktop`
- **Icon**: `$PREFIX/share/icons/hicolor/scalable/apps/btop.svg`

### 6.3 Permissions
- **Default**: Runs as regular user
- **Optional setcap**: `sudo make setcap` for extended features
- **Optional setuid**: `sudo make setuid` (less secure alternative)

---

## 7. Future Enhancements (Out of Scope for Reproduction)

- Windows native support (btop4win is a separate project)
- Disk smart monitoring
- GPU process list
- Remote monitoring capability
- Plugin system
- More graph types (area, stacked)
- Customizable layouts
- Export/screenshot functionality
- Historical data recording

---

## 8. Known Limitations and Gotchas

### 8.1 Platform Limitations
- GPU support only on Linux x86_64
- LDAP usernames show as UIDs in static glibc builds
- macOS needs Homebrew or MacPorts for dependencies
- BSD systems require gmake (GNU make)

### 8.2 Terminal Limitations
- Braille graphs require font support
- Wide character rendering issues in some terminals
- Bi-directional text rendering may cause misalignment (Konsole/Yakuake)
- Web-based terminals may have issues

### 8.3 Privilege Limitations
- Intel GPU monitoring requires setcap/setuid
- CPU wattage monitoring requires setcap/setuid
- Signal sending to other users' processes requires setcap/setuid

### 8.4 Performance Considerations
- `/proc/[pid]/smaps` parsing is very slow (optional)
- Update intervals <1000ms can cause high CPU usage
- Large process lists (>1000) may impact performance
- Network graphs auto-scale down to 10 KiB minimum

---

## 9. Version History Context

- **v1.0.0** (Sept 2021): Initial Linux release
- **v1.1.0** (Nov 2021): macOS support added
- **v1.2.0** (Jan 2022): FreeBSD support added
- **v1.3.0** (Jan 2024): GPU support (Linux) and OpenBSD support added
- **v1.4.0** (Sept 2024): Intel GPU support and NetBSD support added
- **v1.4.6** (Current): Latest stable release

---

## 10. Critical Success Factors

For a successful reproduction of btop++, ensure:

1. **C++23 Compliance**: Strict adherence to modern C++ standards
2. **Platform Abstraction**: Clean separation of platform-specific code
3. **Performance**: Minimal overhead, efficient data structures
4. **Terminal Handling**: Robust escape sequence generation and parsing
5. **Thread Safety**: Proper use of atomics without race conditions
6. **UTF-8 Support**: Correct handling of wide characters
7. **Responsive UI**: Sub-second input handling, smooth animations
8. **Error Handling**: Graceful degradation when features unavailable
9. **Code Quality**: Follow existing style, RAII principles, STL idioms
10. **Testing**: Comprehensive testing across platforms and terminals

---

## Appendix A: Configuration Options Reference

(See section 1.3 for full list, or run `btop --default-config`)

## Appendix B: Keyboard Shortcuts Reference

| Key | Action |
|-----|--------|
| `q`, `Ctrl+C`, `Ctrl+D` | Quit |
| `F1`, `h` | Help menu |
| `F2`, `o` | Options menu |
| `F3`, `f` | Filter processes |
| `F5`, `t` | Toggle tree view |
| `F6` | Select sorting column |
| `F9`, `k` | Send signal (with Shift in vim mode) |
| `Up`, `Down` | Navigate process list |
| `PgUp`, `PgDn` | Scroll process list |
| `Home`, `End` (`g`, `G` in vim mode) | Top/bottom of list |
| `Enter` | Show detailed process info |
| `Backspace`, `ESC` | Close details/menus |
| `Tab` | Next sort column |
| `Space` | Pause/unpause |
| `c` | Sort by CPU |
| `m` | Sort by memory |
| `p` | Sort by PID |
| `n` | Sort by name |
| `r` | Reverse sort |
| `e` | Toggle CPU graph upper stats |
| `s` | Toggle CPU graph lower stats |
| `d` | Toggle disks in memory box |
| `z` | Toggle net auto scaling |
| `a` | Toggle net sync scaling |
| `1-4` | Toggle CPU/Mem/Net/Proc boxes |
| `5-0` | Toggle GPU boxes (if enabled) |
| `Shift+1-9` | Load preset |
| `+`, `-` | Increment/decrement update time |
| `Left`, `Right` | Scroll process list horizontally |

(See `F1` menu in app for complete list)

## Appendix C: File Locations

| Item | Path |
|------|------|
| Config | `$XDG_CONFIG_HOME/btop/btop.conf` or `$HOME/.config/btop/btop.conf` |
| User Themes | `$XDG_CONFIG_HOME/btop/themes/` or `$HOME/.config/btop/themes/` |
| System Themes | `../share/btop/themes/`, `/usr/local/share/btop/themes/`, `/usr/share/btop/themes/` |
| Log | `$XDG_STATE_HOME/btop/btop.log` or `$HOME/.local/state/btop/btop.log` |
| Binary | `/usr/local/bin/btop` (or per $PREFIX) |

---

## Conclusion

This document provides a complete specification for reproducing btop++ from scratch. The architecture is designed for:
- **Modularity**: Clear separation of concerns
- **Performance**: Minimal overhead, efficient algorithms
- **Portability**: Platform-specific code isolated to collectors
- **Maintainability**: Clean code, RAII, modern C++
- **Extensibility**: Easy to add new features or platforms

By following this specification and the recommended implementation sequence, a functionally equivalent btop++ can be created. The key is maintaining the threaded architecture, proper terminal handling, and platform abstraction while implementing each module incrementally.
