package run

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type validator struct{ cwd string }

func newValidator() validator {
	return validator{}
}

func newValidatorWithCWD(cwd string) (validator, error) {
	if abs, err := filepath.Abs(cwd); err != nil {
		return validator{}, err
	} else {
		return validator{abs}, nil
	}
}

func (v validator) validate(ts Tasks) error {
	var problems []string

	ids := map[string]struct{}{}
	for id, t := range ts {
		if id != t.Metadata().ID {
			problems = append(problems, fmt.Sprintf("- task '%s' has mismatched key '%s'", t.Metadata().ID, id))
		}
		ids[t.Metadata().ID] = struct{}{}
	}
	for _, t := range ts {
		for _, err := range v.validateTask(ts, t) {
			problems = append(problems, "- "+err.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(append([]string{"invalid taskfile"}, problems...), "\n"))
	}
	return nil
}

func (v validator) validateTask(ts Tasks, t Task) []error {
	var problems []error

	meta := t.Metadata()
	if meta.ID == "" {
		problems = append(problems, errors.New("Task has no ID."))
	}

	if meta.ID == "interleaved" || meta.ID == "run" {
		problems = append(problems, fmt.Errorf("'%s' is reserved and cannot be used as a task ID.", meta.ID))
	}

	for _, c := range meta.ID {
		if unicode.IsSpace(c) {
			problems = append(problems, fmt.Errorf("Task IDs cannot contain whitespace characters."))
		}
	}

	if meta.Type != "long" && meta.Type != "short" && meta.Type != "group" {
		problems = append(problems, fmt.Errorf("Task '%s' has invalid type '%s'; must be 'long', 'short', or 'group'.", meta.ID, meta.Type))
	}

	if meta.Type == "group" {
		if len(meta.Dependencies) == 0 {
			problems = append(problems, fmt.Errorf("Task '%s' is a group, but has no dependencies. Groups must include at least one dependency.", meta.ID))
		}
		if len(meta.Triggers) > 0 {
			problems = append(problems, fmt.Errorf("Task '%s' is a group, but has triggers. Groups may not have triggers.", meta.ID))
		}
		if len(meta.Watch) > 0 {
			problems = append(problems, fmt.Errorf("Task '%s' is a group, but has watch. Groups may not have watch.", meta.ID))
		}
		if s, isScript := t.(*scriptTask); isScript {
			if s.script != "" {
				problems = append(problems, fmt.Errorf("Task '%s' is a group, but has a cmd. The cmd will not be executed.", meta.ID))
			}
		}
	} else {
		if s, isScript := t.(*scriptTask); isScript {
			if s.script == "" {
				problems = append(problems, fmt.Errorf("Task '%s' is not a group, but has no cmd. It should be a group.", meta.ID))
			}
		}
	}

	for _, id := range meta.Dependencies {
		if _, ok := ts[id]; !ok {
			problems = append(problems, fmt.Errorf("Task '%s' lists dependency '%s', which is not the ID of a task.", meta.ID, id))
		}
	}

	for _, id := range meta.Triggers {
		if source, ok := ts[id]; !ok {
			problems = append(problems, fmt.Errorf("Task '%s' lists trigger '%s', which is not the ID of a task.", meta.ID, id))
		} else if source.Metadata().Type == "long" {
			problems = append(problems, fmt.Errorf("Task '%s' lists trigger '%s', which is long. Long tasks aren't expected to end, so using them as triggers is invalid.", meta.ID, id))
		}
	}

	for _, path := range meta.Watch {
		if strings.HasPrefix(path, string(os.PathSeparator)) {
			problems = append(problems, fmt.Errorf("Task '%s' wants to watch path '%s', which is absolute.", meta.ID, path))
		}
		if s, isScript := t.(*scriptTask); isScript {
			if abs, err := filepath.Abs(filepath.Join(s.dir, path)); err != nil {
				problems = append(problems, fmt.Errorf("Task '%s' had an error resolving path '%s': %s.", meta.ID, path, err))
			} else if !strings.HasPrefix(abs, v.cwd) {
				problems = append(problems, fmt.Errorf("Task '%s' wants to watch path '%s', which is outside of the working directory.", meta.ID, path))
			}
		}
	}

	return problems
}
