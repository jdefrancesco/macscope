package launchd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Job struct {
	Path              string   `json:"path,omitempty"`
	Label             string   `json:"label,omitempty"`
	Program           string   `json:"program,omitempty"`
	ProgramArguments  []string `json:"program_arguments,omitempty"`
	RunAtLoad         bool     `json:"run_at_load"`
	KeepAlive         bool     `json:"keep_alive"`
	KeepAliveDetail   string   `json:"keep_alive_detail,omitempty"`
	Disabled          bool     `json:"disabled"`
	WorkingDirectory  string   `json:"working_directory,omitempty"`
	StandardOutPath   string   `json:"standard_out_path,omitempty"`
	StandardErrorPath string   `json:"standard_error_path,omitempty"`
}

func ParsePlist(path string, data []byte) (Job, error) {
	values, err := parsePlistDict(data)
	if err != nil {
		return Job{}, err
	}

	job := Job{
		Path:              path,
		Label:             stringValue(values["Label"]),
		Program:           stringValue(values["Program"]),
		ProgramArguments:  stringSliceValue(values["ProgramArguments"]),
		RunAtLoad:         boolValue(values["RunAtLoad"]),
		Disabled:          boolValue(values["Disabled"]),
		WorkingDirectory:  stringValue(values["WorkingDirectory"]),
		StandardOutPath:   stringValue(values["StandardOutPath"]),
		StandardErrorPath: stringValue(values["StandardErrorPath"]),
	}

	switch keepAlive := values["KeepAlive"].(type) {
	case bool:
		job.KeepAlive = keepAlive
	case map[string]any:
		job.KeepAlive = len(keepAlive) > 0
		job.KeepAliveDetail = "dictionary"
	case nil:
	default:
		job.KeepAliveDetail = fmt.Sprintf("%v", keepAlive)
	}

	return job, nil
}

func parsePlistDict(data []byte) (map[string]any, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("plist dict not found")
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "dict" {
			continue
		}
		return parseDict(decoder)
	}
}

func parseDict(decoder *xml.Decoder) (map[string]any, error) {
	values := make(map[string]any)

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch tok := token.(type) {
		case xml.StartElement:
			if tok.Name.Local != "key" {
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			var key string
			if err := decoder.DecodeElement(&key, &tok); err != nil {
				return nil, err
			}
			value, err := parseNextValue(decoder)
			if err != nil {
				return nil, err
			}
			values[strings.TrimSpace(key)] = value
		case xml.EndElement:
			if tok.Name.Local == "dict" {
				return values, nil
			}
		}
	}
}

func parseNextValue(decoder *xml.Decoder) (any, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		return parseValue(decoder, start)
	}
}

func parseValue(decoder *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "string", "data", "date":
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return nil, err
		}
		return strings.TrimSpace(value), nil
	case "integer":
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return nil, err
		}
		i, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return strings.TrimSpace(value), nil
		}
		return i, nil
	case "real":
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return nil, err
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return strings.TrimSpace(value), nil
		}
		return f, nil
	case "true":
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return true, nil
	case "false":
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return false, nil
	case "array":
		return parseArray(decoder)
	case "dict":
		return parseDict(decoder)
	default:
		if err := decoder.Skip(); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

func parseArray(decoder *xml.Decoder) ([]any, error) {
	var values []any
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			value, err := parseValue(decoder, tok)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		case xml.EndElement:
			if tok.Name.Local == "array" {
				return values, nil
			}
		}
	}
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func stringSliceValue(value any) []string {
	array, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(array))
	for _, item := range array {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func boolValue(value any) bool {
	b, ok := value.(bool)
	return ok && b
}
