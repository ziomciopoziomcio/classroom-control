package agent

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

const localUIListenAddr = "127.0.0.1:18777"

func (r *Runner) serveLocalUI(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.cfgMu.RLock()
		cfg := r.Config
		r.cfgMu.RUnlock()
		overview, err := r.TicketsOverview()
		if err != nil {
			log.Printf("tickets overview fetch failed: %v", err)
		}
		tpl := template.Must(template.New("ui").Parse(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>SchoolAgent</title></head><body>
<h1>{{.About}}</h1>
<p><strong>{{.RoomLabel}}:</strong> {{.RoomID}} / {{.ComputerID}}</p>
<p><strong>{{.VersionLabel}}:</strong> {{.Version}}</p>
<h2>Helpdesk</h2>
<form action="/ticket" method="post">
<label>Category: <input type="text" name="category" required></label><br>
<label>Description: <textarea name="description" required></textarea></label><br>
<button type="submit">Submit</button>
</form>
<h2>Tickets Overview (all rooms)</h2>
<table border="1" cellspacing="0" cellpadding="4"><tr><th>ID</th><th>Room</th><th>Computer</th><th>Category</th><th>Description</th></tr>
{{range .Tickets}}<tr><td>{{index . "ticket_id"}}</td><td>{{index . "room_id"}}</td><td>{{index . "computer_id"}}</td><td>{{index . "category"}}</td><td>{{index . "description"}}</td></tr>{{end}}
</table>
</body></html>`))
		_ = tpl.Execute(w, map[string]any{
			"About":        T(cfg.Language, "about_title"),
			"RoomLabel":    T(cfg.Language, "room_station"),
			"VersionLabel": T(cfg.Language, "app_version"),
			"RoomID":       cfg.RoomID,
			"ComputerID":   cfg.ComputerID,
			"Version":      cfg.Version,
			"Tickets":      overview,
		})
	})
	mux.HandleFunc("/ticket", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		category := req.FormValue("category")
		desc := req.FormValue("description")
		err := r.SubmitTicket(category, desc)
		r.cfgMu.RLock()
		lang := r.Config.Language
		r.cfgMu.RUnlock()
		if err != nil {
			_ = Notify("SchoolAgent", T(lang, "ticket_submit_failed"))
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = Notify("SchoolAgent", T(lang, "ticket_submit_success"))
		http.Redirect(w, req, "/", http.StatusFound)
	})

	srv := &http.Server{Addr: localUIListenAddr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("local ui stopped: %v", err)
	}
}

func LocalUIURL() string {
	return fmt.Sprintf("http://%s", localUIListenAddr)
}
