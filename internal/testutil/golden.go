// Copyright 2020 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package testutil

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var updateRegex = flag.String("update", "", "Update testdata of tests matching the given regex")

// Update returns true if update regex mathces given test name.
func Update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

// AssertGolden compares what's got and what's in the golden file. It updates
// the golden file on-demand.
func AssertGolden(t testing.TB, path string, update bool, got interface{}) {
	t.Helper()

	data := marshal(t, got)

	if update {
		if err := ioutil.WriteFile(path, data, 0640); err != nil {
			t.Fatalf("update golden file %q: %s", path, err)
		}
	}

	golden, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %q: %s", path, err)
	}

	if diff := cmp.Diff(string(golden), string(data)); diff != "" {
		t.Fatal(diff)
	}
}

func marshal(t testing.TB, v interface{}) []byte {
	t.Helper()

	switch v2 := v.(type) {
	case string:
		return []byte(v2)
	case []byte:
		return v2
	default:
		data, err := json.MarshalIndent(v, " ", " ")
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
}
