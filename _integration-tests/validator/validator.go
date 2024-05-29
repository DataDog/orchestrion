// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// A Validation is a structure read from validation.json files.
type Validation struct {
	//URL    string
	Output []map[string]interface{}
}

// assembleTraces Reads the traces from the fake agent and returns
// a map structure in the same format as the "output" field of validation.json files.
func assembleTraces(r io.Reader) ([]map[string]interface{}, error) {
	roots := make(map[uint64]map[string]interface{})
	spans := make(map[uint64]map[string]interface{})

	// This is what we get from the fake agent.
	var dec [][]map[string]interface{}
	d := json.NewDecoder(r)
	d.UseNumber()
	err := d.Decode(&dec)
	if err != nil {
		return nil, err
	}

	// Build the entire map of SpanID -> *Span in spans, and the map of roots.
	for _, decSpans := range dec {
		for _, span := range decSpans {
			var (
				parentID uint64
				spanID   uint64
			)
			if pid, ok := span["parent_id"]; ok {
				npid, ok := pid.(json.Number)
				if !ok {
					fmt.Printf("Expected parent_id to be a Number, but it was %#v (%s)\n",
						pid, reflect.TypeOf(pid).String())
					os.Exit(1)
				}
				ipid, err := npid.Int64()
				if err != nil {
					fmt.Printf("Failed to get parent_id as int64: %v\n", err)
					os.Exit(1)
				}
				parentID = uint64(ipid)
			} else {
				fmt.Printf("Received bad span from fake agent with no parent_id: %v", span)
				os.Exit(1)
			}
			if sid, ok := span["span_id"]; ok {
				nsid, ok := sid.(json.Number)
				if !ok {
					fmt.Printf("Expected span_id to be a Number, but it was %#v (%s)\n",
						sid, reflect.TypeOf(sid).String())
					os.Exit(1)
				}
				isid, err := nsid.Int64()
				if err != nil {
					fmt.Printf("Failed to get span_id as int64: %v\n", err)
					os.Exit(1)
				}
				spanID = uint64(isid)
			} else {
				fmt.Printf("Received bad span from fake agent with no parent_id: %v", span)
				os.Exit(1)
			}
			spans[spanID] = span
			if parentID == 0 {
				if roots[spanID] != nil {
					fmt.Printf("Found root span with duplicate IDs:\n\t%v\n\t%v\n",
						roots[spanID], span)
					os.Exit(1)
				}
				roots[spanID] = span
			}
		}
	}

	// Connect the tree of spans
	for _, s := range spans {
		ipid, _ := s["parent_id"].(json.Number).Int64()
		pid := uint64(ipid)
		if pid == 0 {
			continue
		}
		parent := spans[pid]
		if parent == nil {
			fmt.Printf("Found span with no parent present: %v\n", s)
			os.Exit(1)
		}
		if parent["_children"] == nil {
			parent["_children"] = []interface{}{s}
		} else {
			children := parent["_children"].([]interface{})
			parent["_children"] = append(children, s)
		}
	}

	// Create the root slice
	rs := make([]map[string]interface{}, 0, len(roots))
	for _, s := range roots {
		rs = append(rs, s)
	}
	return rs, nil
}

func main() {
	vfile := flag.String("vfile", "", "the validation file to check against the fake agent's output")
	surl := flag.String("surl", "", "the URL to request from the fake agent to retrieve the trace(s) to validate")
	tname := flag.String("tname", "", "the name of the test we are running.")
	flag.Parse()

	if *vfile == "" {
		fmt.Printf("No validation file specified. (-vfile)")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *surl == "" {
		fmt.Printf("No fake agent url specified. (-surl)")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *tname == "" {
		fmt.Printf("No test name specified. (-tname)")
		flag.PrintDefaults()
		os.Exit(1)
	}

	f, err := os.Open(*vfile)
	if err != nil {
		fmt.Printf("Failed to open validation file %q: %v\n", *vfile, err)
		os.Exit(1)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.UseNumber()

	var v Validation
	err = d.Decode(&v)
	if err != nil {
		fmt.Printf("Failed to decode validation from %q: %v\n", *vfile, err)
		os.Exit(1)
	}

	var fakeTraces []map[string]any
	if url, err := url.Parse(*surl); err == nil && url.Scheme == "file" {
		filename := url.Path
		if runtime.GOOS == "windows" {
			filename = filepath.FromSlash(filename[1:]) // The Path would include a leading `/`
		}
		file, err := os.Open(filename)
		if err != nil {
			fmt.Printf("Failed to open traces file %q: %v", url.Path, err)
			os.Exit(1)
		}
		defer file.Close()
		if fakeTraces, err = assembleTraces(file); err != nil {
			fmt.Printf("Failed to decode traces from %q: %v\n", url.Path, err)
			os.Exit(1)
		}
	} else if resp, err := http.Get(*surl); err != nil {
		fmt.Printf("Failed to retrieve traces from the fake agent: %v\n", err)
		os.Exit(1)
	} else {
		defer resp.Body.Close()

		if fakeTraces, err = assembleTraces(resp.Body); err != nil {
			fmt.Printf("Failed to decode traces from the fake agent: %v\n", err)
			os.Exit(1)
		}
	}

	var vkvs [][]kv
	for _, v := range v.Output {
		kvs := genKVs(v)
		vkvs = append(vkvs, kvs)
	}
	var fkvs [][]kv
	for _, v := range fakeTraces {
		kvs := genKVs(v)
		fkvs = append(fkvs, kvs)
	}

	var failed strings.Builder

valid:
	for _, v := range vkvs {
		var (
			closest      [][]kv
			closestLen   int
			closestDiffs []string
		)

		for _, f := range fkvs {
			var d diff
			compare(v, f, "", 0, &d)
			if d.Len() == 0 {
				continue valid
			}
			if closestLen == 0 || d.Len() < closestLen {
				closestLen = d.Len()
				closest = [][]kv{f}
				closestDiffs = []string{d.String()}
			} else if closestLen == d.Len() {
				closest = append(closest, f)
				closestDiffs = append(closestDiffs, d.String())
			}
		}
		fmt.Fprintf(&failed, "Failed to validate Trace:\n")
		DumpKV(&failed, v)
		fmt.Fprintf(&failed, "\nClosest traces(%d) from fake agent:", len(closest))
		for i := 0; i < len(closest); i++ {
			fmt.Fprintf(&failed, "\n\n#################### Candidate %d ####################\n", i)
			DumpKV(&failed, closest[i])
			fmt.Fprintf(&failed, "\nDiff between valid and candidate %d:\n%s", i, closestDiffs[i])
		}
	}
	faileds := failed.String()
	if len(faileds) != 0 {
		fmt.Printf("################################################################################\nFAILED:\n%s\n", failed.String())
		os.Exit(1)
	}
}

// A diff keeps track of the differences between two things.
type diff struct {
	lines []string
}

// Len returns the number of differences added with AddDiffLine
func (d *diff) Len() int {
	return len(d.lines)
}

// AddDifference adds a "difference" to the diff. The arguments are
// as those to printf, and the format string, f, should end in a newline.
func (d *diff) AddDifference(f string, args ...interface{}) {
	s := fmt.Sprintf(f, args...)
	d.lines = append(d.lines, s)
}

// String assembles the differences into a single string that can
// be printed for human consumption.
// It is important to end differences registered with AddDifference
// with newlines so that they are printed correctly.
func (d *diff) String() string {
	var b strings.Builder
	for _, l := range d.lines {
		io.WriteString(&b, l)
	}
	return b.String()
}

// Compare walks through valid and fake, creating a sort of diff between the
// two. Unlike a normal diff, only the things in valid are expected to be in
// fake, and fake can (and almost always does) contain more fields than valid.
//
// The fields are called valid and fake, because the valid argument is meant
// to be sourced from a validation.json file, whereas the fake argument should
// come from the fake agent. Given output from the fake agent, this is a way
// to easily compare the output to the expected output described in a
// validation.json file.
//
// For a given valid[vi], we advance through fake with fi, trying to match
// the field's key. Once we find the valid[vi] key, we compare the values
// including recursively for meta, metrics, and _children.
//
// If valid[vi] is not found, we note that it is missing and start
// looking for valid[vi+1] from fake[lastfi], where lastfi is the
// index of the last valid field we found in fake.
func compare(valid, fake []kv, prefix string, indent int, d *diff) {
	var vi int
	var fi int
	var lastfi int
	for {
		if vi >= len(valid) {
			break
		}
		vkv := valid[vi]
		fkv := fake[fi]
		if fkv.k == vkv.k {
			// We found valid[vi]. Advance vi to the next valid field.
			vi++
			// Save the spot of the last fake field that we found, so we can
			// resume from lastfi if we don't find valid[vi+1]
			lastfi = fi
			if vkv.k == "metrics" {
				compare(vkv.v.([]kv), fkv.v.([]kv), prefix+".metrics", indent+1, d)
			} else if vkv.k == "meta" {
				compare(vkv.v.([]kv), fkv.v.([]kv), prefix+".meta", indent+1, d)
			} else if vkv.k == "_children" {
				vcs := vkv.v.([][]kv)
				fcs := fkv.v.([][]kv)
				if len(vcs) != len(fcs) {
					d.AddDifference("%s._children (valid) count(%d) != %s._children (fake agent) count(%d)\n", prefix, len(vcs), prefix, len(fcs))
				}
				for i, vc := range vcs {
					if len(fcs) <= i {
						d.AddDifference("%s._children[%d] != nil, but %s._children[%d] == nil\n", prefix, i, prefix, i)
						break
					}
					compare(vc, fcs[i],
						fmt.Sprintf("%s._children[%d]", prefix, i),
						indent+1, d)
				}
			} else {
				// The inferred service name may include a `.exe` suffix on Windows!
				if vkv.v != fkv.v && (runtime.GOOS != "windows" || vkv.k != "service" || fmt.Sprintf("%v.exe", vkv.v) != fkv.v) {
					d.AddDifference("validation: %v.%v:%v\nfake agent: %v.%v:%v\n",
						prefix, vkv.k, vkv.v, prefix, fkv.k, fkv.v)
				}
			}
			continue
		}
		fi++
		if fi == len(fake) {
			// We went through the whole fake trace and did not find valid[vi].
			// Go back to the last fake field we found and try finding valid[vi+1]
			d.AddDifference("validation: %v.%v:%v\nfake agent: %v.%v NOT PRESENT\n", prefix, vkv.k, vkv.v, prefix, vkv.k)
			fi = lastfi
			vi += 1
		}
	}
}

// A kv is an element of a trace, consisting of a key such as the name, service, etc. and
// the value corresponding to that key.
// A typical trace will be represented as a slice of kv.
// Nested structures such as "meta" and "metrics" should have values that are themselves
// slices of kv. i.e. kv{k: "meta", v: []kv{...}}
//
// The nested "_children" field is a slice of slice of kv. Each child "span" is a slice of kv, and
// we have zero or more of those in a slice, which gives us [][]kv
//
// The reason this is necessary is to create an ordered set of key/value pairs so that we
// can compare two traces in an element-by-element "diff" style.
type kv struct {
	k string
	v interface{}
}

// getKey returns the value for some key in a slice of kv.
func getKey(k string, kvs []kv) interface{} {
	for _, kv := range kvs {
		if kv.k == k {
			return kv.v
		}
	}
	return nil
}

// alphaKey implements the sort interface to sort a slice of kv by
// alphabetical order on the keys.
type alphaKey []kv

func (k alphaKey) Len() int {
	return len(k)
}
func (k alphaKey) Less(i, j int) bool {
	return k[i].k < k[j].k
}

func (k alphaKey) Swap(i, j int) {
	k[i], k[j] = k[j], k[i]
}

// byResource sorts a slice of slice of kv (a slice of spans) by the value of the "resource"
// key in each span. This is useful for sorting a slice of children.
type byResource [][]kv

func (k byResource) Len() int {
	return len(k)
}
func (k byResource) Less(i, j int) bool {
	k1 := getKey("resource", k[i])
	k2 := getKey("resource", k[j])
	if k1 == nil {
		return true
	}
	if k2 == nil {
		return false
	}
	return k1.(string) < k2.(string)
}

func (k byResource) Swap(i, j int) {
	k[i], k[j] = k[j], k[i]
}

// genKVs turns a trace in the form described by validation.json into a
// trace in kv form (see type kv).
func genKVs(validation map[string]interface{}) []kv {
	if validation["_children"] != nil {
		var children [][]kv
		for _, child := range validation["_children"].([]interface{}) {
			mc := child.(map[string]interface{})
			children = append(children, genKVs(mc))
		}
		sort.Sort(byResource(children))
		validation["_children"] = children

	}
	if validation["meta"] != nil {
		m := validation["meta"].(map[string]interface{})
		var sorted []kv
		for k, v := range m {
			sorted = append(sorted, kv{k: k, v: v})
		}
		sort.Sort(alphaKey(sorted))
		validation["meta"] = sorted
	}

	if validation["metrics"] != nil {
		m := validation["metrics"].(map[string]interface{})
		var sorted []kv
		for k, v := range m {
			sorted = append(sorted, kv{k: k, v: v})
		}
		sort.Sort(alphaKey(sorted))
		validation["metrics"] = sorted
	}

	var sorted []kv
	for k, v := range validation {
		sorted = append(sorted, kv{k: k, v: v})
	}
	sort.Sort(alphaKey(sorted))
	return sorted
}

// DumpKV prints out a trace in kv format, meant for human consumption.
func DumpKV(o io.Writer, kvs []kv) {
	fmt.Fprintf(o, "START TRACE ##########\n")
	dumpKVRec(o, kvs, 1)
	fmt.Fprintf(o, "END TRACE ##########\n")
}

func printIndent(o io.Writer, indent int) {
	for i := 0; i < indent; i++ {
		fmt.Fprint(o, "\t")
	}
}

func dumpKVRec(o io.Writer, kvs []kv, indent int) {
	var children interface{}
	for _, gkv := range kvs {
		if gkv.k == "_children" {
			children = gkv.v
			continue
		}
		printIndent(o, indent)
		if gkv.k == "meta" {
			fmt.Fprintln(o, "meta:")
			v := gkv.v.([]kv)
			dumpKVRec(o, v, indent+1)
		} else if gkv.k == "metrics" {
			fmt.Fprintln(o, "metrics:")
			v := gkv.v.([]kv)
			dumpKVRec(o, v, indent+1)
		} else {
			fmt.Fprintf(o, "%s: %s\n", gkv.k, gkv.v)
		}
	}
	if children != nil {
		printIndent(o, indent)
		fmt.Fprintln(o, "children:")
		cs := children.([][]kv)
		for i, c := range cs {
			printIndent(o, indent+1)
			fmt.Fprintf(o, "[%d]:\n", i)
			dumpKVRec(o, c, indent+2)
		}
	}
}
