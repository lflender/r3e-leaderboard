package internal

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"
)

// EnsureLogDir creates the log directory
func EnsureLogDir() error {
    return os.MkdirAll("log", 0755)
}

// AppendLog writes a message to a daily log file and stdout
func AppendLog(prefix, msg string) {
    if err := EnsureLogDir(); err != nil {
        log.Printf("⚠️ Could not create log dir: %v", err)
        return
    }
    filename := filepath.Join("log", time.Now().Format("2006-01-02")+".log")
    f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        log.Printf("⚠️ Could not open log file: %v", err)
        return
    }
    defer f.Close()
    line := fmt.Sprintf("%s %s: %s\n", time.Now().Format(time.RFC3339), prefix, msg)
    f.WriteString(line)
    log.Print(prefix, " ", msg)
}

