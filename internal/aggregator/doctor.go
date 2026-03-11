package aggregator

import (
	"fmt"
	"os/exec"
	"strings"
)

// DoctorStatus represents the state of a required external dependency.
type DoctorStatus struct {
	Name      string
	Available bool
	Version   string
	Error     string
}

// CheckEnvironment verifies that python3 and pip3 are installed and accessible.
func CheckEnvironment() []DoctorStatus {
	var results []DoctorStatus

	// Check python3
	pyStatus := DoctorStatus{Name: "python3"}
	pyCmd := exec.Command("python3", "--version")
	if out, err := pyCmd.CombinedOutput(); err == nil {
		pyStatus.Available = true
		pyStatus.Version = strings.TrimSpace(string(out))
	} else {
		pyStatus.Error = "python3 not found in PATH"
	}
	results = append(results, pyStatus)

	// Check pip3
	pipStatus := DoctorStatus{Name: "pip3"}
	pipCmd := exec.Command("pip3", "--version")
	if out, err := pipCmd.CombinedOutput(); err == nil {
		pipStatus.Available = true
		// pip version output is usually long, just take the first part
		pipStatus.Version = strings.TrimSpace(string(out))
		if idx := strings.Index(pipStatus.Version, " from"); idx != -1 {
			pipStatus.Version = pipStatus.Version[:idx]
		}
	} else {
		pipStatus.Error = "pip3 not found in PATH"
	}
	results = append(results, pipStatus)

	return results
}

// GetDoctorReport returns a human-readable string of the environment status.
func GetDoctorReport() (string, bool) {
	statuses := CheckEnvironment()
	var report strings.Builder
	allOk := true

	report.WriteString("Aggregator Environment Check:\n\n")
	for _, s := range statuses {
		if s.Available {
			report.WriteString(fmt.Sprintf("✅ %s: %s\n", s.Name, s.Version))
		} else {
			report.WriteString(fmt.Sprintf("❌ %s: %s\n", s.Name, s.Error))
			allOk = false
		}
	}

	if !allOk {
		report.WriteString("\nWarning: External aggregators may not function correctly.\nPlease install missing dependencies.")
	} else {
		report.WriteString("\nAll systems nominal. Aggregators ready.")
	}

	return report.String(), allOk
}
