package reporter

import (
	"io"
	"text/template"

	"github.com/aquilax/hranoprovod-cli/v2/shared"
)

const summaryTemplate = `{{formatDate .Time}} :
{{- if .Totals }}
{{- range $total := .Totals }}
{{ formatValue $total.Positive }} : {{ $total.Name }}
{{- end}}
{{- end}}
------------
{{- if .Elements }}
{{- range $el := .Elements}}
{{ formatValue $el.Value }} : {{ $el.Name }}
{{- end}}
{{- end}}
`

// SummaryReporterTemplate is a summary reporter
type SummaryReporterTemplate struct {
	config   Config
	db       shared.DBNodeList
	output   io.Writer
	template *template.Template
}

// NewSummaryReporterTemplate creates new summary reporter
func NewSummaryReporterTemplate(config Config, db shared.DBNodeList) *SummaryReporterTemplate {
	return &SummaryReporterTemplate{
		config,
		db,
		config.Output,
		template.Must(template.New("summary").Funcs(getTemplateFunctions(config)).Parse(summaryTemplate)),
	}
}

// Process process shared node
func (r *SummaryReporterTemplate) Process(ln *shared.LogNode) error {
	return r.template.Execute(r.output, getReportItem(ln, r.db, r.config))
}

// Flush does nothing
func (r *SummaryReporterTemplate) Flush() error {
	return nil
}
