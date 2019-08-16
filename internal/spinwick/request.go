package spinwick

// Request tracks information about a given SpinWick request.
type Request struct {
	InstallationID string
	Error          error
	ReportError    bool
	Aborted        bool
}

// WithInstallationID updates the installation ID of a Request.
func (r *Request) WithInstallationID(id string) *Request {
	r.InstallationID = id
	return r
}

// WithError updates the error of a Request.
func (r *Request) WithError(err error) *Request {
	r.Error = err
	return r
}

// ShouldReportError marks the request's error to be reported.
func (r *Request) ShouldReportError() *Request {
	r.ReportError = true
	return r
}

// IntentionalAbort marks the request as being aborted intentionally.
func (r *Request) IntentionalAbort() *Request {
	r.Aborted = true
	return r
}
