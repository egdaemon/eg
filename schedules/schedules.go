// Package schedules manages the workloads the control plane runs on a schedule for a repository.
// the functions take an http client authorized for the compute api, see authn.New.
package schedules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

func endpoint() string {
	return fmt.Sprintf("%s/r/schedule", eg.EnvAPIHostDefault())
}

// Create the schedule, the control plane assigns its id and computes its next run.
func Create(ctx context.Context, c *http.Client, s *Schedule) (_ *Schedule, err error) {
	var (
		out ScheduleCreateResponse
	)

	if err = do(ctx, c, http.MethodPost, endpoint()+"/", &ScheduleCreateRequest{Schedule: s}, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to create schedule")
	}

	return out.GetSchedule(), nil
}

// Search a single page of schedules. the response's next carries the offset of the following page,
// zero once there are no more pages.
func Search(ctx context.Context, c *http.Client, req *ScheduleSearchRequest) (_ *ScheduleSearchResponse, err error) {
	var (
		out ScheduleSearchResponse
		q   = url.Values{
			"query":   {req.GetQuery()},
			"vcs_uri": {req.GetVcsUri()},
			"offset":  {strconv.FormatUint(req.GetOffset(), 10)},
			"limit":   {strconv.FormatUint(req.GetLimit(), 10)},
		}
	)

	if err = do(ctx, c, http.MethodGet, endpoint()+"/?"+q.Encode(), nil, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to search schedules")
	}

	return &out, nil
}

// Find pages through the schedules of vcsuri until it locates id, the control plane has no lookup by id.
// an empty vcsuri searches every schedule of the account. returns nil when the schedule does not exist.
func Find(ctx context.Context, c *http.Client, vcsuri string, id string) (_ *Schedule, err error) {
	var (
		page *ScheduleSearchResponse
		req  = &ScheduleSearchRequest{VcsUri: vcsuri, Limit: 100}
	)

	for {
		if page, err = Search(ctx, c, req); err != nil {
			return nil, err
		}

		for _, s := range page.GetItems() {
			if s.GetId() == id {
				return s, nil
			}
		}

		if page.GetNext().GetOffset() == 0 {
			return nil, nil
		}

		req = page.GetNext()
	}
}

// Update replaces the schedule. omitted fields are reset to their defaults and the next run is computed
// from lastrun_at, send the full record.
func Update(ctx context.Context, c *http.Client, id string, s *Schedule) (_ *Schedule, err error) {
	var (
		out ScheduleUpdateResponse
	)

	if err = do(ctx, c, http.MethodPatch, endpoint()+"/"+url.PathEscape(id), &ScheduleUpdateRequest{Schedule: s}, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to update schedule")
	}

	return out.GetSchedule(), nil
}

// Remove the schedule with the id.
func Remove(ctx context.Context, c *http.Client, id string) error {
	return errorsx.Wrap(do(ctx, c, http.MethodDelete, endpoint()+"/"+url.PathEscape(id), nil, nil), "unable to remove schedule")
}

func do(ctx context.Context, c *http.Client, method string, target string, in any, out any) (err error) {
	var (
		encoded []byte
		req     *http.Request
		resp    *http.Response
	)

	if in != nil {
		if encoded, err = json.Marshal(in); err != nil {
			return errorsx.Wrap(err, "unable to encode request")
		}
	}

	if req, err = http.NewRequestWithContext(ctx, method, target, bytes.NewReader(encoded)); err != nil {
		return errorsx.Wrap(err, "unable to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpx.AsError(c.Do(req))
	defer func() { errorsx.Log(httpx.AutoClose(resp)) }()
	if err != nil {
		return err
	}

	if out == nil {
		return nil
	}

	return errorsx.Wrap(json.NewDecoder(resp.Body).Decode(out), "unable to decode response")
}
