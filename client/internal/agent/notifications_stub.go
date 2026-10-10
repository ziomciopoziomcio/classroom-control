//go:build !windows

package agent

import "log"

func Notify(title, message string) error {
	log.Printf("NOTIFY %s: %s", title, message)
	return nil
}
