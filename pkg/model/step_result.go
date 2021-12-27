package model

import "fmt"

type stepStatus int

const (
	StepStatusSuccess stepStatus = iota
	StepStatusFailure
)

var stepStatusStrings = [...]string{
	"success",
	"failure",
}

func (s stepStatus) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *stepStatus) UnmarshalText(b []byte) error {
	str := string(b)
	for i, name := range stepStatusStrings {
		if name == str {
			*s = stepStatus(i)
			return nil
		}
	}
	return fmt.Errorf("invalid step status %q", str)
}

func (s stepStatus) String() string {
	if int(s) >= len(stepStatusStrings) {
		return ""
	}
	return stepStatusStrings[s]
}

type StepResult struct {
	Outputs    map[string]string `json:"outputs"`
	Conclusion stepStatus        `json:"conclusion"`
	Outcome    stepStatus        `json:"outcome"`
}
