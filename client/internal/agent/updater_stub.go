//go:build !windows

package agent

func RunUpdater(_ Config, _ string) error {
	return nil
}
