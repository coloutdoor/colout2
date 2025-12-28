package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type PageData struct {
	PageTitle string
}

type ContactForm struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone,omitempty"`
	Project string `json:"project"`
	Message string `json:"message"`
}

var sg *sendgrid.Client

func init() {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	if apiKey == "" {
		log.Fatal("Unable to send email: SENDGRID_API_KEY is required")
	}
	sg = sendgrid.NewSendClient(apiKey)
}

func contactHandler(w http.ResponseWriter, r *http.Request) {

	// POST Response!!!!
	if r.Method == "POST" {
		// Simple form handling (expand with email, DB, etc.)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Start Captcha
		cfSecretKey := os.Getenv("CLOUDFLARE_SECRET_KEY")
		if cfSecretKey == "" {
			log.Fatal("CF Captcha missing secret key")
		}

		// In your POST handler
		token := r.FormValue("cf-turnstile-response")
		if token == "" {
			// Reject: missing token
			log.Printf("Missing CF token from POST")
			return
		}

		resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
			"secret":   {cfSecretKey}, // TODO - Replace with SECRET KEY
			"response": {token},
			"remoteip": {r.RemoteAddr},
		})
		if err != nil { /* handle error */
			// Reject: BOT
			log.Printf("Cloudflare Post failed!.")
			return
		}

		cf_body, _ := io.ReadAll(resp.Body)
		var result struct {
			Success bool `json:"success"`
		}
		json.Unmarshal(cf_body, &result)

		if !result.Success {
			// Spam/bot — reject or log
			log.Printf("Captcha Failed. Bot - %v", cf_body)
			return
		}
		// End Captcha - Cloudflare

		data := ContactForm{
			Name:    r.FormValue("name"),
			Email:   r.FormValue("email"),
			Phone:   r.FormValue("phone"),
			Project: r.FormValue("project"),
			Message: r.FormValue("message"),
		}

		from := mail.NewEmail("Columbia Outdoor", "support@columbiaoutdoor.com")
		toTeam := mail.NewEmail("Team - CO", "support@columbiaoutdoor.com")
		replyTo := mail.NewEmail(data.Name, data.Email)
		// 1. Email to your team (rich HTML)
		htmlContent := `
		<h2>New Contact Form Submission</h2>
		<p><strong>Name:</strong> {{.Name}}</p>
		<p><strong>Email:</strong> {{.Email}}</p>
		<p><strong>Phone:</strong> {{.Phone}}</p>
		<p><strong>Project Type:</strong> {{.Project}}</p>
		<p><strong>Message:</strong><br>{{.Message}}</p>
		<hr>
		<small>Sent from columbiaoutdoor.com – Pacific Northwest’s trusted outdoor living platform</small>
	`
		t := template.Must(template.New("email").Funcs(template.FuncMap{
			"replace": func(s, old, new string) string { return strings.ReplaceAll(s, old, new) },
		}).Parse(htmlContent))

		var body bytes.Buffer
		t.Execute(&body, data)

		teamMessage := mail.NewSingleEmail(from, "New Lead – "+data.Name, toTeam, "", body.String())
		teamMessage.SetReplyTo(replyTo)

		// Auto-reply using your Dynamic Template (replace with your real template ID)
		visitorMessage := mail.NewV3Mail()
		visitorMessage.SetFrom(from)
		visitorMessage.SetTemplateID("d-e9a41a151cec4963a7454f4678deb030") // SendGrid Dynamic Template

		p := mail.NewPersonalization()
		p.AddTos(mail.NewEmail(data.Name, data.Email))
		p.SetDynamicTemplateData("name", data.Name)
		p.SetDynamicTemplateData("project", data.Project)
		visitorMessage.AddPersonalizations(p)

		// Send both emails in background
		go func() {
			teamRR, err := sg.Send(teamMessage)
			if err != nil {
				log.Printf("Team email failed: %v", err)
			}
			if teamRR.StatusCode != 200 {
				log.Printf("Team Error: send response is: %v", teamRR)
			}

			visitorRR, err := sg.Send(visitorMessage)
			if err != nil {
				log.Printf("Auto-reply failed: %v", err)
			}
			if visitorRR.StatusCode != 200 {
				log.Printf("visitor send response is: %v", visitorRR)
			}

		}()

		// Redirect to nice thank-you page
		http.Redirect(w, r, "/contact?sent=1", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.New("contact.html").
		Funcs(funcMap).
		ParseFiles("templates/contact.html", "templates/header.html", "templates/footer.html"))

	data := PageData{PageTitle: "Contact Us"}
	if r.URL.Query().Get("sent") == "1" {
		data.PageTitle = "Thank You – Message Sent!"
	}

	userAuth := getUserAuth(r)
	userAuth.Title = "Contact Us"
	userAuth.Subtitle = "For any outdoor deck, patio, cover.  One of our experts will get in touch with you soon."
	userAuth.MetaDesc = "Contact us today for a quick and easy estimate for Timbertech, Trex, or wood deck."
	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "contact.html", rd); err != nil {
		http.Error(w, "Server Error", 500)
		log.Printf("contact error: %v", err)
	}
}
