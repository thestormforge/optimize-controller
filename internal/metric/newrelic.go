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
	"github.com/newrelic/newrelic-client-go/pkg/nerdgraph"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
)

func captureNewRelicMetric(m *redskyv1beta1.Metric, startTime, completionTime time.Time) (float64, float64, error) {
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

	query := `
	query($accountId: Int!, $nrqlQuery: Nrql!) {
		actor {
			account(id: $accountId) {
				nrql(query: $nrqlQuery, timeout: 5) {
					totalResult
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"accountId": accountID,

		// Add query timestamp bounds
		// Seems like there is a requirement for timestamp being in UTC --
		// https://docs.newrelic.com/docs/query-your-data/nrql-new-relic-query-language/get-started/nrql-syntax-clauses-functions/#sel-since
		"nrqlQuery": fmt.Sprintf("%s SINCE %d UNTIL %d", m.Query, startTime.UTC().Unix(), completionTime.UTC().Unix()),
	}

	fmt.Println(variables)

	resp, err := client.NerdGraph.Query(query, variables)
	if err != nil {
		return 0, 0, err
	}

	queryResp := resp.(nerdgraph.QueryResponse)
	actor := queryResp.Actor.(map[string]interface{})
	account := actor["account"].(map[string]interface{})
	nrql := account["nrql"].(map[string]interface{})
	results := nrql["totalResult"].(map[string]interface{})

	if len(results) != 1 {
		// Maybe note about using RAW instead of TIMESERIES (?)
		return 0, 0, fmt.Errorf("expected one series")
	}

	// TODO
	// How do we ensure they have data from our scrape interval ( akin to prometheus )

	// This feels janky, but not sure how they do a transformation of the query into the
	// key returned in the results
	// Ex, SELECT sum(`k8s.container.memoryRequestedBytes`) returns
	// "sum.k8s.container.memoryRequestedBytes": 143957950464

	var result float64
	for _, v := range results {
		result = v.(float64)
	}

	return result, math.NaN(), nil
}

// For reference, using https://api.newrelic.com/graphiql
// with this query
/*
{
  actor {
    account(id: xxx) {
      nrql(query: "SELECT sum(`k8s.container.memoryRequestedBytes`) FROM Metric FACET `k8s.containerName` WHERE `k8s.containerName` = 'postgres' SINCE 10 MINUTES AGO RAW", timeout: 5) {
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
          "otherResult": {
            "sum.k8s.container.memoryRequestedBytes": 0
          },
          "totalResult": {
            "sum.k8s.container.memoryRequestedBytes": 143957950464
          }
        }
      }
    }
  }
}
*/
