package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	custom_i18n "pvmss/i18n"
	"pvmss/logger"
)

// Render executes a template with a request-specific translation function.
func Render(w io.Writer, r *http.Request, tmpl *template.Template, name string, data map[string]interface{}) error {
	localizer := custom_i18n.GetLocalizerFromRequest(r)

	instance, err := tmpl.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone template: %w", err)
	}

	funcMap := template.FuncMap{
		"T": func(messageID string, args ...interface{}) template.HTML {
			msg := messageID
			var localized string

			config := &i18n.LocalizeConfig{MessageID: messageID}
			if len(args) > 0 {
				if count, ok := args[0].(int); ok {
					config.PluralCount = count
				}
			}

			localized, err := localizer.Localize(config)
			if err == nil && localized != "" {
				msg = localized
			}

			return template.HTML(msg)
		},
	}

	instance.Funcs(funcMap)

	targetTemplate := instance.Lookup(name)
	if targetTemplate == nil {
		return fmt.Errorf("template not found: %s", name)
	}

	var buf bytes.Buffer
	if err := instance.ExecuteTemplate(&buf, name, data); err != nil {
		logger.Get().Error().Err(err).Str("template", name).Msg("Error executing template")
		return err
	}

	_, err = w.Write(buf.Bytes())
	return err
}
