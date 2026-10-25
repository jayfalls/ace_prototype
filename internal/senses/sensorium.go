// Package senses hosts ACE's pluggable Proprioceptive and Exteroceptive
// sensor suite (spec 9.1: senses move here as built-in Sense plugins).
// Sensors are system-agnostic: each activates only when ProbeAvailable
// reports real hardware/feeds on this host.
package senses

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Deps is everything the sensor suite needs from the host engine: the
// workspace dir (for .harness_d feeds) and the circadian estimator (which
// stays in the engine because it queries chat/human-event history).
type Deps struct {
	WorkspaceDir func() string
	Circadian    func(ctx context.Context, nowUTC time.Time) (probActive, expectedReplySec float64, totalEvents int)
}

// wsDir resolves the workspace dir or "" when unconfigured.
func (d Deps) wsDir() string {
	if d.WorkspaceDir == nil {
		return ""
	}
	return d.WorkspaceDir()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// SensorDomain classifies whether a sensor plugin measures internal agent/compute vitals (Proprioceptive)
// or external physical/environmental/peripheral/behavioral signals (Exteroceptive).
type SensorDomain string

const (
	SensorProprioceptive SensorDomain = "PROPRIOCEPTIVE" // Self-referential: CPU, RAM, Disk, GPU VRAM, SQLite wait, LLM quota/cache
	SensorExteroceptive  SensorDomain = "EXTEROCEPTIVE"  // Externally-referential: Thermal/Temp, IIO (UV/Light/IR/Air/Pressure/Motion/Lidar), Power/Battery, Audio/Video/Peripherals, Access Control, Derived Human Circadian, Custom Plugins
)

// SensorReading is a standardized reading emitted by any pluggable sensor on the current host.
type SensorReading struct {
	SensorID  string         `json:"sensor_id"`
	Domain    SensorDomain   `json:"domain"`
	Modality  string         `json:"modality"` // compute, thermal, optical_uv_ir, air_env, power, audio_video, motion_lidar, radio_rf, access_ctrl, human_circadian, custom
	Summary   string         `json:"summary"`
	Values    map[string]any `json:"values,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
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
func BuiltinPluggableSensors(d Deps) []PluggableSensor {
	return []PluggableSensor{
		&hostComputeProprioSensor{deps: d},
		&embodiedKinematicsIMUProprioSensor{deps: d},
		&sysfsThermalExteroSensor{},
		&iioEnvironmentalExteroSensor{},
		&powerSupplyExteroSensor{},
		&mediaPeripheralsExteroSensor{},
		&tactileGpsLidarReplicaExteroSensor{deps: d},
		&humanCircadianExteroSensor{deps: d},
		&workspaceCustomExteroSensor{deps: d},
	}
}

// 1b. Proprioceptive Sensor: Embodied Robotics / Kinematics / Vestibular IMU (Gyroscope, Accelerometer, Joint Encoders, Motor Torque)
type embodiedKinematicsIMUProprioSensor struct{ deps Deps }

func (s *embodiedKinematicsIMUProprioSensor) ID() string           { return "proprio.embodied_kinematics_imu" }
func (s *embodiedKinematicsIMUProprioSensor) Domain() SensorDomain { return SensorProprioceptive }
func (s *embodiedKinematicsIMUProprioSensor) Modality() string     { return "imu_gyro_joints" }
func (s *embodiedKinematicsIMUProprioSensor) ProbeAvailable(_ context.Context) bool {
	if m, _ := filepath.Glob("/sys/bus/iio/devices/iio:device*/in_anglvel_*"); len(m) > 0 {
		return true
	}
	if s.deps.WorkspaceDir != nil {
		wsDir := s.deps.wsDir()
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
	if s.deps.WorkspaceDir != nil {
		wsDir := s.deps.wsDir()
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
type tactileGpsLidarReplicaExteroSensor struct{ deps Deps }

func (s *tactileGpsLidarReplicaExteroSensor) ID() string           { return "extero.tactile_gps_lidar_replica" }
func (s *tactileGpsLidarReplicaExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *tactileGpsLidarReplicaExteroSensor) Modality() string     { return "tactile_gps_lidar_replica" }
func (s *tactileGpsLidarReplicaExteroSensor) ProbeAvailable(_ context.Context) bool {
	for _, pat := range []string{"/dev/pps*", "/dev/gnss*", "/dev/input/touchscreen*"} {
		if m, _ := filepath.Glob(pat); len(m) > 0 {
			return true
		}
	}
	if s.deps.WorkspaceDir != nil {
		wsDir := s.deps.wsDir()
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
	if s.deps.WorkspaceDir != nil {
		wsDir := s.deps.wsDir()
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
type hostComputeProprioSensor struct{ deps Deps }

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
	if s.deps.WorkspaceDir != nil {
		wsDir := s.deps.wsDir()
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
			"load_avg_1m":  load1,
			"mem_used_mb":  memUsedMB,
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
type humanCircadianExteroSensor struct{ deps Deps }

func (s *humanCircadianExteroSensor) ID() string           { return "extero.human_circadian" }
func (s *humanCircadianExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *humanCircadianExteroSensor) Modality() string     { return "human_circadian" }
func (s *humanCircadianExteroSensor) ProbeAvailable(_ context.Context) bool {
	return s.deps.Circadian != nil
}
func (s *humanCircadianExteroSensor) Read(ctx context.Context) (SensorReading, error) {
	now := time.Now().UTC()
	pAct, expReply, nEv := s.deps.Circadian(ctx, now)
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
type workspaceCustomExteroSensor struct{ deps Deps }

func (s *workspaceCustomExteroSensor) ID() string           { return "extero.workspace_plugins" }
func (s *workspaceCustomExteroSensor) Domain() SensorDomain { return SensorExteroceptive }
func (s *workspaceCustomExteroSensor) Modality() string     { return "custom_bus" }
func (s *workspaceCustomExteroSensor) ProbeAvailable(_ context.Context) bool {
	if s.deps.WorkspaceDir == nil {
		return false
	}
	wsDir := s.deps.wsDir()
	if wsDir == "" {
		return false
	}
	files, _ := filepath.Glob(filepath.Join(wsDir, ".harness_d", "sensors", "*.json"))
	return len(files) > 0
}
func (s *workspaceCustomExteroSensor) Read(_ context.Context) (SensorReading, error) {
	wsDir := s.deps.wsDir()
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
