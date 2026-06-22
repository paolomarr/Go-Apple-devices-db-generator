package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dlclark/regexp2"
)

type BuildNumber struct {
	Major int // two digits
	Minor string // one uppercase letter
	Build int // variable number of digits
	Patch string // optional, one lowercase letter
}

// BuildNumber string
func (b BuildNumber) String() string {
	return fmt.Sprintf("%02d%s%d%s", b.Major, b.Minor, b.Build, b.Patch)
}
type IOSVersion struct {
	Version OSVersion
	Builds []BuildNumber
}

func BuildNumberFromString(buildNumber string) (BuildNumber, error) {
	bnregex, err := regexp2.Compile(`(?<major>[0-9]{2})(?<minor>[A-Z])(?<build>[0-9]+)(?<patch>[a-z])?`, regexp2.None)
	if err != nil {
		return BuildNumber{}, err
	}
	match, err := bnregex.FindStringMatch(buildNumber)
	if err != nil {
		return BuildNumber{}, err
	}
	if match == nil {
		return BuildNumber{}, errors.New("No match")
	}
	major, _ := strconv.Atoi(match.GroupByName("major").Capture.String())
	build, _ := strconv.Atoi(match.GroupByName("build").Capture.String())
	return BuildNumber{
		Major: major,
		Minor: match.GroupByName("minor").Capture.String(),
		Build: build,
		Patch: match.GroupByName("patch").Capture.String(),
	}, nil
}

// IOSVersion String
func (i IOSVersion) String() string {
	buildStrings := []string{}
	for _, b := range i.Builds {
		buildStrings = append(buildStrings, b.String())
	}
	return fmt.Sprintf("%s (%s)", i.Version.String(), strings.Join(buildStrings, ", "))
}