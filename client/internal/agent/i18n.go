package agent

var translations = map[string]map[string]string{
	"en": {
		"about_title":           "About",
		"room_station":          "Room / Station",
		"app_version":           "Version",
		"ticket_submit_success": "Ticket submitted",
		"ticket_submit_failed":  "Ticket submission failed",
		"action_received":       "Remote administrative action received",
		"speedtest_done":        "Speed test completed",
	},
	"pl": {
		"about_title":           "O aplikacji",
		"room_station":          "Sala / Stanowisko",
		"app_version":           "Wersja",
		"ticket_submit_success": "Zgłoszenie wysłane",
		"ticket_submit_failed":  "Nie udało się wysłać zgłoszenia",
		"action_received":       "Otrzymano zdalne polecenie administratora",
		"speedtest_done":        "Test prędkości zakończony",
	},
}

func T(lang, key string) string {
	if l, ok := translations[lang]; ok {
		if text, ok := l[key]; ok {
			return text
		}
	}
	if text, ok := translations["en"][key]; ok {
		return text
	}
	return key
}
