package main

import (
	"encoding/json"
	"testing"
)

func Test_processJSON(t *testing.T) {
	// object
	var data = []byte(`{"str1":"string","num1":0,"num2":0,"str2":"string", "bool1": true, "object1": {"a": {"a": 1}, "b": 2}}`)
	d := processJSON(data)
	r, err := json.Marshal(d)
	if err != nil {
		t.Fatal(r)
	}
	if len(r) != 283 {
		t.Fatal(len(r))
	}
	// array
	data = []byte(`[{"a": {"b": 1}},{"a": {"b": 2}}]`)
	d = processJSON(data)
	r, _ = json.Marshal(d)
	if err != nil {
		t.Fatal(r)
	}
	if len(r) != 116 {
		t.Fatal(len(r))
	}
}
