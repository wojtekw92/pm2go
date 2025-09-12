# D-Bus Extension Design for PM2go

This document outlines the design for extending PM2go with native systemd D-Bus API support as an alternative to the current command-based approach.

## Overview

Currently, PM2go uses `exec.Command()` to call `systemctl` binaries. While this works reliably, it has performance and feature limitations. The D-Bus extension would provide:

- **Direct IPC communication** with systemd daemon
- **Real-time monitoring** capabilities  
- **Better error handling** with structured responses
- **Performance improvements** by eliminating process forking
- **Advanced features** like property watching and atomic operations

## Architecture

### Current Architecture
```
PM2go CLI → exec.Command("systemctl") → systemctl binary → D-Bus → systemd
```

### Proposed Architecture  
```
PM2go CLI → go-systemd library → D-Bus → systemd
```

### Hybrid Architecture (Recommended)
```
PM2go CLI → Manager Interface → [D-Bus API | Command Fallback] → systemd
```

## Dependencies

### Primary Library: CoreOS go-systemd
```go
// Add to go.mod
github.com/coreos/go-systemd/v22 v22.5.0
```

### Key Packages:
- `github.com/coreos/go-systemd/v22/dbus` - D-Bus communication
- `github.com/coreos/go-systemd/v22/unit` - Unit file manipulation  
- `github.com/coreos/go-systemd/v22/journal` - Journal integration

## Interface Design

### Manager Interface
```go
// pkg/systemd/interface.go
package systemd

import "context"

// SystemdBackend defines the interface for systemd operations
type SystemdBackend interface {
    // Service lifecycle
    StartUnit(ctx context.Context, name string) error
    StopUnit(ctx context.Context, name string) error  
    RestartUnit(ctx context.Context, name string) error
    EnableUnit(ctx context.Context, name string) error
    DisableUnit(ctx context.Context, name string) error
    
    // Service information
    ListUnits(ctx context.Context) ([]UnitInfo, error)
    GetUnitStatus(ctx context.Context, name string) (*UnitStatus, error)
    GetUnitProperties(ctx context.Context, name string) (map[string]interface{}, error)
    
    // System operations
    Reload(ctx context.Context) error
    
    // Monitoring (D-Bus only)
    WatchUnits(ctx context.Context) (<-chan UnitEvent, error)
    
    // Cleanup
    Close() error
}

// UnitInfo represents basic unit information
type UnitInfo struct {
    Name        string
    LoadState   string
    ActiveState string
    SubState    string
    Description string
    MainPID     uint32
}

// UnitStatus represents detailed unit status
type UnitStatus struct {
    Name         string
    LoadState    string
    ActiveState  string
    SubState     string
    MainPID      uint32
    ExecMainPID  uint32
    ActiveEnterTimestamp uint64
    Properties   map[string]interface{}
}

// UnitEvent represents unit state changes (D-Bus only)
type UnitEvent struct {
    UnitName    string
    ActiveState string
    SubState    string
    Timestamp   time.Time
}
```

### Manager Implementation
```go
// pkg/systemd/manager.go
package systemd

type Manager struct {
    backend SystemdBackend
    userMode bool
    prefix   string
}

func NewManager() *Manager {
    userMode := os.Getuid() != 0
    
    // Try D-Bus first, fallback to command
    var backend SystemdBackend
    if dbusBackend, err := NewDBusBackend(userMode); err == nil {
        backend = dbusBackend
    } else {
        backend = NewCommandBackend(userMode)
    }
    
    return &Manager{
        backend:  backend,
        userMode: userMode,
        prefix:   "pm2-",
    }
}

func (m *Manager) Start(config AppConfig) error {
    ctx := context.Background()
    serviceName := m.serviceName(config.Name)
    
    // Generate and write service file (same as current)
    if err := m.writeServiceFile(config); err != nil {
        return err
    }
    
    // Use backend for systemd operations
    if err := m.backend.Reload(ctx); err != nil {
        return fmt.Errorf("failed to reload systemd: %v", err)
    }
    
    if err := m.backend.StartUnit(ctx, serviceName+".service"); err != nil {
        return fmt.Errorf("failed to start service: %v", err)
    }
    
    if err := m.backend.EnableUnit(ctx, serviceName+".service"); err != nil {
        return fmt.Errorf("failed to enable service: %v", err)
    }
    
    return nil
}
```

## Backend Implementations

### D-Bus Backend
```go
// pkg/systemd/dbus_backend.go
package systemd

import (
    "context"
    "github.com/coreos/go-systemd/v22/dbus"
)

type DBusBackend struct {
    conn *dbus.Conn
    userMode bool
}

func NewDBusBackend(userMode bool) (*DBusBackend, error) {
    var conn *dbus.Conn
    var err error
    
    if userMode {
        conn, err = dbus.NewUserConnection()
    } else {
        conn, err = dbus.NewSystemdConnection()
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to connect to D-Bus: %v", err)
    }
    
    return &DBusBackend{
        conn: conn,
        userMode: userMode,
    }, nil
}

func (d *DBusBackend) StartUnit(ctx context.Context, name string) error {
    _, err := d.conn.StartUnitContext(ctx, name, "replace", nil)
    return err
}

func (d *DBusBackend) StopUnit(ctx context.Context, name string) error {
    _, err := d.conn.StopUnitContext(ctx, name, "replace", nil)
    return err
}

func (d *DBusBackend) ListUnits(ctx context.Context) ([]UnitInfo, error) {
    units, err := d.conn.ListUnitsContext(ctx)
    if err != nil {
        return nil, err
    }
    
    var result []UnitInfo
    for _, unit := range units {
        result = append(result, UnitInfo{
            Name:        unit.Name,
            LoadState:   unit.LoadState,
            ActiveState: unit.ActiveState, 
            SubState:    unit.SubState,
            Description: unit.Description,
            MainPID:     unit.MainPID,
        })
    }
    return result, nil
}

func (d *DBusBackend) GetUnitProperties(ctx context.Context, name string) (map[string]interface{}, error) {
    return d.conn.GetUnitPropertiesContext(ctx, name)
}

func (d *DBusBackend) WatchUnits(ctx context.Context) (<-chan UnitEvent, error) {
    // Subscribe to unit state changes
    updates, err := d.conn.SubscribeUnits(1 * time.Second)
    if err != nil {
        return nil, err
    }
    
    events := make(chan UnitEvent, 10)
    
    go func() {
        defer close(events)
        for {
            select {
            case <-ctx.Done():
                return
            case update := <-updates:
                if update != nil {
                    for unit, status := range update {
                        events <- UnitEvent{
                            UnitName:    unit,
                            ActiveState: status.ActiveState,
                            SubState:    status.SubState,
                            Timestamp:   time.Now(),
                        }
                    }
                }
            }
        }
    }()
    
    return events, nil
}

func (d *DBusBackend) Reload(ctx context.Context) error {
    return d.conn.ReloadContext(ctx)
}

func (d *DBusBackend) Close() error {
    if d.conn != nil {
        d.conn.Close()
    }
    return nil
}
```

### Command Backend (Current Implementation)
```go
// pkg/systemd/command_backend.go
package systemd

type CommandBackend struct {
    userMode bool
}

func NewCommandBackend(userMode bool) *CommandBackend {
    return &CommandBackend{userMode: userMode}
}

func (c *CommandBackend) StartUnit(ctx context.Context, name string) error {
    cmd := []string{"systemctl"}
    if c.userMode {
        cmd = append(cmd, "--user")
    }
    cmd = append(cmd, "start", name)
    
    return exec.CommandContext(ctx, cmd[0], cmd[1:]...).Run()
}

// ... implement other methods similarly to current code

func (c *CommandBackend) WatchUnits(ctx context.Context) (<-chan UnitEvent, error) {
    return nil, fmt.Errorf("real-time monitoring not available with command backend")
}
```

## Enhanced Features

### Real-time Monitoring
```go
// Example usage for web API
func (api *WebAPI) StreamUnitEvents(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    events, err := api.manager.WatchUnits(ctx)
    if err != nil {
        http.Error(w, "Monitoring not available", http.StatusServiceUnavailable)
        return
    }
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    for {
        select {
        case <-ctx.Done():
            return
        case event := <-events:
            fmt.Fprintf(w, "data: %s\n\n", toJSON(event))
            if f, ok := w.(http.Flusher); ok {
                f.Flush()
            }
        }
    }
}
```

### Advanced Unit Operations
```go
// Enhanced service management
func (m *Manager) RestartUnit(name string) error {
    ctx := context.Background()
    serviceName := m.serviceName(name) + ".service"
    return m.backend.RestartUnit(ctx, serviceName)
}

func (m *Manager) GetDetailedStatus(name string) (*DetailedStatus, error) {
    ctx := context.Background()
    serviceName := m.serviceName(name) + ".service"
    
    props, err := m.backend.GetUnitProperties(ctx, serviceName)
    if err != nil {
        return nil, err
    }
    
    return &DetailedStatus{
        Name:             name,
        ActiveState:      props["ActiveState"].(string),
        SubState:         props["SubState"].(string),
        MainPID:          props["MainPID"].(uint32),
        ExecMainPID:      props["ExecMainPID"].(uint32),
        MemoryCurrent:    props["MemoryCurrent"].(uint64),
        CPUUsageNSec:     props["CPUUsageNSec"].(uint64),
        ActiveEnterTime:  time.Unix(int64(props["ActiveEnterTimestamp"].(uint64)/1000000), 0),
        LoadState:        props["LoadState"].(string),
        UnitFileState:    props["UnitFileState"].(string),
        Description:      props["Description"].(string),
        ExecStart:        extractExecStart(props["ExecStart"]),
        Environment:      extractEnvironment(props["Environment"]),
        WorkingDirectory: props["WorkingDirectory"].(string),
    }, nil
}
```

### Performance Benchmarks
```go
// Benchmark comparison
func BenchmarkStartUnit(b *testing.B) {
    // D-Bus backend
    dbusManager := NewManagerWithBackend(NewDBusBackend(true))
    b.Run("D-Bus", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            dbusManager.Start(testConfig)
            dbusManager.Stop(testConfig.Name)
        }
    })
    
    // Command backend  
    cmdManager := NewManagerWithBackend(NewCommandBackend(true))
    b.Run("Command", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            cmdManager.Start(testConfig)
            cmdManager.Stop(testConfig.Name)
        }
    })
}
```

## Migration Strategy

### Phase 1: Interface Abstraction
1. Extract current command-based code into `CommandBackend`
2. Create `SystemdBackend` interface
3. Update `Manager` to use interface
4. Ensure full backward compatibility

### Phase 2: D-Bus Implementation
1. Add `coreos/go-systemd` dependency
2. Implement `DBusBackend` 
3. Add hybrid selection logic
4. Extensive testing on various Linux distros

### Phase 3: Enhanced Features  
1. Add real-time monitoring
2. Implement advanced property queries
3. Add performance optimizations
4. Create web API examples

### Phase 4: Documentation & Examples
1. Update README with D-Bus benefits
2. Add performance benchmarks
3. Create web API integration guide
4. Add troubleshooting for D-Bus issues

## Configuration

### Environment Variables
```bash
# Force specific backend
PM2GO_BACKEND=dbus     # Force D-Bus (fail if unavailable)  
PM2GO_BACKEND=command  # Force command execution
PM2GO_BACKEND=auto     # Auto-detect (default)

# D-Bus connection timeout
PM2GO_DBUS_TIMEOUT=30s

# Enable monitoring features
PM2GO_ENABLE_MONITORING=true
```

### Runtime Detection
```go
func detectBestBackend(userMode bool) SystemdBackend {
    // Try D-Bus first
    if backend, err := NewDBusBackend(userMode); err == nil {
        log.Printf("Using D-Bus backend for systemd communication")
        return backend
    }
    
    // Fallback to command
    log.Printf("D-Bus unavailable, using command backend") 
    return NewCommandBackend(userMode)
}
```

## Error Handling

### D-Bus Specific Errors
```go
// D-Bus error mapping
func mapDBusError(err error) error {
    if dbusErr, ok := err.(dbus.Error); ok {
        switch dbusErr.Name {
        case "org.freedesktop.systemd1.NoSuchUnit":
            return ErrUnitNotFound
        case "org.freedesktop.systemd1.UnitExists": 
            return ErrUnitExists
        case "org.freedesktop.DBus.Error.AccessDenied":
            return ErrPermissionDenied
        default:
            return fmt.Errorf("systemd D-Bus error: %v", dbusErr)
        }
    }
    return err
}
```

### Fallback Logic
```go
func (m *Manager) StartWithFallback(config AppConfig) error {
    err := m.backend.StartUnit(context.Background(), serviceName)
    
    // If D-Bus fails, try falling back to command
    if err != nil && m.backend.Type() == "dbus" {
        log.Printf("D-Bus start failed, trying command fallback: %v", err)
        cmdBackend := NewCommandBackend(m.userMode)
        return cmdBackend.StartUnit(context.Background(), serviceName)
    }
    
    return err
}
```

## Testing Strategy

### Unit Tests
- Mock D-Bus connections for testing
- Test fallback scenarios  
- Verify interface compliance
- Error handling edge cases

### Integration Tests
- Test on multiple Linux distributions
- Verify D-Bus and command backends produce same results
- Test monitoring features
- Performance comparisons

### Load Tests
- Concurrent service operations
- Memory usage under load
- D-Bus connection pooling
- Monitoring event throughput

## Future Web API Integration

### Real-time Dashboard
```go
// WebSocket endpoint for real-time monitoring
func (api *API) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    
    ctx, cancel := context.WithCancel(r.Context())
    defer cancel()
    
    // Start monitoring
    events, err := api.manager.WatchUnits(ctx)
    if err != nil {
        conn.WriteJSON(map[string]string{"error": "monitoring unavailable"})
        return
    }
    
    for event := range events {
        if err := conn.WriteJSON(event); err != nil {
            break
        }
    }
}
```

### RESTful Service Management
```go
// Enhanced REST endpoints with D-Bus features
func (api *API) GetServiceDetails(w http.ResponseWriter, r *http.Request) {
    name := mux.Vars(r)["name"]
    
    // Get detailed properties via D-Bus
    details, err := api.manager.GetDetailedStatus(name)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    
    json.NewEncoder(w).Encode(details)
}
```

## Performance Benefits

### Expected Improvements
- **Startup time**: 60-80% faster (no process forking)
- **Memory usage**: 30-50% lower (no subprocess overhead)  
- **Error latency**: 90% faster (structured errors vs text parsing)
- **Monitoring**: Real-time events vs polling

### Benchmark Targets
```
Command Backend:  ~50ms per operation
D-Bus Backend:    ~5-10ms per operation  
Monitoring:       Real-time vs 1-second polling
Memory:           -20MB daemon overhead
```

## Risk Assessment

### Risks
- **Additional complexity**: D-Bus connection management
- **Dependency**: External library requirement
- **Compatibility**: D-Bus version differences across distros
- **Debugging**: D-Bus issues harder to troubleshoot than command errors

### Mitigations
- **Hybrid approach**: Always maintain command fallback
- **Graceful degradation**: Disable advanced features if D-Bus unavailable  
- **Extensive testing**: Test on major Linux distributions
- **Clear logging**: Detailed D-Bus connection and error logging

## Conclusion

The D-Bus extension would provide significant performance and feature improvements for PM2go, especially valuable for the planned web API. The hybrid approach ensures backward compatibility while enabling advanced features when available.

**Recommendation**: Implement in phases with the command backend as a reliable fallback, focusing on the web API use case where real-time monitoring and performance matter most.