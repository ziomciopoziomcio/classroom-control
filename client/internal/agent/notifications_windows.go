//go:build windows

package agent

import "os/exec"

func Notify(title, message string) error {
	script := `
$title = $args[0]
$message = $args[1]
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
$template = "<toast><visual><binding template='ToastGeneric'><text>$title</text><text>$message</text></binding></visual></toast>"
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('SchoolAgent')
$notifier.Show($toast)
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script, title, message)
	return cmd.Run()
}
