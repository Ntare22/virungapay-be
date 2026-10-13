package mail

import (
	_ "embed"

	"github.com/aymerick/raymond"
)

//go:embed templates/standard.hbs
var standardTemplate string

// Standard is the branded shell used for account and sign-in email.
type Standard struct {
	Eyebrow  string
	Heading  string
	Body     string
	Code     string
	Footnote string
}

// RenderStandard fills the shared VirungaPay email layout.
func RenderStandard(data Standard) (string, error) {
	return raymond.Render(standardTemplate, map[string]string{
		"eyebrow":  data.Eyebrow,
		"heading":  data.Heading,
		"body":     data.Body,
		"code":     data.Code,
		"footnote": data.Footnote,
	})
}
