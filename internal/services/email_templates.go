package services

import "fmt"

type EmailTemplateData struct {
	LogoURL        string
	DefaultLogoURL string
	BusinessName   string
	Heading        string
	Intro          string
	ButtonText     string
	ButtonURL      string
	BodyHTML       string
	Footer         string
}

func BuildBrandedEmail(data EmailTemplateData) string {
	logo := data.LogoURL
	if logo == "" {
		logo = data.DefaultLogoURL
	}

	logoHTML := ""
	if logo != "" {
		logoHTML = fmt.Sprintf(`
			<div style="margin-bottom:20px;">
				<img src="%s" alt="%s" style="max-height:60px;max-width:220px;display:block;">
			</div>
		`, logo, data.BusinessName)
	}

	footer := data.Footer
	if footer == "" {
		footer = data.BusinessName
	}

	buttonHTML := ""
	if data.ButtonText != "" && data.ButtonURL != "" {
		buttonHTML = fmt.Sprintf(`
			<div style="margin:28px 0;">
				<a href="%s" style="display:inline-block;padding:12px 18px;background:#059669;color:#ffffff;text-decoration:none;border-radius:10px;font-weight:600;">
					%s
				</a>
			</div>
		`, data.ButtonURL, data.ButtonText)
	}

	return fmt.Sprintf(`
		<div style="margin:0;padding:32px;background:#f8fafc;font-family:Arial,sans-serif;color:#0f172a;">
			<div style="max-width:640px;margin:0 auto;background:#ffffff;border:1px solid #e2e8f0;border-radius:20px;overflow:hidden;">
				<div style="padding:32px;">
					%s
					<div style="font-size:28px;font-weight:700;line-height:1.2;margin-bottom:12px;color:#0f172a;">
						%s
					</div>
					<div style="font-size:15px;line-height:1.7;color:#475569;margin-bottom:8px;">
						%s
					</div>
					%s
					<div style="font-size:15px;line-height:1.7;color:#334155;">
						%s
					</div>
				</div>

				<div style="padding:20px 32px;background:#f8fafc;border-top:1px solid #e2e8f0;font-size:13px;color:#64748b;">
					Sent by %s
				</div>
			</div>
		</div>
	`,
		logoHTML,
		data.Heading,
		data.Intro,
		buttonHTML,
		data.BodyHTML,
		footer,
	)
}
