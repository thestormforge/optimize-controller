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

package metric

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	optimizev1beta2 "github.com/thestormforge/optimize-controller/v2/api/v1beta2"
)

const query = `
	query($accountId: Int!, $nrqlQuery: Nrql!) {
		actor {
			account(id: $accountId) {
				nrql(query: $nrqlQuery, timeout: 5) {
					results
					totalResult
				}
			}
		}
	}`

func captureNewRelicMetric(m *optimizev1beta2.Metric, startTime, completionTime time.Time) (float64, float64, error) {
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		return 0, 0, errors.New("NEW_RELIC_API_KEY environment variable missing")
	}

	envAccountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if envAccountID == "" {
		return 0, 0, errors.New("NEW_RELIC_ACCOUNT_ID environment variable missing")
	}

	accountID, err := strconv.Atoi(strings.TrimSpace(envAccountID))
	if err != nil {
		return 0, 0, errors.New("invalid account id, must be a number")
	}

	client, err := newrelic.New(newrelic.ConfigPersonalAPIKey(strings.TrimSpace(apiKey)))
	if err != nil {
		return 0, 0, err
	}

	nrql := fmt.Sprintf("%s SINCE %d UNTIL %d", m.Query, startTime.UTC().Unix(), completionTime.UTC().Unix())
	variables := map[string]interface{}{
		"accountId": accountID,

		// Add query timestamp bounds
		// Seems like there is a requirement for timestamp being in UTC --
		// https://docs.newrelic.com/docs/query-your-data/nrql-new-relic-query-language/get-started/nrql-syntax-clauses-functions/#sel-since
		"nrqlQuery": nrql,
	}

	resp := gqlNrglQueryResponse{}
	if err := client.NerdGraph.QueryWithResponse(query, variables, &resp); err != nil {
		return 0, 0, err
	}

	switch len(resp.Actor.Account.NRQL.Results) {
	case 0:
		// TODO: see if we can inject a delay/retry here
		// We cant use CaptureError as is today because it causes retry counter to not decrement
		// return 0, 0, &CaptureError{Message: "metric data not available", Query: nrql, RetryAfter: 30 * time.Second}
		return 0, 0, fmt.Errorf("query returned no results: %s", nrql)
	case 1:
		// Successful case
	default:
		// Maybe note about using RAW instead of TIMESERIES (?)
		return 0, 0, fmt.Errorf("expected one result")
	}

	// This feels janky, but not sure how they do a transformation of the query into the
	// key returned in the results
	// Ex, SELECT sum(`k8s.container.memoryRequestedBytes`) returns
	// "sum.k8s.container.memoryRequestedBytes": 143957950464
	var result float64
	for _, v := range resp.Actor.Account.NRQL.Results[0] {
		result = v.(float64)
	}

	return result, math.NaN(), nil
}

// Using the pattern provided by NewRelic instead of dealing with map[string]interface casts
// ref: https://github.com/newrelic/newrelic-client-go/blob/2932f5d6275d9017fd1ce5764d2de258575dd187/pkg/nrdb/nrdb_query.go#L50
type gqlNrglQueryResponse struct {
	Actor struct {
		Account struct {
			NRQL nrdb.NRDBResultContainer
		}
	}
}

// For reference, using https://api.newrelic.com/graphiql
// with this query
/*
{
  actor {
    account(id: xxx) {
      nrql(query: "SELECT sum(`k8s.container.memoryRequestedBytes`) FROM Metric FACET `k8s.containerName` WHERE `k8s.containerName` = 'postgres' SINCE 10 MINUTES AGO RAW", timeout: 5) {
			  results
        totalResult
      }
    }
  }
}
*/
//
// returns this result
//
/*
{
  "data": {
    "actor": {
      "account": {
        "nrql": {
          "nrql": "SELECT sum(`k8s.container.memoryRequestedBytes`) FROM Metric FACET `k8s.containerName` WHERE `k8s.containerName` = 'postgres' SINCE 10 MINUTES AGO RAW",
          "results": [
            {
              "facet": "postgres",
              "k8s.containerName": "postgres",
              "sum.k8s.container.memoryRequestedBytes": 143957950464
            }
          ],
          "totalResult": {
            "sum.k8s.container.memoryRequestedBytes": 143957950464
          }
        }
      }
    }
  }
}
*/
