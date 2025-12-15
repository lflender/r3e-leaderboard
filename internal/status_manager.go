package internal

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
    "time"
)

// TrackStatus holds per-track refresh status
type TrackStatus struct {
    TrackID     string    `json:"track_id"`
    Status      string    `json:"status"` // updating | ready
    LastUpdated time.Time `json:"last_updated,omitempty"`
}

// TrackStatusManager persists per-track statuses to disk
type TrackStatusManager struct {
    filePath string
    mutex    sync.RWMutex
    statuses map[string]TrackStatus
}

// NewTrackStatusManager creates a manager that stores statuses in cache/track_status.json
func NewTrackStatusManager() *TrackStatusManager {
    return &TrackStatusManager{
        filePath: filepath.Join("cache", "track_status.json"),
        statuses: make(map[string]TrackStatus),
    }
}

// Load loads persisted statuses from disk
func (m *TrackStatusManager) Load() error {
    m.mutex.Lock()
    defer m.mutex.Unlock()

    data, err := os.ReadFile(m.filePath)
    if err != nil {
        // missing file is not an error
        return nil
    }
    var arr []TrackStatus
    if err := json.Unmarshal(data, &arr); err != nil {
        return err
    }
    for _, s := range arr {
        m.statuses[s.TrackID] = s
    }
    return nil
}

// Save persists current statuses
func (m *TrackStatusManager) Save() error {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
        return err
    }
    arr := make([]TrackStatus, 0, len(m.statuses))
    for _, s := range m.statuses {
        arr = append(arr, s)
    }
    data, err := json.MarshalIndent(arr, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(m.filePath, data, 0644)
}

// SetUpdating marks a track as updating
func (m *TrackStatusManager) SetUpdating(trackID string) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    s := m.statuses[trackID]
    s.TrackID = trackID
    s.Status = "updating"
    s.LastUpdated = time.Time{}
    m.statuses[trackID] = s
    return m.Save()
}

// SetUpdated marks a track as updated with timestamp
func (m *TrackStatusManager) SetUpdated(trackID string, ts time.Time) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    s := m.statuses[trackID]
    s.TrackID = trackID
    s.Status = "ready"
    s.LastUpdated = ts
    m.statuses[trackID] = s
    return m.Save()
}

// GetAll returns a copy of statuses
func (m *TrackStatusManager) GetAll() []TrackStatus {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    out := make([]TrackStatus, 0, len(m.statuses))
    for _, s := range m.statuses {
        out = append(out, s)
    }
    return out
}
