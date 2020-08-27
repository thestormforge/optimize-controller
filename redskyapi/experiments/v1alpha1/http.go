/*
Copyright 2020 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/redskyops/redskyops-controller/redskyapi"
)

// NewAPI returns a new API implementation for the specified client
func NewAPI(c redskyapi.Client) API {
	return &httpAPI{client: c}
}

type httpAPI struct {
	client redskyapi.Client
}

func (h *httpAPI) Options(ctx context.Context) (ServerMeta, error) {
	u := h.client.URL(endpointExperiment).String()
	sm := ServerMeta{}

	req, err := http.NewRequest(http.MethodOptions, u, nil)
	if err != nil {
		return sm, err
	}

	// We actually want to do OPTIONS for the whole server, now that the host:port has been captured, overwrite the RequestURL
	// TODO This isn't working because of backend configuration issues
	// req.URL.Opaque = "*"

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return sm, err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent,
		// TODO Current behavior is to return 404 or 405 instead of 204 (or 200?)
		http.StatusNotFound, http.StatusMethodNotAllowed:
		sm.Unmarshal(resp.Header)
		return sm, nil
	default:
		return sm, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) GetAllExperiments(ctx context.Context, q *ExperimentListQuery) (ExperimentList, error) {
	u := h.client.URL(endpointExperiment)
	u.RawQuery = q.Encode()

	return h.GetAllExperimentsByPage(ctx, u.String())
}

func (h *httpAPI) GetAllExperimentsByPage(ctx context.Context, u string) (ExperimentList, error) {
	lst := ExperimentList{}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return lst, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return lst, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		metaUnmarshal(resp.Header, &lst.ExperimentListMeta)
		err = json.Unmarshal(body, &lst)
		for i := range lst.Experiments {
			metaUnmarshal(http.Header(lst.Experiments[i].Metadata), &lst.Experiments[i].Experiment.ExperimentMeta)
		}
		return lst, err
	default:
		return lst, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) GetExperimentByName(ctx context.Context, n ExperimentName) (Experiment, error) {
	u := h.client.URL(endpointExperiment + n.Name()).String()
	exp, err := h.GetExperiment(ctx, u)

	// Improve the "not found" error message using the name
	if eerr, ok := err.(*Error); ok && eerr.Type == ErrExperimentNotFound {
		eerr.Message = fmt.Sprintf(`experiment "%s" not found`, n.Name())
	}

	return exp, err
}

func (h *httpAPI) GetExperiment(ctx context.Context, u string) (Experiment, error) {
	e := Experiment{}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return e, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return e, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		metaUnmarshal(resp.Header, &e.ExperimentMeta)
		err = json.Unmarshal(body, &e)
		return e, err
	case http.StatusNotFound:
		return e, newError(ErrExperimentNotFound, resp, body)
	default:
		return e, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) CreateExperiment(ctx context.Context, n ExperimentName, exp Experiment) (Experiment, error) {
	u := h.client.URL(endpointExperiment + n.Name()).String()
	e := Experiment{}

	req, err := httpNewJSONRequest(http.MethodPut, u, exp)
	if err != nil {
		return e, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return e, err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		metaUnmarshal(resp.Header, &e.ExperimentMeta)
		err = json.Unmarshal(body, &e)
		return e, err
	case http.StatusBadRequest:
		return e, newError(ErrExperimentNameInvalid, resp, body)
	case http.StatusConflict:
		return e, newError(ErrExperimentNameConflict, resp, body)
	case http.StatusUnprocessableEntity:
		return e, newError(ErrExperimentInvalid, resp, body)
	default:
		return e, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) DeleteExperiment(ctx context.Context, u string) error {
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return newError(ErrExperimentNotFound, resp, body)
	default:
		return newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) GetAllTrials(ctx context.Context, u string, q *TrialListQuery) (TrialList, error) {
	lst := TrialList{}

	rawQuery := q.Encode()
	if rawQuery != "" {
		if uu, err := url.Parse(u); err == nil {
			// TODO Should we be merging the query in this case?
			uu.RawQuery = rawQuery
			u = uu.String()
		}
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return lst, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return lst, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		err = json.Unmarshal(body, &lst)
		for i := range lst.Trials {
			metaUnmarshal(http.Header(lst.Trials[i].Metadata), &lst.Trials[i].TrialAssignments.TrialMeta)
		}
		return lst, err
	default:
		return lst, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) CreateTrial(ctx context.Context, u string, asm TrialAssignments) (TrialAssignments, error) {
	ta := TrialAssignments{}

	req, err := httpNewJSONRequest(http.MethodPost, u, asm)
	if err != nil {
		return ta, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return ta, err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		metaUnmarshal(resp.Header, &ta.TrialMeta)
		err = json.Unmarshal(body, &ta)
		return ta, nil // TODO Stop ignoring this when the server starts sending a response body
	case http.StatusConflict:
		return ta, newError(ErrExperimentStopped, resp, body)
	case http.StatusUnprocessableEntity:
		return ta, newError(ErrTrialInvalid, resp, body)
	default:
		return ta, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) NextTrial(ctx context.Context, u string) (TrialAssignments, error) {
	asm := TrialAssignments{}

	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return asm, err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return asm, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		metaUnmarshal(resp.Header, &asm.TrialMeta)
		err = json.Unmarshal(body, &asm)
		return asm, err
	case http.StatusGone:
		return asm, newError(ErrExperimentStopped, resp, body)
	case http.StatusServiceUnavailable:
		return asm, newError(ErrTrialUnavailable, resp, body)
	default:
		return asm, newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) ReportTrial(ctx context.Context, u string, vls TrialValues) error {
	if vls.Failed {
		vls.Values = nil
	}

	req, err := httpNewJSONRequest(http.MethodPost, u, vls)
	if err != nil {
		return err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return newError(ErrTrialNotFound, resp, body)
	case http.StatusConflict:
		return newError(ErrTrialAlreadyReported, resp, body)
	case http.StatusUnprocessableEntity:
		return newError(ErrTrialInvalid, resp, body)
	default:
		return newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) AbandonRunningTrial(ctx context.Context, u string) error {
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return newError(ErrTrialNotFound, resp, body)
	default:
		return newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) LabelExperiment(ctx context.Context, u string, lbl ExperimentLabels) error {
	req, err := httpNewJSONRequest(http.MethodPost, u, lbl)
	if err != nil {
		return err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return newError(ErrTrialNotFound, resp, body)
	case http.StatusUnprocessableEntity:
		return newError(ErrTrialInvalid, resp, body)
	default:
		return newError(ErrUnexpected, resp, body)
	}
}

func (h *httpAPI) LabelTrial(ctx context.Context, u string, lbl TrialLabels) error {
	req, err := httpNewJSONRequest(http.MethodPost, u, lbl)
	if err != nil {
		return err
	}

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return newError(ErrTrialNotFound, resp, body)
	case http.StatusUnprocessableEntity:
		return newError(ErrTrialInvalid, resp, body)
	default:
		return newError(ErrUnexpected, resp, body)
	}
}

// httpNewJSONRequest returns a new HTTP request with a JSON payload
func httpNewJSONRequest(method, u string, body interface{}) (*http.Request, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, u, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, err
}

// newError returns a new error with an API specific error condition, it also captures the details of the response
func newError(t ErrorType, resp *http.Response, body []byte) error {
	err := &Error{Type: t}

	// Unmarshal the response body into the error to get the server supplied error message
	// TODO We should be comparing compatible media types here (e.g. charset)
	if resp.Header.Get("Content-Type") == "application/json" {
		_ = json.Unmarshal(body, err)
	}

	// Capture the URL of the request
	if resp.Request != nil && resp.Request.URL != nil {
		err.Location = resp.Request.URL.String()
	}

	// Capture the Retry-After header for "service unavailable"
	if resp.StatusCode == http.StatusServiceUnavailable {
		if ra, raerr := strconv.Atoi(resp.Header.Get("Retry-After")); raerr == nil {
			if ra < 1 {
				ra = 5
			} else if ra > 120 {
				ra = 120
			}
			err.RetryAfter = time.Duration(ra) * time.Second
		}
	}

	// Try to report a more specific error if the error was undocumented (e.g. came from a proxy)
	if err.Type == ErrUnexpected {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			err.Type = ErrUnauthorized
			if err.Message == "" {
				err.Message = "unauthorized"
			}
		case http.StatusPaymentRequired:
			err.Type = ErrUnauthorized
			if err.Message == "" {
				err.Message = "account is not activated"
			}
		default:
			if err.Message == "" {
				err.Message = fmt.Sprintf("unexpected server response (%s)", http.StatusText(resp.StatusCode))
			}
		}
	}

	// Make sure we have a message
	if err.Message == "" {
		switch resp.StatusCode {
		case http.StatusNotFound:
			err.Message = fmt.Sprintf("not found: %s", err.Location)
		default:
			err.Message = strings.ReplaceAll(string(err.Type), "-", " ")
		}
	}

	return err
}

// Extract metadata from the response headers, failures are silently ignored, always call before extracting entity body
func metaUnmarshal(header http.Header, meta Meta) {
	if location := header.Get("Location"); location != "" {
		meta.SetLocation(location)
	}

	if text := header.Get("Last-Modified"); text != "" {
		if lastModified, err := http.ParseTime(text); err == nil {
			meta.SetLastModified(lastModified)
		}
	}

	for _, rh := range header[http.CanonicalHeaderKey("Link")] {
		for _, h := range strings.Split(rh, ",") {
			var link, rel string
			for _, l := range strings.Split(h, ";") {
				l = strings.Trim(l, " ")
				if l == "" {
					continue
				}

				if l[0] == '<' && l[len(l)-1] == '>' {
					link = strings.Trim(l, "<>")
					continue
				}

				p := strings.SplitN(l, "=", 2)
				if len(p) == 2 && strings.ToLower(p[0]) == "rel" {
					rel = strings.Trim(p[1], "\"")
					continue
				}
			}
			meta.SetLink(rel, link)
		}
	}
}
