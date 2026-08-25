package config

import (
	"log"
	"os"
)

// CheckUID - Checks to make sure user has root privileges
func CheckUID() {
	if os.Geteuid() != 0 {
		log.Fatal("This program must be run with elevated privileges. Please re-run with sudo or as root.")
	}
}
