package parsing

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"testing"
)

type minifyPlayerDataTest struct {
	name   string
	before []byte
	after  []byte
	error  bool
}

const minifyFixtureDir = "fixtures/"

var literalTests = []minifyPlayerDataTest{
	{name: "empty object", before: []byte(`{}`), after: []byte(`{"success":false,"player":null}`)},
	{name: "empty list", before: []byte(`[]`), after: []byte{}, error: true},
	{name: "empty string", before: []byte(``), after: []byte{}, error: true},
}

func parsePlayerDataFile(filePath string) (minifyPlayerDataTest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return minifyPlayerDataTest{}, err
	}
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) != 2 {
		return minifyPlayerDataTest{}, fmt.Errorf("File %s does not contain 2 lines", filePath)
	}
	return minifyPlayerDataTest{name: fmt.Sprintf("<%s>", filePath), before: lines[0], after: lines[1]}, nil
}

func runMinifyPlayerDataTest(t *testing.T, test minifyPlayerDataTest) {
	minified, err := MinifyPlayerData(test.before)

	if test.error {
		if err == nil {
			t.Errorf("minifyPlayerData(%s) - expected error", test.name)
		}
		return
	}

	if err != nil {
		t.Errorf("minifyPlayerData(%s) - %s", test.name, err.Error())
	}
	if string(minified) != string(test.after) {
		t.Errorf("minifyPlayerData(%s) = '%s' != '%s'", test.name, minified, test.after)
	}
}

func TestMinifyPlayerDataLiterals(t *testing.T) {
	for _, test := range literalTests {
		runMinifyPlayerDataTest(t, test)
	}
}

func TestMinifyPlayerDataFiles(t *testing.T) {
	files, err := os.ReadDir(minifyFixtureDir)
	if err != nil {
		t.Errorf("Error reading fixtures directory: %s", err.Error())
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := path.Join(minifyFixtureDir, file.Name())
		test, err := parsePlayerDataFile(filePath)
		if err != nil {
			t.Errorf("Error parsing file %s: %s", filePath, err.Error())
			continue
		}
		runMinifyPlayerDataTest(t, test)
	}
}