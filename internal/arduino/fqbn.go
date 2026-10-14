package arduino

import "strings"

// FQBN maps library.properties architectures to a compile board.
func FQBN(properties string) string {
	arch := ""
	for _, line := range strings.Split(properties, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "architectures=") {
			arch = strings.TrimSpace(line[len("architectures="):])
			break
		}
	}
	if strings.Contains(strings.ToLower(arch), "esp32") {
		return "esp32:esp32:esp32"
	}
	return "arduino:avr:uno"
}
