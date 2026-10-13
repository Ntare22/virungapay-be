package mail

import (
	_ "embed"

	"github.com/aymerick/raymond"
)

//go:embed templates/welcome.hbs
var welcomeTemplate string

// Welcome is the data for the operator welcome email.
type Welcome struct {
	FirstName string
	FullName  string
	VerifyURL string
}

// RenderWelcome fills the Handlebars welcome template.
func RenderWelcome(data Welcome) (string, error) {
	return raymond.Render(welcomeTemplate, map[string]string{
		"firstName": data.FirstName,
		"fullName":  data.FullName,
		"verifyURL": data.VerifyURL,
	})
}
