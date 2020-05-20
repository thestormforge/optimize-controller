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
	sm := ServerMeta{}
	u := h.client.URL(endpointExperiment).String()

	req, err := http.NewRequest(http.MethodOptions, u, nil)
	if err != nil {
		return sm, err
	}

	// We actually want to do OPTIONS for the whole server, now that the host:port has been captured, overwrite the RequestURL
	// TODO Change this back to `req.URL.Opaque = "*"`?

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return sm, err
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		sm.Unmarshal(resp.Header)
		return sm, nil
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		// TODO Current behavior is to return 404 or 405 instead of 204 (or 200?)
		sm.Unmarshal(resp.Header)
		return sm, nil
	default:
		return sm, unexpected(resp, body)
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
		return lst, unexpected(resp, body)
	}
}

func (h *httpAPI) GetExperimentByName(ctx context.Context, n ExperimentName) (Experiment, error) {
	u := h.client.URL(endpointExperiment + n.Name())
	return h.GetExperiment(ctx, u.String())
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
		return e, &Error{Type: ErrExperimentNotFound}
	default:
		return e, unexpected(resp, body)
	}
}

func (h *httpAPI) CreateExperiment(ctx context.Context, n ExperimentName, exp Experiment) (Experiment, error) {
	e := Experiment{}
	u := h.client.URL(endpointExperiment + url.PathEscape(n.Name()))
	b, err := json.Marshal(exp)
	if err != nil {
		return e, err
	}

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(b))
	if err != nil {
		return e, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return e, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		metaUnmarshal(resp.Header, &e.ExperimentMeta)
		err = json.Unmarshal(body, &e)
		return e, err
	case http.StatusCreated:
		metaUnmarshal(resp.Header, &e.ExperimentMeta)
		err = json.Unmarshal(body, &e)
		return e, err
	case http.StatusBadRequest:
		return e, &Error{Type: ErrExperimentNameInvalid}
	case http.StatusConflict:
		return e, &Error{Type: ErrExperimentNameConflict}
	case http.StatusUnprocessableEntity:
		return e, &Error{Type: ErrExperimentInvalid}
	default:
		return e, unexpected(resp, body)
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
		return &Error{Type: ErrExperimentNotFound}
	default:
		return unexpected(resp, body)
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
		return lst, unexpected(resp, body)
	}
}

func (h *httpAPI) CreateTrial(ctx context.Context, u string, asm TrialAssignments) (string, error) {
	l := ""
	b, err := json.Marshal(asm)
	if err != nil {
		return l, err
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(b))
	if err != nil {
		return l, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return l, err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		l = resp.Header.Get("Location")
		return l, nil
	case http.StatusConflict:
		return l, &Error{Type: ErrExperimentStopped}
	case http.StatusUnprocessableEntity:
		return l, &Error{Type: ErrTrialInvalid}
	default:
		return l, unexpected(resp, body)
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
		return asm, &Error{Type: ErrExperimentStopped}
	case http.StatusServiceUnavailable:
		ra, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil || ra < 1 {
			ra = 5
		} else if ra > 120 {
			ra = 120
		}
		return asm, &Error{Type: ErrTrialUnavailable, RetryAfter: time.Duration(ra) * time.Second}
	default:
		return asm, unexpected(resp, body)
	}
}

func (h *httpAPI) ReportTrial(ctx context.Context, u string, vls TrialValues) error {
	if vls.Failed {
		vls.Values = nil
	}
	b, err := json.Marshal(vls)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return &Error{Type: ErrTrialNotFound}
	case http.StatusConflict:
		return &Error{Type: ErrTrialAlreadyReported}
	case http.StatusUnprocessableEntity:
		return &Error{Type: ErrTrialInvalid}
	default:
		return unexpected(resp, body)
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
		return &Error{Type: ErrTrialNotFound}
	default:
		return unexpected(resp, body)
	}
}

func (h *httpAPI) LabelExperiment(ctx context.Context, u string, lbl ExperimentLabels) error {
	b, err := json.Marshal(lbl)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return &Error{Type: ErrTrialNotFound}
	case http.StatusUnprocessableEntity:
		return &Error{Type: ErrTrialInvalid}
	default:
		return unexpected(resp, body)
	}
}

func (h *httpAPI) LabelTrial(ctx context.Context, u string, lbl TrialLabels) error {
	b, err := json.Marshal(lbl)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, body, err := h.client.Do(ctx, req)
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusNotFound:
		return &Error{Type: ErrTrialNotFound}
	case http.StatusUnprocessableEntity:
		return &Error{Type: ErrTrialInvalid}
	default:
		return unexpected(resp, body)
	}
}

// TODO Unmarshal _expected_ errors to get better messages as well
// TODO Just return nil for any 2xx status codes?
func unexpected(resp *http.Response, body []byte) error {
	err := &Error{Type: ErrUnexpected}

	if resp.Header.Get("Content-Type") == "application/json" {
		// Unmarshal body into the error to get the error message
		_ = json.Unmarshal(body, err)
	}

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
	case http.StatusNotFound:
		if resp.Request != nil && resp.Request.URL != nil {
			err.Message = fmt.Sprintf("not found: %s", resp.Request.URL.String())
		}
	}

	if err.Message == "" {
		err.Message = fmt.Sprintf("unexpected server response: %s", resp.Status)
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
