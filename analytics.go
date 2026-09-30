package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

// gaEventsCookie carries GA4 events from the server to the browser. Most
// conversion actions (save, signup, accept) finish with a redirect, so the
// event can't be written into the page that handled the action. Instead the
// handler queues it here, and a script in header.gohtml fires every queued
// event through gtag on the next page load and then clears the cookie.
const gaEventsCookie = "ga_events"

type gaEvent struct {
	Name   string                 `json:"name"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// queueGAEvent adds a GA4 event to the ga_events cookie. Events already
// queued by an earlier response that the browser hasn't fired yet (e.g.
// sign_up, then an auto-save on the redirect target) are kept. Must be called
// before the response headers are written.
func queueGAEvent(w http.ResponseWriter, r *http.Request, name string, params map[string]interface{}) {
	var events []gaEvent
	if c, err := r.Cookie(gaEventsCookie); err == nil {
		if raw, err := url.QueryUnescape(c.Value); err == nil {
			_ = json.Unmarshal([]byte(raw), &events)
		}
	}
	events = append(events, gaEvent{Name: name, Params: params})

	b, err := json.Marshal(events)
	if err != nil {
		log.Printf("queueGAEvent: marshal %s: %v", name, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     gaEventsCookie,
		Value:    url.QueryEscape(string(b)),
		Path:     "/",
		MaxAge:   300,
		SameSite: http.SameSiteLaxMode,
		// Not HttpOnly: header.gohtml reads and clears it from JavaScript.
	})
}
