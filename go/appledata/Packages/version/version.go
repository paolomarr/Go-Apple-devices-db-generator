package version

import (
	"fmt"
	"strconv"
	"strings"
)

type OSVersion struct {
	X int
	Y int
	Z int
}
type OSVersionRangeLimit struct {
	V         OSVersion
	Inclusive bool
}
type OSVersionRange struct {
	Left  OSVersionRangeLimit
	Right OSVersionRangeLimit
}

func (o OSVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", o.X, o.Y, o.Z)
}
func (o OSVersion) Eq(rhs OSVersion) bool {
	return o.X == rhs.X && o.Y == rhs.Y && o.Z == rhs.Z
}
func (o OSVersion) Lt(rhs OSVersion) bool {
	return o.X < rhs.X || (o.X == rhs.X && o.Y < rhs.Y) || (o.X == rhs.X && o.Y == rhs.Y && o.Z < rhs.Z)
}
func (o OSVersion) Lte(rhs OSVersion) bool {
	return o.Lt(rhs) || o.Eq(rhs)
}
func (o OSVersion) Gt(rhs OSVersion) bool {
	return !o.Lte(rhs)
}
func (o OSVersion) Gte(rhs OSVersion) bool {
	return !o.Lt(rhs)
}
func (o OSVersion) InRange(rng OSVersionRange) bool {
	return (o.Gt(rng.Left.V) || (rng.Left.Inclusive && o.Eq(rng.Left.V))) &&
		(o.Lt(rng.Right.V) || (rng.Right.Inclusive && o.Eq(rng.Right.V)))
}
func (ovrl OSVersionRange) String() string {
	var leftbkt, rightbkt string = "(", ")"
	if ovrl.Left.Inclusive {
		leftbkt = "["
	}
	if ovrl.Right.Inclusive {
		rightbkt = "]"
	}
	return fmt.Sprintf("%s%s,%s%s", leftbkt, ovrl.Left.V.String(), ovrl.Right.V.String(), rightbkt)
}

func OSVersionFromString(verstring string) (OSVersion, error) {
	var outver OSVersion = OSVersion{X: 0, Y: 0, Z: 0}
	var parts []string = strings.Split(verstring, ".")
	var err error
	outver.X, err = strconv.Atoi(parts[0])
	if err != nil {
		return outver, err
	}
	if len(parts) > 1 {
		outver.Y, err = strconv.Atoi(parts[1])
		if err != nil {
			return outver, err
		}
	}
	if len(parts) > 2 {
		outver.Z, err = strconv.Atoi(parts[2])
		if err != nil {
			return outver, err
		}
	}
	return outver, nil
}
