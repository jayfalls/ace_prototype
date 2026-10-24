package cognitive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SensorDomain classifies whether a sensor plugin measures internal agent/compute vitals (Proprioceptive)
// or external physical/environmental/peripheral/behavioral signals (Exteroceptive).
type SensorDomain string

const (
	SensorProprioceptive SensorDomain = "PROPRIOCEPTIVE" // Self-referential: CPU, RAM, Disk, GPU VRAM, SQLite wait, LLM quota/cache
	SensorExteroceptive  SensorDomain = "EXTEROCEPTIVE"  // Externally-referential: Thermal/Temp, IIO (UV/Light/IR/Air/Pressure/Motion/Lidar), Power/Battery, Audio/Video/Peripherals, Access Control, Derived Human Circadian, Custom Plugins
)

// SensorReading is a standardized reading emitted by any pluggable sensor on the current host.
type SensorReading struct {
	SensorID  string            `json:"sensor_id"`
	Domain    SensorDomain      `json:"domain"`
	Modality  string            `json:"modality"` // compute, thermal, optical_uv_ir, air_env, power, audio_video, motion_lidar, radio_rf, access_ctrl, human_circadian, custom
	Summary   string            `json:"summary"`
	Values    map[string]any    `json:"values,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// PluggableSensor defines the system-agnostic interface for any Proprioceptive or Exteroceptive sensor.
// Each host/deployment only activates plugins whose ProbeAvailable(ctx) returns true.
type PluggableSensor interface {
	ID() string
	Domain() SensorDomain
	Modality() string
	ProbeAvailable(ctx context.Context) bool
	Read(ctx context.Context) (SensorReading, error)
}

// SensoriumSnapshot holds the aggregated Proprioceptive + Exteroceptive state for ACE v0.6 (§7).
type SensoriumSnapshot struct {
	LoadAvg1m           float64         `json:"load_avg_1m"`
	MemUsedMB           int64           `json:"mem_used_mb"`
	MemAvailMB          int64           `json:"mem_avail_mb"`
	DiskFreeMB          int64           `json:"disk_free_mb"`
	Goroutines          int             `json:"goroutines"`
	HumanProbActiveNow  float64         `json:"human_prob_active_now"`
	ExpectedReplySec    float64         `json:"expected_reply_sec"`
	ObservedHumanEvents int             `json:"observed_human_events"`
	CurrentHourUTC      int             `json:"current_hour_utc"`
	Readings            []SensorReading `json:"readings"`
}

// BuiltinPluggableSensors returns the default suite of system-agnostic Proprioceptive and Exteroceptive sensor plugins.
// Any custom sensors dropped into `/workspace/.harness_d/sensors/*.json` are also dynamically discovered and read.
func (e *Engine) BuiltinPluggableSensors() []PluggableSensor {
	return []PluggableSensor{
		&hostComputeProprioSensor{engine: e},
		&embodiedKinematicsIMUProprioSensor{engine: e},
		&sysfsThermalExteroSensor{},
		&iioEnvironmentalExteroSensor{},
		&powerSupplyExteroSensor{},
		&mediaPeripheralsExteroSensor{},
		&tactileGpsLidarReplicaExteroSensor{engine: e},
		&humanCircadianExteroSensor{engine: e},
		&workspaceCustomExteroSensor{engine: e},
	}
}

// 1b. Proprioceptive Sensor: Embodied Robotics / Kinematics / Vestibular IMU (Gyroscope, Accelerometer, Joint Encoders, Motor Torque)
type embodiedKinematicsIMUProprioSensor struct{ engine *Engine }

func (s *embodiedKinematicsIMUProprioSensor) ID() string           { return "proprio.embodied_kinematics_imu" }
func (s *embodiedKinematicsIMUProprioSensor) Domain() SensorDomain { return SensorProprioceptive }
func (s *embodiedKinematicsIMUProprioSensor) Modality() string     { return "imu_gyro_joints" }
func (s *embodiedKinematicsIMUProprioSensor) ProbeAvailable(_ context.Context) bool {
	if m, _ := filepath.Glob("/sys/bus/iio/devices/iio:device*/in_anglvel_*"); len(m) > 0 {
		return true
	}
	if s.engine != nil {
		wsDir, _ := s.engine.resolveSandboxDirs()
		if wsDir != "" {
			if m, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "proprio_robotics", "*.json")); len(m) > 0 {
				return true
			}
		}
	}
	return false
}
func (s *embodiedKinematicsIMUProprioSensor) Read(_ context.Context) (SensorReading, error) {
	var parts []string
	if m, _ := filepath.Glob("/sys/bus/iio/devices/iio:device*/in_anglvel_*"); len(m) > 0 {
		parts = append(parts, fmt.Sprintf("iio_gyro_channels=%d", len(m)))
	}
	if s.engine != nil {
		wsDir, _ := s.engine.resolveSandboxDirs()
		if wsDir != "" {
			files, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "proprio_robotics", "*.json"))
			for i, f := range files {
				if i >= 4 {
					break
				}
				if b, err := os.ReadFile(f); err == nil {
					parts = append(parts, fmt.Sprintf("%s:%s", strings.TrimSuffix(filepath.Base(f), ".json"), truncateString(string(b), 50)))
				}
			}
		}
	}
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   strings.Join(parts, "; "),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 5b. Exteroceptive Sensor: Tactile/Touch Force Arrays, GPS/GNSS, LiDAR & Reality Replicas / Digital Twins
type tactileGpsLidarReplicaExteroSensor struct{ engine *Engine }

func (s *tactileGpsLidarReplicaExteroSensor) ID() string           { return "extero.tactile_gps_lidar_replica" }
func (s *tactileGpsLidarReplicaExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *tactileGpsLidarReplicaExteroSensor) Modality() string     { return "tactile_gps_lidar_replica" }
func (s *tactileGpsLidarReplicaExteroSensor) ProbeAvailable(_ context.Context) bool {
	for _, pat := range []string{"/dev/pps*", "/dev/gnss*", "/dev/input/touchscreen*"} {
		if m, _ := filepath.Glob(pat); len(m) > 0 {
			return true
		}
	}
	if s.engine != nil {
		wsDir, _ := s.engine.resolveSandboxDirs()
		if wsDir != "" {
			if m, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "reality_replica", "*.json")); len(m) > 0 {
				return true
			}
		}
	}
	return false
}
func (s *tactileGpsLidarReplicaExteroSensor) Read(_ context.Context) (SensorReading, error) {
	var parts []string
	gnss, _ := filepath.Glob("/dev/gnss*")
	if len(gnss) > 0 {
		parts = append(parts, fmt.Sprintf("gnss=%d", len(gnss)))
	}
	if s.engine != nil {
		wsDir, _ := s.engine.resolveSandboxDirs()
		if wsDir != "" {
			files, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "reality_replica", "*.json"))
			for i, f := range files {
				if i >= 4 {
					break
				}
				if b, err := os.ReadFile(f); err == nil {
					parts = append(parts, fmt.Sprintf("replica[%s]=%s", strings.TrimSuffix(filepath.Base(f), ".json"), truncateString(string(b), 60)))
				}
			}
		}
	}
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   strings.Join(parts, "; "),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 1. Proprioceptive Sensor: Host Compute, Memory, Disk & Runtime Vitals
type hostComputeProprioSensor struct{ engine *Engine }

func (s *hostComputeProprioSensor) ID() string           { return "proprio.host_compute" }
func (s *hostComputeProprioSensor) Domain() SensorDomain { return SensorProprioceptive }
func (s *hostComputeProprioSensor) Modality() string     { return "compute" }
func (s *hostComputeProprioSensor) ProbeAvailable(_ context.Context) bool {
	return true
}
func (s *hostComputeProprioSensor) Read(_ context.Context) (SensorReading, error) {
	var load1 float64
	if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
		if f := strings.Fields(string(raw)); len(f) > 0 {
			load1, _ = strconv.ParseFloat(f[0], 64)
		}
	}
	var totalKB, availKB int64
	if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				if f := strings.Fields(line); len(f) >= 2 {
					totalKB, _ = strconv.ParseInt(f[1], 10, 64)
				}
			} else if strings.HasPrefix(line, "MemAvailable:") {
				if f := strings.Fields(line); len(f) >= 2 {
					availKB, _ = strconv.ParseInt(f[1], 10, 64)
				}
			}
		}
	}
	memAvailMB := availKB / 1024
	memUsedMB := int64(0)
	if totalKB > availKB {
		memUsedMB = (totalKB - availKB) / 1024
	}
	var diskFreeMB int64
	if s.engine != nil {
		wsDir, _ := s.engine.resolveSandboxDirs()
		if wsDir != "" {
			var st syscall.Statfs_t
			if syscall.Statfs(wsDir, &st) == nil {
				diskFreeMB = int64(st.Bavail) * int64(st.Bsize) / (1024 * 1024)
			}
		}
	}
	goroutines := runtime.NumGoroutine()
	return SensorReading{
		SensorID: s.ID(),
		Domain:   s.Domain(),
		Modality: s.Modality(),
		Summary:  fmt.Sprintf("load=%.2f ram_used=%dMB ram_avail=%dMB disk_free=%dMB goroutines=%d", load1, memUsedMB, memAvailMB, diskFreeMB, goroutines),
		Values: map[string]any{
			"load_avg_1m": load1,
			"mem_used_mb": memUsedMB,
			"mem_avail_mb": memAvailMB,
			"disk_free_mb": diskFreeMB,
			"goroutines":   goroutines,
		},
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 2. Exteroceptive Sensor: Hardware Thermal / Infrared / Temperature Zones (/sys/class/thermal & /sys/class/hwmon)
type sysfsThermalExteroSensor struct{}

func (s *sysfsThermalExteroSensor) ID() string           { return "extero.thermal_zones" }
func (s *sysfsThermalExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *sysfsThermalExteroSensor) Modality() string     { return "thermal" }
func (s *sysfsThermalExteroSensor) ProbeAvailable(_ context.Context) bool {
	matches, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	return len(matches) > 0
}
func (s *sysfsThermalExteroSensor) Read(_ context.Context) (SensorReading, error) {
	matches, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	var temps []string
	var maxC float64
	for i, m := range matches {
		if i >= 4 {
			break
		}
		raw, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		milliC, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if err != nil {
			continue
		}
		degC := milliC / 1000.0
		if degC > maxC {
			maxC = degC
		}
		temps = append(temps, fmt.Sprintf("%.1fC", degC))
	}
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   fmt.Sprintf("thermal_max=%.1fC zones=[%s]", maxC, strings.Join(temps, ",")),
		Values:    map[string]any{"max_temp_c": maxC},
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 3. Exteroceptive Sensor: Linux Industrial I/O (IIO) Environmental / UV / Light / IR / Air Quality / Motion / LiDAR / Magnetometer (/sys/bus/iio/devices)
type iioEnvironmentalExteroSensor struct{}

func (s *iioEnvironmentalExteroSensor) ID() string           { return "extero.iio_env_optical_motion" }
func (s *iioEnvironmentalExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *iioEnvironmentalExteroSensor) Modality() string     { return "env_optical_motion" }
func (s *iioEnvironmentalExteroSensor) ProbeAvailable(_ context.Context) bool {
	devs, _ := filepath.Glob("/sys/bus/iio/devices/iio:device*")
	return len(devs) > 0
}
func (s *iioEnvironmentalExteroSensor) Read(_ context.Context) (SensorReading, error) {
	devs, _ := filepath.Glob("/sys/bus/iio/devices/iio:device*")
	var summaries []string
	for i, d := range devs {
		if i >= 4 {
			break
		}
		nameBytes, _ := os.ReadFile(filepath.Join(d, "name"))
		name := strings.TrimSpace(string(nameBytes))
		if name == "" {
			name = filepath.Base(d)
		}
		// Check common IIO channels: illuminance (light), uvindex (UV), proximity/distance (IR/LiDAR), humidityrelative, pressure, temp, resistance (VOC/air quality), accel/anglvel (motion)
		var chanVals []string
		for _, globPat := range []string{"in_*_input", "in_*_raw"} {
			inputs, _ := filepath.Glob(filepath.Join(d, globPat))
			for j, inp := range inputs {
				if j >= 3 {
					break
				}
				valB, err := os.ReadFile(inp)
				if err == nil {
					chanVals = append(chanVals, fmt.Sprintf("%s=%s", filepath.Base(inp), strings.TrimSpace(string(valB))))
				}
			}
		}
		summaries = append(summaries, fmt.Sprintf("%s(%s)", name, strings.Join(chanVals, ",")))
	}
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   "iio=[" + strings.Join(summaries, "; ") + "]",
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 4. Exteroceptive Sensor: Power Supply / Battery / AC State (/sys/class/power_supply)
type powerSupplyExteroSensor struct{}

func (s *powerSupplyExteroSensor) ID() string           { return "extero.power_supply" }
func (s *powerSupplyExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *powerSupplyExteroSensor) Modality() string     { return "power" }
func (s *powerSupplyExteroSensor) ProbeAvailable(_ context.Context) bool {
	ps, _ := filepath.Glob("/sys/class/power_supply/*")
	return len(ps) > 0
}
func (s *powerSupplyExteroSensor) Read(_ context.Context) (SensorReading, error) {
	ps, _ := filepath.Glob("/sys/class/power_supply/*")
	var items []string
	for _, p := range ps {
		base := filepath.Base(p)
		if capB, err := os.ReadFile(filepath.Join(p, "capacity")); err == nil {
			stB, _ := os.ReadFile(filepath.Join(p, "status"))
			items = append(items, fmt.Sprintf("%s=%s%%(%s)", base, strings.TrimSpace(string(capB)), strings.TrimSpace(string(stB))))
		} else if onB, err := os.ReadFile(filepath.Join(p, "online")); err == nil {
			items = append(items, fmt.Sprintf("%s:online=%s", base, strings.TrimSpace(string(onB))))
		}
	}
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   "power=[" + strings.Join(items, ", ") + "]",
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 5. Exteroceptive Sensor: Audio / Video / Radio / Access-Control / Peripheral Device Presence (/dev/video*, /dev/snd/*, /dev/ttyUSB*, /dev/ttyACM*, /dev/swradio*)
type mediaPeripheralsExteroSensor struct{}

func (s *mediaPeripheralsExteroSensor) ID() string           { return "extero.peripherals" }
func (s *mediaPeripheralsExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *mediaPeripheralsExteroSensor) Modality() string     { return "audio_video_rf_serial" }
func (s *mediaPeripheralsExteroSensor) ProbeAvailable(_ context.Context) bool {
	for _, pat := range []string{"/dev/video*", "/dev/snd/pcm*", "/dev/ttyUSB*", "/dev/ttyACM*", "/dev/swradio*"} {
		if m, _ := filepath.Glob(pat); len(m) > 0 {
			return true
		}
	}
	return false
}
func (s *mediaPeripheralsExteroSensor) Read(_ context.Context) (SensorReading, error) {
	vids, _ := filepath.Glob("/dev/video*")
	pcms, _ := filepath.Glob("/dev/snd/pcm*")
	serA, _ := filepath.Glob("/dev/ttyUSB*")
	serB, _ := filepath.Glob("/dev/ttyACM*")
	rads, _ := filepath.Glob("/dev/swradio*")
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   fmt.Sprintf("video_devs=%d audio_pcm=%d serial_sensors=%d rf_sdr=%d", len(vids), len(pcms), len(serA)+len(serB), len(rads)),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// 6. Exteroceptive Sensor: Derived Human Circadian & Interaction Availability Pattern (§7.2)
type humanCircadianExteroSensor struct{ engine *Engine }

func (s *humanCircadianExteroSensor) ID() string                       { return "extero.human_circadian" }
func (s *humanCircadianExteroSensor) Domain() SensorDomain             { return SensorExteroceptive }
func (s *humanCircadianExteroSensor) Modality() string                 { return "human_circadian" }
func (s *humanCircadianExteroSensor) ProbeAvailable(_ context.Context) bool { return s.engine != nil }
func (s *humanCircadianExteroSensor) Read(ctx context.Context) (SensorReading, error) {
	now := time.Now().UTC()
	pAct, expReply, nEv := s.engine.EstimateHumanCircadianAvailability(ctx, now)
	return SensorReading{
		SensorID: s.ID(),
		Domain:   s.Domain(),
		Modality: s.Modality(),
		Summary:  fmt.Sprintf("UTC_h=%02d P(active)=%.2f E[reply]=%.0fs (n=%d events)", now.Hour(), pAct, expReply, nEv),
		Values: map[string]any{
			"prob_active":        pAct,
			"expected_reply_sec": expReply,
			"observed_events":    nEv,
			"hour_utc":           now.Hour(),
		},
		UpdatedAt: now,
	}, nil
}

// 7. Exteroceptive Sensor: Pluggable Workspace External Sensor Feeds (`/workspace/.harness_d/sensors/*.json`)
// Allows any external dumb sensor, IoT bridge, access-control webhook, LiDAR, UV/IR/air-quality daemon, or computer-use monitor
// to publish readings by writing JSON files into `/workspace/.harness_d/sensors/<name>.json`.
type workspaceCustomExteroSensor struct{ engine *Engine }

func (s *workspaceCustomExteroSensor) ID() string           { return "extero.workspace_plugins" }
func (s *workspaceCustomExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *workspaceCustomExteroSensor) Modality() string     { return "custom_bus" }
func (s *workspaceCustomExteroSensor) ProbeAvailable(_ context.Context) bool {
	if s.engine == nil {
		return false
	}
	wsDir, _ := s.engine.resolveSandboxDirs()
	if wsDir == "" {
		return false
	}
	files, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "sensors", "*.json"))
	return len(files) > 0
}
func (s *workspaceCustomExteroSensor) Read(_ context.Context) (SensorReading, error) {
	wsDir, _ := s.engine.resolveSandboxDirs()
	files, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "sensors", "*.json"))
	var parts []string
	vals := make(map[string]any)
	for i, f := range files {
		if i >= 8 {
			break
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		var payload map[string]any
		if json.Unmarshal(raw, &payload) == nil {
			vals[name] = payload
			if sum, ok := payload["summary"].(string); ok && sum != "" {
				parts = append(parts, fmt.Sprintf("%s:%s", name, sum))
				continue
			}
		}
		parts = append(parts, fmt.Sprintf("%s:%s", name, truncateString(string(raw), 60)))
	}
	return SensorReading{
		SensorID:  s.ID(),
		Domain:    s.Domain(),
		Modality:  s.Modality(),
		Summary:   "custom_sensors=[" + strings.Join(parts, "; ") + "]",
		Values:    vals,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// CollectSensoriumSnapshot dynamically probes all registered Proprioceptive + Exteroceptive sensor plugins
// available on the current system and aggregates their readings (§7.1–7.2).
func (e *Engine) CollectSensoriumSnapshot(ctx context.Context) SensoriumSnapshot {
	snap := SensoriumSnapshot{
		Goroutines:     runtime.NumGoroutine(),
		CurrentHourUTC: time.Now().UTC().Hour(),
	}

	for _, plugin := range e.BuiltinPluggableSensors() {
		if !plugin.ProbeAvailable(ctx) {
			continue
		}
		reading, err := plugin.Read(ctx)
		if err != nil {
			continue
		}
		snap.Readings = append(snap.Readings, reading)
		if reading.SensorID == "proprio.host_compute" {
			if v, ok := reading.Values["load_avg_1m"].(float64); ok {
				snap.LoadAvg1m = v
			}
			if v, ok := reading.Values["mem_used_mb"].(int64); ok {
				snap.MemUsedMB = v
			}
			if v, ok := reading.Values["mem_avail_mb"].(int64); ok {
				snap.MemAvailMB = v
			}
			if v, ok := reading.Values["disk_free_mb"].(int64); ok {
				snap.DiskFreeMB = v
			}
		} else if reading.SensorID == "extero.human_circadian" {
			if v, ok := reading.Values["prob_active"].(float64); ok {
				snap.HumanProbActiveNow = v
			}
			if v, ok := reading.Values["expected_reply_sec"].(float64); ok {
				snap.ExpectedReplySec = v
			}
			if v, ok := reading.Values["observed_events"].(int); ok {
				snap.ObservedHumanEvents = v
			}
		}
	}
	return snap
}

// RecordHumanInteractionEvent records a real human timestamp (from Cockpit chat, Telegram, or git commits)
// into `human_events` so the 24-bin circadian model learns purely from empirical data (§7.2).
func (e *Engine) RecordHumanInteractionEvent(ctx context.Context, source string, ts time.Time, replyLatencySec float64) {
	db := e.getDB()
	if db == nil {
		return
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	} else {
		ts = ts.UTC()
	}
	_, _ = db.ExecContext(ctx, `
		INSERT INTO human_events (source, event_ts, hour_utc, reply_latency_sec)
		VALUES (?, ?, ?, ?)
	`, source, ts, ts.Hour(), replyLatencySec)
}

// EstimateHumanCircadianAvailability implements the 24-bin hourly Beta(1 + k_h, 1 + N_h - k_h) estimator
// with 3-hour circular smoothing over observed human interaction timestamps (§7.2).
// Never hardcodes a sleep window.
func (e *Engine) EstimateHumanCircadianAvailability(ctx context.Context, nowUTC time.Time) (probActive float64, expectedReplySec float64, totalEvents int) {
	var hourHits [24]float64
	daysSet := make(map[string]bool)

	db := e.getDB()
	if db != nil {
		rows, err := db.QueryContext(ctx, `SELECT event_ts, hour_utc FROM human_events ORDER BY event_ts DESC LIMIT 2000`)
		if err == nil {
			for rows.Next() {
				var ts time.Time
				var hr int
				if rows.Scan(&ts, &hr) == nil && hr >= 0 && hr < 24 {
					dayKey := ts.UTC().Format("2006-01-02")
					daysSet[dayKey] = true
					hourHits[hr] += 1.0
					totalEvents++
				}
			}
			rows.Close()
		}

		crows, cerr := db.QueryContext(ctx, `SELECT created_at FROM chat_messages WHERE role = 'user' ORDER BY created_at DESC LIMIT 500`)
		if cerr == nil {
			for crows.Next() {
				var ts time.Time
				if crows.Scan(&ts) == nil {
					u := ts.UTC()
					daysSet[u.Format("2006-01-02")] = true
					hourHits[u.Hour()] += 1.0
					totalEvents++
				}
			}
			crows.Close()
		}
	}

	if totalEvents < 10 {
		out, err := exec.CommandContext(ctx, "git", "log", "-n", "120", "--format=%aI").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if t, perr := time.Parse(time.RFC3339, strings.TrimSpace(line)); perr == nil {
					u := t.UTC()
					daysSet[u.Format("2006-01-02")] = true
					hourHits[u.Hour()] += 1.0
					totalEvents++
				}
			}
		}
	}

	numDays := float64(len(daysSet))
	if numDays < 3.0 {
		numDays = 3.0
	}

	hNow := nowUTC.UTC().Hour()
	weights := []struct {
		offset int
		w      float64
	}{
		{-1, 0.25},
		{0, 0.50},
		{1, 0.25},
	}
	var smoothedProb float64
	for _, wk := range weights {
		h := (hNow + wk.offset + 24) % 24
		kh := math.Min(hourHits[h], numDays)
		p := (1.0 + kh) / (2.0 + numDays)
		smoothedProb += wk.w * p
	}
	probActive = math.Max(0.05, math.Min(0.95, smoothedProb))
	expectedReplySec = math.Round(300.0 / math.Max(0.08, probActive))
	return probActive, expectedReplySec, totalEvents
}

func (e *Engine) FormatSensoriumPromptLine(ctx context.Context) string {
	s := e.CollectSensoriumSnapshot(ctx)
	var proprio, extero []string
	for _, r := range s.Readings {
		if r.Domain == SensorProprioceptive {
			proprio = append(proprio, fmt.Sprintf("%s(%s)", r.Modality, r.Summary))
		} else {
			extero = append(extero, fmt.Sprintf("%s(%s)", r.Modality, r.Summary))
		}
	}
	return fmt.Sprintf("Sensorium [Proprio: %s | Extero: %s]", strings.Join(proprio, "; "), strings.Join(extero, "; "))
}

// DispatchHumanContradictionAlert sends a <=4-line plain-English contradiction message + pure-Go isometric
// `anatomy` SVG diagram to Telegram (or queues it into the overnight digest if P(human_active_now) < 0.25).
func (e *Engine) DispatchHumanContradictionAlert(ctx context.Context, cycleID string, nodeID string, fourLineMessage string, targetPath string) {
	probActive, _, _ := e.EstimateHumanCircadianAvailability(ctx, time.Now().UTC())
	db := e.getDB()

	if db != nil {
		var state string
		if db.QueryRowContext(ctx, `SELECT state FROM connectome_nodes WHERE id = ?`, nodeID).Scan(&state) == nil {
			if state == "PARKED_HUMAN" {
				return
			}
		}
		_, _ = db.ExecContext(ctx, `UPDATE connectome_nodes SET state = 'PARKED_HUMAN', updated_at = ? WHERE id = ?`, time.Now().UTC(), nodeID)
	}

	svgBytes := BuildAnatomyIsometricSVG(nodeID, targetPath, fourLineMessage)
	wsDir, _ := e.resolveSandboxDirs()
	if wsDir != "" {
		diagDir := filepath.Join(wsDir, ".harness_d", "anatomy")
		_ = os.MkdirAll(diagDir, 0o755)
		_ = os.WriteFile(filepath.Join(diagDir, sanitizeTaskID(nodeID)+".svg"), svgBytes, 0o644)
	}

	if probActive < 0.25 && e.memory != nil {
		prev, _ := e.memory.LoadEngineMeta(ctx, "overnight_contradiction_digest")
		entry := fmt.Sprintf("[%s] %s", nodeID, strings.ReplaceAll(fourLineMessage, "\n", " | "))
		if prev != "" {
			entry = prev + "\n" + entry
		}
		_ = e.memory.SaveEngineMeta(ctx, "overnight_contradiction_digest", entry)
		return
	}

	e.sendTelegramAnatomyEscalation(ctx, nodeID, fourLineMessage, svgBytes)
}

func (e *Engine) sendTelegramAnatomyEscalation(ctx context.Context, nodeID string, caption string, svgBytes []byte) {
	db := e.getDB()
	if db == nil {
		return
	}
	var token, allowedIDs string
	var lastChatID int64
	err := db.QueryRowContext(ctx, `SELECT token, allowed_chat_ids, last_chat_id FROM channel_settings WHERE channel = 'telegram' AND enabled = 1`).Scan(&token, &allowedIDs, &lastChatID)
	if err != nil || strings.TrimSpace(token) == "" {
		return
	}
	chatID := lastChatID
	if chatID == 0 && strings.TrimSpace(allowedIDs) != "" {
		first := strings.Split(allowedIDs, ",")[0]
		chatID, _ = strconv.ParseInt(strings.TrimSpace(first), 10, 64)
	}
	if chatID == 0 {
		return
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	if len(caption) > 950 {
		caption = caption[:950]
	}
	_ = writer.WriteField("caption", caption)
	part, err := writer.CreateFormFile("document", sanitizeTaskID(nodeID)+"-anatomy.svg")
	if err == nil {
		_, _ = part.Write(svgBytes)
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", strings.TrimSpace(token)), &body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err == nil && resp != nil {
		resp.Body.Close()
	}
}

type isoBox struct {
	x, y, z    float64
	wx, wy, wz float64
	topFill    string
	leftFill   string
	rightFill  string
	stroke     string
	tier       int // 0 = base/bevel/rails, 1 = pillars/housings, 2 = carriage/pins/details
}

// depthKey guarantees strict topological painter's order: lower z-tier (floor/bevel/rails) ALWAYS paints
// before upper z-tiers (carriages, pins, dial gauges), and within each tier sorts back-to-front (x + y + z).
func (b isoBox) depthKey() float64 {
	return float64(b.tier)*10000.0 + (b.x + b.wx*0.5) + (b.y + b.wy*0.5) + (b.z + b.wz*0.5)
}

func isoProject(x, y, z float64) (float64, float64) {
	const cos30 = 0.8660254
	const sin30 = 0.5
	const scale = 1.05
	const originX = 360.0
	const originY = 215.0
	px := originX + (x-y)*cos30*scale
	py := originY + (x+y)*sin30*scale - z*scale
	return px, py
}

// BuildAnatomyIsometricSVG builds a multi-part physical bench test-rig (`anatomy` skill rules:
// 45+ solid parts, base plate with bevels, 4 corner screws, 12 ruler ticks, twin steel guide rails,
// sliding carriage on 4 bushings, 5-pin shear-line lock tumblers parameterized by targetPath/nodeID,
// and dial gauge housing — with zero text inside the 3D drawing and strict tier-sorted painter's order).
func BuildAnatomyIsometricSVG(nodeID string, targetPath string, message string) []byte {
	seed := 0
	for i := 0; i < len(nodeID+targetPath); i++ {
		seed = (seed*31 + int((nodeID+targetPath)[i])) & 0x7fffffff
	}
	carriageOffset := float64((seed%50)-25) * 1.2

	boxes := []isoBox{
		// Tier 0: Base bevel, cast-iron bed plate, recessed serial plate, 4 corner bolts, 12 ruler ticks, twin guide rails
		{x: -146, y: -106, z: -4, wx: 292, wy: 212, wz: 4, topFill: "#cbd5e1", leftFill: "#94a3b8", rightFill: "#64748b", stroke: "#334155", tier: 0},
		{x: -140, y: -100, z: 0, wx: 280, wy: 200, wz: 14, topFill: "#f1f5f9", leftFill: "#cbd5e1", rightFill: "#94a3b8", stroke: "#334155", tier: 0},
		// Front brass serial plate & 2 rivets
		{x: -45, y: 74, z: 14, wx: 90, wy: 18, wz: 3, topFill: "#fef3c7", leftFill: "#fcd34d", rightFill: "#d97706", stroke: "#92400e", tier: 0},
		{x: -40, y: 80, z: 17, wx: 4, wy: 4, wz: 2, topFill: "#fde68a", leftFill: "#f59e0b", rightFill: "#b45309", stroke: "#78350f", tier: 0},
		{x: 36, y: 80, z: 17, wx: 4, wy: 4, wz: 2, topFill: "#fde68a", leftFill: "#f59e0b", rightFill: "#b45309", stroke: "#78350f", tier: 0},
		// 4 corner hex screws
		{x: -132, y: -92, z: 14, wx: 10, wy: 10, wz: 4, topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		{x: 122, y: -92, z: 14, wx: 10, wy: 10, wz: 4, topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		{x: -132, y: 82, z: 14, wx: 10, wy: 10, wz: 4, topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		{x: 122, y: 82, z: 14, wx: 10, wy: 10, wz: 4, topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		// Twin ground steel guide rails + 4 end pillow blocks
		{x: -115, y: -38, z: 14, wx: 230, wy: 8, wz: 6, topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#64748b", stroke: "#334155", tier: 0},
		{x: -115, y: 26, z: 14, wx: 230, wy: 8, wz: 6, topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#64748b", stroke: "#334155", tier: 0},
		{x: -120, y: -42, z: 14, wx: 14, wy: 16, wz: 12, topFill: "#cbd5e1", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		{x: 106, y: -42, z: 14, wx: 14, wy: 16, wz: 12, topFill: "#cbd5e1", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		{x: -120, y: 22, z: 14, wx: 14, wy: 16, wz: 12, topFill: "#cbd5e1", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
		{x: 106, y: 22, z: 14, wx: 14, wy: 16, wz: 12, topFill: "#cbd5e1", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 0},
	}

	// 12 precision ruler scale ticks along front bed edge
	for i := 0; i < 12; i++ {
		tx := -110.0 + float64(i)*20.0
		wy := 6.0
		if i%5 == 0 {
			wy = 10.0
		}
		boxes = append(boxes, isoBox{
			x: tx, y: 56, z: 14, wx: 2.5, wy: wy, wz: 1.5,
			topFill: "#94a3b8", leftFill: "#64748b", rightFill: "#475569", stroke: "#334155", tier: 0,
		})
	}

	// Tier 1: Rear Shear-Line Gate Bridge & 5 Pin-Tumbler Chambers + Right Dial Gauge Stand
	boxes = append(boxes,
		isoBox{x: -95, y: -84, z: 14, wx: 130, wy: 34, wz: 44, topFill: "#e0f2fe", leftFill: "#7dd3fc", rightFill: "#0284c7", stroke: "#075985", tier: 1},
		isoBox{x: 56, y: -80, z: 14, wx: 58, wy: 42, wz: 52, topFill: "#ede9fe", leftFill: "#c4b5fd", rightFill: "#7c3aed", stroke: "#4c1d95", tier: 1},
		isoBox{x: 68, y: -68, z: 66, wx: 34, wy: 24, wz: 16, topFill: "#fef3c7", leftFill: "#fcd34d", rightFill: "#d97706", stroke: "#78350f", tier: 1},
	)

	// 5 Pin-Tumbler Verification Pins (heights vary deterministically by artifact/gate hash)
	for p := 0; p < 5; p++ {
		px := -82.0 + float64(p)*22.0
		pinH := 12.0 + float64(((seed>>(p*3))&7))*2.5
		isBlockedPin := p == (seed % 5)
		tf, lf, rf, st := "#dcfce7", "#86efac", "#16a34a", "#14532d"
		if isBlockedPin {
			tf, lf, rf, st = "#ffe4e6", "#fda4af", "#e11d48", "#881337"
		}
		boxes = append(boxes, isoBox{
			x: px, y: -74, z: 58, wx: 12, wy: 14, wz: pinH,
			topFill: tf, leftFill: lf, rightFill: rf, stroke: st, tier: 2,
		})
	}

	// Tier 2: Sliding Workpad Carriage on 4 bronze bushings + 6-coil return spring + specimen clamp
	cx := carriageOffset
	boxes = append(boxes,
		// 4 bronze rail bushings
		isoBox{x: cx - 42, y: -41, z: 18, wx: 16, wy: 14, wz: 8, topFill: "#fef3c7", leftFill: "#fbbf24", rightFill: "#b45309", stroke: "#78350f", tier: 2},
		isoBox{x: cx + 26, y: -41, z: 18, wx: 16, wy: 14, wz: 8, topFill: "#fef3c7", leftFill: "#fbbf24", rightFill: "#b45309", stroke: "#78350f", tier: 2},
		isoBox{x: cx - 42, y: 23, z: 18, wx: 16, wy: 14, wz: 8, topFill: "#fef3c7", leftFill: "#fbbf24", rightFill: "#b45309", stroke: "#78350f", tier: 2},
		isoBox{x: cx + 26, y: 23, z: 18, wx: 16, wy: 14, wz: 8, topFill: "#fef3c7", leftFill: "#fbbf24", rightFill: "#b45309", stroke: "#78350f", tier: 2},
		// Main workpad carriage block + upper precision jaw
		isoBox{x: cx - 46, y: -36, z: 26, wx: 92, wy: 68, wz: 22, topFill: "#f8fafc", leftFill: "#cbd5e1", rightFill: "#64748b", stroke: "#1e293b", tier: 2},
		isoBox{x: cx - 28, y: -22, z: 48, wx: 56, wy: 40, wz: 16, topFill: "#ffe4e6", leftFill: "#fda4af", rightFill: "#e11d48", stroke: "#881337", tier: 2},
	)
	// 6 return spring coils between right pillow block and carriage
	for c := 0; c < 6; c++ {
		sx := cx + 48.0 + float64(c)*8.5
		boxes = append(boxes, isoBox{
			x: sx, y: -8, z: 22, wx: 4.5, wy: 12, wz: 10,
			topFill: "#e2e8f0", leftFill: "#94a3b8", rightFill: "#475569", stroke: "#1e293b", tier: 2,
		})
	}

	sort.Slice(boxes, func(i, j int) bool {
		return boxes[i].depthKey() < boxes[j].depthKey()
	})

	var svg strings.Builder
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 720 440" width="720" height="440" font-family="ui-monospace, SFMono-Regular, Menlo, monospace">`)
	svg.WriteString(`<rect x="8" y="8" width="704" height="424" rx="12" fill="#f8fafc" stroke="#cbd5e1" stroke-width="2"/>`)
	svg.WriteString(fmt.Sprintf(`<text x="28" y="34" font-size="13" font-weight="700" fill="#0f172a">ACE v0.7 SHEAR-LINE GATE RIG // %s</text>`, escapeXML(truncateString(filepath.Base(targetPath), 48))))
	svg.WriteString(fmt.Sprintf(`<text x="28" y="52" font-size="11" fill="#475569">5-pin tumbler gate + sliding workpad carriage (%d solid parts, topological painter's order)</text>`, len(boxes)))

	polyPoints := func(pts [][3]float64) string {
		var out []string
		for _, pt := range pts {
			px, py := isoProject(pt[0], pt[1], pt[2])
			out = append(out, fmt.Sprintf("%.1f,%.1f", px, py))
		}
		return strings.Join(out, " ")
	}

	for _, bx := range boxes {
		x0, y0, z0 := bx.x, bx.y, bx.z
		x1, y1, z1 := bx.x+bx.wx, bx.y+bx.wy, bx.z+bx.wz

		leftPts := [][3]float64{{x0, y1, z0}, {x1, y1, z0}, {x1, y1, z1}, {x0, y1, z1}}
		rightPts := [][3]float64{{x1, y0, z0}, {x1, y1, z0}, {x1, y1, z1}, {x1, y0, z1}}
		topPts := [][3]float64{{x0, y0, z1}, {x1, y0, z1}, {x1, y1, z1}, {x0, y1, z1}}

		svg.WriteString(fmt.Sprintf(`<polygon points="%s" fill="%s" stroke="%s" stroke-width="1.1" stroke-linejoin="round"/>`, polyPoints(leftPts), bx.leftFill, bx.stroke))
		svg.WriteString(fmt.Sprintf(`<polygon points="%s" fill="%s" stroke="%s" stroke-width="1.1" stroke-linejoin="round"/>`, polyPoints(rightPts), bx.rightFill, bx.stroke))
		svg.WriteString(fmt.Sprintf(`<polygon points="%s" fill="%s" stroke="%s" stroke-width="1.1" stroke-linejoin="round"/>`, polyPoints(topPts), bx.topFill, bx.stroke))
	}

	// Footer readout card (outside the 3D drawing, never overlapping parts)
	svg.WriteString(`<rect x="24" y="346" width="672" height="72" rx="6" fill="#0f172a"/>`)
	lines := strings.Split(message, "\n")
	for i := 0; i < len(lines) && i < 4; i++ {
		svg.WriteString(fmt.Sprintf(`<text x="36" y="%d" font-size="10.5" fill="#e2e8f0">%s</text>`, 364+i*15, escapeXML(truncateString(lines[i], 96))))
	}
	svg.WriteString(`</svg>`)
	return []byte(svg.String())
}

func escapeXML(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(s)
}
