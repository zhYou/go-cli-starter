package contract

import (
	"encoding/json"
	"fmt"
	"golang.org/x/mod/semver"
	"reflect"
	"slices"
	"sort"
	"strings"
)

type Change struct {
	Path   string          `json:"path"`
	Kind   string          `json:"kind"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}
type Record struct {
	ID              string   `json:"id"`
	Compatibility   string   `json:"compatibility"`
	Summary         string   `json:"summary"`
	Paths           []string `json:"paths"`
	PreviousVersion string   `json:"previous_version"`
	TargetVersion   string   `json:"target_version"`
	Migration       string   `json:"migration"`
}

func Normalize(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return json.MarshalIndent(normalize(value), "", "  ")
}
func normalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			if k == "description" || k == "examples" || k == "use" {
				continue
			}
			out[k] = normalize(item)
		}
		return out
	case []any:
		if len(v) > 0 {
			if first, ok := v[0].(map[string]any); ok {
				key := ""
				if _, ok := first["path"]; ok {
					key = "path"
				} else if _, ok := first["name"]; ok {
					key = "name"
				}
				if key != "" {
					out := map[string]any{}
					for _, item := range v {
						object := item.(map[string]any)
						out[object[key].(string)] = normalize(item)
					}
					return out
				}
			}
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalize(item)
		}
		return out
	}
	return value
}
func Diff(before, after []byte) ([]Change, error) {
	var a, b any
	if err := json.Unmarshal(before, &a); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(after, &b); err != nil {
		return nil, err
	}
	changes := []Change{}
	var visit func(string, any, any, bool, bool)
	visit = func(path string, old, new any, oldOK, newOK bool) {
		if oldOK && newOK && reflect.DeepEqual(old, new) {
			return
		}
		am, aMap := old.(map[string]any)
		bm, bMap := new.(map[string]any)
		if oldOK && newOK && aMap && bMap {
			keys := []string{}
			for k := range am {
				keys = append(keys, k)
			}
			for k := range bm {
				if _, ok := am[k]; !ok {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			for _, k := range keys {
				av, ao := am[k]
				bv, bo := bm[k]
				visit(path+"/"+strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1"), av, bv, ao, bo)
			}
			return
		}
		kind := "changed"
		if !oldOK {
			kind = "added"
		} else if !newOK || strings.HasSuffix(path, "/type") || strings.HasSuffix(path, "/types") || strings.Contains(path, "/results/") || strings.HasSuffix(path, "/required") {
			kind = "breaking"
		}
		if !oldOK {
			if strings.HasSuffix(path, "/required") {
				kind = "breaking"
			}
			if object, ok := new.(map[string]any); ok && object["required"] == true {
				kind = "breaking"
			}
		}
		prev, _ := json.Marshal(old)
		next, _ := json.Marshal(new)
		changes = append(changes, Change{path, kind, prev, next})
	}
	visit("", a, b, true, true)
	return changes, nil
}
func ValidateRecords(changes []Change, records []Record, initial bool) error {
	if initial {
		found := false
		for _, r := range records {
			if r.PreviousVersion == "" && r.ID != "" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("首次建立机器契约须提供 previous_version 为 null 的初始化记录")
		}
	}
	for _, change := range changes {
		covered := false
		for _, r := range records {
			if r.ID == "" || r.Summary == "" || r.Migration == "" || !semver.IsValid(r.TargetVersion) {
				return fmt.Errorf("兼容变更记录字段不完整：%s", r.ID)
			}
			if r.Compatibility != "compatible" && r.Compatibility != "breaking" && r.Compatibility != "review-required" {
				return fmt.Errorf("兼容性声明无效：%s", r.ID)
			}
			if r.PreviousVersion != "" && (!semver.IsValid(r.PreviousVersion) || semver.Compare(r.TargetVersion, r.PreviousVersion) <= 0) {
				return fmt.Errorf("目标版本必须高于有效的旧版本：%s", r.ID)
			}
			matches := slices.ContainsFunc(r.Paths, func(path string) bool {
				return path == change.Path || strings.HasPrefix(change.Path, strings.TrimSuffix(path, "/")+"/")
			})
			if !matches {
				continue
			}
			if change.Kind == "breaking" {
				if r.Compatibility != "breaking" {
					return fmt.Errorf("潜在破坏变更 %s 不能声明为兼容", change.Path)
				}
				if r.PreviousVersion == "" {
					return fmt.Errorf("破坏变更须指定旧版本：%s", r.ID)
				}
				oldMajor := semver.Major(r.PreviousVersion)
				if oldMajor == "v0" {
					if semver.MajorMinor(r.TargetVersion) == semver.MajorMinor(r.PreviousVersion) {
						return fmt.Errorf("0.x 破坏变更须提升次版本")
					}
				} else if semver.Major(r.TargetVersion) == oldMajor {
					return fmt.Errorf("破坏变更须提升主版本")
				}
			}
			if change.Kind == "changed" && r.Compatibility == "compatible" {
				return fmt.Errorf("复杂变化 %s 须声明 review-required 或 breaking", change.Path)
			}
			covered = true
		}
		if !covered {
			return fmt.Errorf("机器契约变化缺少新变更记录：%s（%s）", change.Path, change.Kind)
		}
	}
	return nil
}
