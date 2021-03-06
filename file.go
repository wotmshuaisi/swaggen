package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	hashTagRegex = regexp.MustCompile("^#+ (.*)")
)

type swagFile struct {
	*bufio.Reader
	*bufio.Writer
	Result *Swag
	Seek   int
	bool   // flag for title has been read or not
}

// Methods

func newSwagFile(path, outputpath string) *swagFile {
	var input, err = os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		panic("error occured while opening")
	}
	if (fileRows(input)-1)%3 != 0 {
		panic("invalid file(lines).")
	}
	input.Seek(0, 0) // seek to file top
	// output file
	output, err := os.OpenFile(outputpath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic("error occured while opening")
	}
	var f = &swagFile{
		bufio.NewReader(input),
		bufio.NewWriter(output),
		&Swag{
			Paths:       map[string]map[string]Path{},
			Definitions: map[string]interface{}{},
		},
		0,
		false,
	}
	f.Result.Version = "2.0"
	return f
}

func (s *swagFile) ReadNext(b bool) (string, []byte, bool) {
	data, _, err := s.ReadLine()
	if err == io.EOF {
		return "", nil, true
	}
	if err != nil {
		panic(err)
	}
	// regex
	if !b {
		var d = hashTagRegex.FindAllStringSubmatch(string(data), 1)
		if len(d[0]) == 1 {
			panic("invalid format - line" + strconv.Itoa(s.Seek))
		}
		s.Seek++
		return d[0][1], nil, false
	}
	var d = hashTagRegex.FindAllSubmatch(data, 1)
	if len(d[0]) == 1 {
		panic("invalid format - line" + strconv.Itoa(s.Seek))
	}
	s.Seek++
	return "", d[0][1], false
}

func (s *swagFile) GetTitle() bool {
	if s.bool {
		return false
	}
	var data, _, eof = s.ReadNext(false)
	if eof {
		return eof
	}
	// title processing
	var title = strings.Split(data, " - ")
	s.Result.Info = map[string]string{}
	switch len(title) {
	case 3:
		s.Result.Info["description"] = title[1]
		fallthrough
	case 2:
		s.Result.Info["version"] = title[len(title)-1]
		fallthrough
	case 1:
		s.Result.Info["title"] = title[0]
	}
	s.bool = true
	return false
}

// GetPath write a path into Swag return path
func (s *swagFile) GetPath() bool {
	if s.bool == false {
		panic("title information is required")
	}
	// Method URL
	var data, _, eof = s.ReadNext(false)
	if eof {
		return eof
	}
	var d = strings.Split(data, " | ")
	var method, uri, summary = d[0], d[1], d[2]
	if s.Result.Paths[uri] == nil {
		s.Result.Paths[uri] = map[string]Path{}
	}
	// request && response
	var parameters = []*Parameter{}
	var responses = map[string]Response{}
	var _, request, _ = s.ReadNext(true)
	if len(request) != 0 {
		var req = bytes.Split(request, []byte(" | "))
		switch len(req) {
		case 3:
			// query parameters
			processParams(&parameters, req[len(req)-3], "query")
			fallthrough
		case 2:
			// path parameters
			processParams(&parameters, req[len(req)-2], "path")
			fallthrough
		case 1:
			// json
			var reqID = strings.ToUpper(method) + uriParser(uri) + "__Request"
			parameters = append(
				parameters,
				&Parameter{
					Name:   "body",
					In:     "body",
					Schema: map[string]string{"$ref": "#/definitions/" + reqID},
				},
			)
			// definition
			var reqDef = processJSON(req[len(req)-1])
			s.Result.Definitions[reqID] = reqDef
		}
	}

	var _, response, _ = s.ReadNext(true)
	var res = Response{Description: "OK"}
	if len(response) != 0 {
		var resDef = processJSON(response)
		var resID = strings.ToUpper(method) + uriParser(uri) + "__Response"
		res.Schema = map[string]string{"$ref": "#/definitions/" + resID}
		// definition
		s.Result.Definitions[resID] = resDef
	}
	responses["200"] = res

	// result
	s.Result.Paths[uri][method] = Path{
		Summary:    summary,
		Parameters: parameters,
		Responses:  responses,
	}
	return false
}

// SaveToPath save result to file
func (s *swagFile) SaveToPath(pretty bool) {
	var d []byte
	if *prettyprint {
		d, _ = json.MarshalIndent(s.Result, "", "  ")
	} else {
		d, _ = json.Marshal(s.Result)
	}
	s.Write(d)
	s.Flush()
}

// public functions

func processJSON(j []byte) *Definition {
	// return  definition
	var result = &Definition{}
	var d = map[string]interface{}{}
	var s = []interface{}{}
	var object = true
	if len(j) == 0 {
		return nil
	}
	if err := json.Unmarshal(j, &d); err != nil {
		if err := json.Unmarshal(j, &s); err != nil {
			fmt.Println(string(j))
			panic(err)
		}
		object = false
	}
	// object
	if object {
		if len(d) == 0 {
			return &Definition{}
		}
		result.Type = Object
		var properties = map[string]*Definition{}
		for k, v := range d {
			var childData, _ = json.Marshal(v)
			properties[k] = &Definition{
				Type: typeDetection(v),
			}
			switch properties[k].Type {
			case Object:
				var childMap = map[string]interface{}{}
				json.Unmarshal(childData, &childMap)
				properties[k].Properties = map[string]*Definition{}
				for kk, vv := range childMap {
					var t = typeDetection(vv)
					if t != Object {
						properties[k].Properties[kk] = &Definition{
							Type: t,
						}
						continue
					}
					var mapData, _ = json.Marshal(vv)
					properties[k].Properties[kk] = processJSON(mapData)
				}
			case Array:
				var childArray = []interface{}{}
				json.Unmarshal(childData, &childArray)
				var arrayData, _ = json.Marshal(childArray[0])
				switch typeDetection(childArray[0]) {
				case Object, Array:
					properties[k].Items = processJSON(arrayData)
				default:
					properties[k].Items = &Definition{
						Type: typeDetection(childArray[0]),
					}
				}
			}
		}
		result.Properties = properties
		return result
	}
	// array
	if len(s) == 0 {
		return &Definition{}
	}
	result.Type = "array"
	result.Items = &Definition{
		Type:       typeDetection(s[0]),
		Properties: map[string]*Definition{},
	}
	var childData, _ = json.Marshal(s[0]) // get first element
	var childMap = map[string]interface{}{}
	json.Unmarshal(childData, &childMap)
	for kk, vv := range childMap {
		var t = typeDetection(vv)
		switch t {
		case Object, Array:
			var mapData, _ = json.Marshal(vv)
			result.Items.Properties[kk] = processJSON(mapData)
		default:
			result.Items.Properties[kk] = &Definition{
				Type: t,
			}
		}
	}
	return result
}

func processParams(list *[]*Parameter, params []byte, t string) {
	for _, item := range bytes.Split(params, []byte(",")) {
		var keyValue = bytes.Split(item, []byte(":"))
		*list = append(*list, &Parameter{
			Required: true,
			Name:     string(keyValue[0]),
			Type:     string(keyValue[1]),
			In:       t,
		})
	}
}

func fileRows(r io.Reader) int {
	var rb = bufio.NewReader(r)
	var count = 0
	for {
		_, _, err := rb.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		count++
	}
	return count
}

func inDetect(items, index int) string {
	return ""
}

func uriParser(uri string) string {
	uri = strings.ReplaceAll(uri, "/", "_")
	uri = strings.ReplaceAll(uri, "{", "__")
	uri = strings.ReplaceAll(uri, "}", "__")
	return uri
}
