package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	resend "github.com/resend/resend-go/v2"
)

type PageData struct {
	PageTitle string
	Sent      bool
}

type ContactForm struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone,omitempty"`
	Project string `json:"project"`
	Message string `json:"message"`
}

var resendClient *resend.Client

func init() {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Fatal("RESEND_API_KEY is required — email will not work without it")
	}
	resendClient = resend.NewClient(apiKey)
}

func contactHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Cloudflare Turnstile CAPTCHA
		cfSecretKey := os.Getenv("CLOUDFLARE_SECRET_KEY")
		if cfSecretKey == "" {
			log.Fatal("CF Captcha missing secret key")
		}
		token := r.FormValue("cf-turnstile-response")
		if token == "" {
			log.Printf("Missing CF token from POST")
			return
		}
		resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
			"secret":   {cfSecretKey},
			"response": {token},
			"remoteip": {r.RemoteAddr},
		})
		if err != nil {
			log.Printf("Cloudflare Post failed: %v", err)
			return
		}
		cfBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var result struct {
			Success bool `json:"success"`
		}
		json.Unmarshal(cfBody, &result)
		if !result.Success {
			log.Printf("Captcha failed — possible bot: %s", cfBody)
			return
		}

		data := ContactForm{
			Name:    r.FormValue("name"),
			Email:   r.FormValue("email"),
			Phone:   r.FormValue("phone"),
			Project: r.FormValue("project"),
			Message: r.FormValue("message"),
		}

		// Build team notification HTML
		teamHTML := fmt.Sprintf(`
<h2>New Contact Form Submission</h2>
<p><strong>Name:</strong> %s</p>
<p><strong>Email:</strong> %s</p>
<p><strong>Phone:</strong> %s</p>
<p><strong>Project Type:</strong> %s</p>
<p><strong>Message:</strong><br>%s</p>
<hr>
<small>Sent from columbiaoutdoor.com</small>`,
			data.Name, data.Email, data.Phone, data.Project,
			strings.ReplaceAll(data.Message, "\n", "<br>"))

		// Build visitor auto-reply HTML
		visitorHTML := fmt.Sprintf(`
<h2>Thanks for reaching out, %s!</h2>
<p>We received your message about your <strong>%s</strong> project and will be in touch shortly.</p>
<p>In the meantime, feel free to explore our <a href="https://columbiaoutdoor.com/calc?option=deck">free deck estimator</a>.</p>
<br>
<p>— The Columbia Outdoor Team<br>(360) 787-8062 · columbiaoutdoor.com</p>`,
			data.Name, data.Project)

		// Build plain text for team email template
		htmlTmpl := template.Must(template.New("email").Funcs(template.FuncMap{
			"replace": func(s, old, newS string) string { return strings.ReplaceAll(s, old, newS) },
		}).Parse(teamHTML))
		var body bytes.Buffer
		htmlTmpl.Execute(&body, data)

		go func() {
			// Team notification
			_, err := resendClient.Emails.Send(&resend.SendEmailRequest{
				From:    "Columbia Outdoor <support@columbiaoutdoor.com>",
				To:      []string{"support@columbiaoutdoor.com"},
				ReplyTo: data.Email,
				Subject: "New Lead — " + data.Name,
				Html:    body.String(),
			})
			if err != nil {
				log.Printf("Contact: team email failed: %v", err)
			}

			// Visitor auto-reply
			_, err = resendClient.Emails.Send(&resend.SendEmailRequest{
				From:    "Columbia Outdoor <support@columbiaoutdoor.com>",
				To:      []string{data.Email},
				Subject: "We received your message — Columbia Outdoor",
				Html:    visitorHTML,
			})
			if err != nil {
				log.Printf("Contact: visitor reply failed: %v", err)
			}
		}()

		http.Redirect(w, r, "/contact?sent=1", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.New("contact.gohtml").
		Funcs(funcMap).
		ParseFiles("templates/contact.gohtml", "templates/header.gohtml", "templates/footer.gohtml"))

	data := PageData{PageTitle: "Contact Us"}
	if r.URL.Query().Get("sent") == "1" {
		data.PageTitle = "Thank You – Message Sent!"
		data.Sent = true
	}

	userAuth := getUserAuth(r, w)
	userAuth.Title = "Contact Us"
	userAuth.Subtitle = "For any outdoor deck, patio, cover. One of our experts will get in touch with you soon."
	userAuth.MetaDesc = "Contact us today for a quick and easy estimate for Timbertech, Trex, or wood deck."
	rd := renderData{
		Page:   &data,
		Header: &userAuth,
	}
	if err := tmpl.ExecuteTemplate(w, "contact.gohtml", rd); err != nil {
		http.Error(w, "Server Error", 500)
		log.Printf("contact error: %v", err)
	}
}
