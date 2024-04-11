package version

import (
	"testing"
)

func TestOsVersionFromString(t *testing.T) {
	var osver OSVersion
	// good strings
	var err error
	var testString1 string = "12"
	osver, err = OSVersionFromString(testString1)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 12 || osver.Y != 0 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString1, osver.String())
	}
	var testString2 string = "12.0"
	osver, err = OSVersionFromString(testString2)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 12 || osver.Y != 0 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString2, osver.String())
	}
	var testString3 string = "12.1"
	osver, err = OSVersionFromString(testString3)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 12 || osver.Y != 1 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString3, osver.String())
	}
	var testString4 string = "12.10"
	osver, err = OSVersionFromString(testString4)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 12 || osver.Y != 10 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString4, osver.String())
	}
	var testString5 string = "12.0.0"
	osver, err = OSVersionFromString(testString5)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 12 || osver.Y != 0 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString5, osver.String())
	}
	var testString6 string = "13.0.0"
	osver, err = OSVersionFromString(testString6)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 13 || osver.Y != 0 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString6, osver.String())
	}
	var testString7 string = "13.14.0"
	osver, err = OSVersionFromString(testString7)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 13 || osver.Y != 14 || osver.Z != 0 {
		t.Fatalf("Expected: %s, got: %s", testString7, osver.String())
	}
	var testString8 string = "13.14.21"
	osver, err = OSVersionFromString(testString8)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 13 || osver.Y != 14 || osver.Z != 21 {
		t.Fatalf("Expected: %s, got: %s", testString8, osver.String())
	}
	var testString9 string = "13.14.123"
	osver, err = OSVersionFromString(testString9)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if osver.X != 13 || osver.Y != 14 || osver.Z != 123 {
		t.Fatalf("Expected: %s, got: %s", testString9, osver.String())
	}

	// bad strings
	badstrings := []string{
		"12.a.0",
		"",
		"Q",
		":",
		"1.3,4",
		".3",
		"10,3",
	}
	for _, bad := range badstrings {
		osver, err = OSVersionFromString(bad)
		if err == nil {
			t.Fatalf("%s should produce an error", bad)
		}
	}
}
func TestCompareFunctions(t *testing.T) {
	ver0, _ := OSVersionFromString("1.0.0")
	ver1, _ := OSVersionFromString("1.0.1")
	ver1_1, _ := OSVersionFromString("1.1.1")
	ver0_bis, _ := OSVersionFromString("1.0.0")
	ver2, _ := OSVersionFromString("2.0.0")
	if !ver0.Lt(ver1) {
		t.Fatalf("%s should be less than %s", ver0.String(), ver1.String())
	}
	if !ver1.Gt(ver0) {
		t.Fatalf("%s should be greater than %s", ver1.String(), ver0.String())
	}
	if !ver0.Eq(ver0_bis) {
		t.Fatalf("%s should equal %s", ver0.String(), ver0_bis.String())
	}
	if !ver0.Lte(ver0_bis) {
		t.Fatalf("%s should be less than or equal to %s", ver0.String(), ver0_bis.String())
	}
	if !ver2.Gt(ver1_1) || !ver2.Gte(ver1_1) {
		t.Fatalf("%s should be greater than or equal to %s", ver2.String(), ver1_1.String())
	}
}
func TestRanges(t *testing.T) {
	ver0, _ := OSVersionFromString("1.0.0")
	ver2, _ := OSVersionFromString("2.0.0")
	rng := OSVersionRange{
		Left:  OSVersionRangeLimit{V: ver0, Inclusive: true},
		Right: OSVersionRangeLimit{V: ver2, Inclusive: false},
	}
	t.Logf("Testing range %s", rng.String())
	var testVer OSVersion
	testVer, _ = OSVersionFromString("1.0")
	if !testVer.InRange(rng) {
		t.Fatalf("%s should be in range %s", testVer.String(), rng.String())
	}
	// make left limit become NON-inclusive
	rng.Left.Inclusive = false
	t.Logf("Testing range %s", rng.String())
	if testVer.InRange(rng) {
		t.Fatalf("%s should NOT be in range %s", testVer.String(), rng.String())
	}
	testVer, _ = OSVersionFromString("1.3.1")
	if !testVer.InRange(rng) {
		t.Fatalf("%s should be in range %s", testVer.String(), rng.String())
	}
	testVer, _ = OSVersionFromString("3.4.32")
	if testVer.InRange(rng) {
		t.Fatalf("%s should NOT be in range %s", testVer.String(), rng.String())
	}
	testVer, _ = OSVersionFromString("2.0.0")
	if testVer.InRange(rng) {
		t.Fatalf("%s should NOT be in range %s", testVer.String(), rng.String())
	}
	// make ritght limit become inclusive
	rng.Right.Inclusive = true
	t.Logf("Testing range %s", rng.String())
	if !testVer.InRange(rng) {
		t.Fatalf("%s should be in range %s", testVer.String(), rng.String())
	}
	testVer, _ = OSVersionFromString("0.100")
	if testVer.InRange(rng) {
		t.Fatalf("%s should NOT be in range %s", testVer.String(), rng.String())
	}
}
