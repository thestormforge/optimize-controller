/*
Copyright 2019 GramLabs, Inc.

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

package check

import (
	"fmt"
	"math/rand"
)

var (
	left = [...]string{
		"agitated",
		"awesome",
		"bold",
		"cranky",
		"determined",
		"elated",
		"epic",
		"frosty",
		"happy",
		"jolly",
		"nostalgic",
		"quirky",
		"thirsty",
		"vigilant",
	}

	right = [...]string{
		"aimua",
		"bower",
		"gustie",
		"hochadel",
		"idan",
		"perol",
		"platt",
		"provo",
		"sutherland",
	}
)

// GetRandomName returns a randomly generated name, if the number of retries is greater then 0 a small random number is also produced
func GetRandomName(retry int) string {
	name := fmt.Sprintf("%s_%s", left[rand.Intn(len(left))], right[rand.Intn(len(right))])

	if retry > 0 {
		name = fmt.Sprintf("%s%d", name, rand.Intn(10))
	}
	return name
}

var (
	paramsA = [...]string{
		"",
		"max_",
		"min_",
		"target_",
	}

	paramsB = [...]string{
		"memory",
		"cpu",
		"queue",
		"heap",
		"network",
		"database",
	}

	paramsC = [...]string{
		"",
		"_time",
		"_length",
		"_size",
		"_percentage",
		"_capacity",
	}

	metrics = [...]string{
		"cost",
		"errors",
		"excellence",
		"greatness",
		"performance",
		"requests",
		"responsiveness",
		"speed",
		"time",
		"utilization",
		"waits",
	}
)

func GetRandomParameter() string {
	return paramsA[rand.Intn(len(paramsA))] + paramsB[rand.Intn(len(paramsB))] + paramsC[rand.Intn(len(paramsC))]
}

func GetRandomMetric() string {
	return metrics[rand.Intn(len(metrics))]
}

// Helper that attempts to find a unique value from either GetRandomParameter or GetRandomMetric
func getUnique(used map[string]bool, gen func() string) string {
	// We never need more then 20 parameter names and 2 metric names (as indicated by the capacity of the supplied map)
	for i := 0; i < 1000; i++ {
		s := gen()
		if !used[s] {
			used[s] = true
			return s
		}
	}
	panic("unable to produce unique string")
}
