package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"

	e "github.com/Amund211/flashlight/internal/errors"

	"github.com/stretchr/testify/assert"
)

type processPlayerDataTest struct {
	name               string
	before             []byte
	hypixelStatusCode  int
	after              []byte
	expectedStatusCode int
	error              error
}

const processFixtureDir = "fixtures/"

// NOTE: for readability, after is compacted before being compared
var literalTests = []processPlayerDataTest{
	{name: "empty object", before: []byte(`{}`), after: []byte(`{"success":false,"player":null}`)},
	{name: "empty list", before: []byte(`[]`), after: []byte{}, error: e.APIServerError},
	{name: "empty string", before: []byte(``), after: []byte{}, error: e.APIServerError},
	{
		name: "float experience",
		before: []byte(`{
			"success": true,
			"player": {
				"stats": {
					"Bedwars": {
						"Experience": 1087.0
					}
				}
			}
		}`),
		after: []byte(`{
			"success": true,
			"player": {
				"stats": {
					"Bedwars": {
						"Experience": 1087
					}
				}
			}
		}`),
	},
	{
		name: "float experience - scientific notation",
		before: []byte(`{
			"success": true,
			"player": {
				"stats": {
					"Bedwars": {
						"Experience": 1.2227806E7
					}
				}
			}
		}`),
		after: []byte(`{
			"success": true,
			"player": {
				"stats": {
					"Bedwars": {
						"Experience": 12227806
					}
				}
			}
		}`),
	},
	{
		name:               "not found",
		before:             []byte(`{"success": true, "player": null}`),
		after:              []byte(`{"success": true, "player": null}`),
		expectedStatusCode: 404,
	},
	{
		name:              "hypixel 500",
		before:            []byte(`{"success":false,"cause":"Internal error"}`),
		hypixelStatusCode: 500,
		error:             e.RetriableError,
	},
	// The "hypixel weird" cases are just made up to test status code handling
	{
		name:              "hypixel weird 100",
		before:            []byte(`{"success": true, "player": null}`),
		hypixelStatusCode: 100,
		error:             e.APIServerError,
	},
	{
		name:              "hypixel weird 204",
		before:            []byte(`{"success": true, "player": null}`),
		hypixelStatusCode: 204,
		error:             e.APIServerError,
	},
	{
		name:              "hypixel weird 301",
		before:            []byte(`{"success": true, "player": null}`),
		hypixelStatusCode: 301,
		error:             e.APIServerError,
	},
	{
		name:              "hypixel weird 418",
		before:            []byte(`{"success": true, "player": null}`),
		hypixelStatusCode: 418,
		error:             e.APIServerError,
	},
	{
		name:              "hypixel weird 508",
		before:            []byte(`{"success": true, "player": null}`),
		hypixelStatusCode: 508,
		error:             e.APIServerError,
	},
}

func parsePlayerDataFile(filePath string) (processPlayerDataTest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return processPlayerDataTest{}, err
	}
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) != 2 {
		return processPlayerDataTest{}, fmt.Errorf("File %s does not contain 2 lines", filePath)
	}
	return processPlayerDataTest{name: fmt.Sprintf("<%s>", filePath), before: lines[0], after: lines[1]}, nil
}

func runProcessPlayerDataTest(t *testing.T, test processPlayerDataTest) {
	hypixelStatusCode := 200
	if test.hypixelStatusCode != 0 {
		hypixelStatusCode = test.hypixelStatusCode
	}
	expectedStatusCode := 200
	if test.expectedStatusCode != 0 {
		expectedStatusCode = test.expectedStatusCode
	}
	parsedResponse, statusCode, err := ParseHypixelAPIResponse(context.Background(), test.before, hypixelStatusCode)

	if test.error != nil {
		assert.Equal(t, 0, test.expectedStatusCode, "status code not returned on error")
		assert.ErrorIs(t, err, test.error, "processPlayerData(%s) - expected error", test.name)
		return
	}
	assert.Nil(t, err, "processPlayerData(%s) - unexpected parse error: %v", test.name, err)

	minified, err := MarshalPlayerData(context.Background(), parsedResponse)

	assert.Nil(t, err, "processPlayerData(%s) - unexpected marshall error: %v", test.name, err)
	assert.Equal(t, expectedStatusCode, statusCode, test.name)
	assert.Equal(t, string(test.after), string(minified), "processPlayerData(%s) - expected '%s', got '%s'", test.name, test.after, minified)
}

func TestProcessPlayerDataLiterals(t *testing.T) {
	cloudfareTests := []processPlayerDataTest{}
	for _, statusCode := range []int{502, 503, 504, 520, 521, 522, 523, 524, 525, 526, 527, 530} {
		cloudfareTests = append(cloudfareTests, processPlayerDataTest{
			name:              fmt.Sprintf("cloudflare %d", statusCode),
			before:            []byte(fmt.Sprintf("error code: %d", statusCode)),
			hypixelStatusCode: statusCode,
			error:             e.RetriableError,
		})
	}

	for _, test := range append(literalTests, cloudfareTests...) {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if test.error == nil {
				var compacted bytes.Buffer
				err := json.Compact(&compacted, test.after)
				assert.Nil(t, err, "processPlayerData(%s): Error compacting JSON: %v", test.name, err)
				test.after = compacted.Bytes()
			}

			// Real test
			runProcessPlayerDataTest(t, test)

			if test.error == nil {
				// Test that minification is idempotent
				test.before = test.after
				test.name = test.name + " (minified)"
				runProcessPlayerDataTest(t, test)
			}
		})
	}
}

func TestProcessPlayerDataFiles(t *testing.T) {
	files, err := os.ReadDir(processFixtureDir)

	assert.Nil(t, err, "Error reading fixtures directory: %v", err)

	wg := sync.WaitGroup{}
	wg.Add(len(files))

	for _, file := range files {
		file := file
		if file.IsDir() {
			continue
		}
		go func() {
			filePath := path.Join(processFixtureDir, file.Name())
			test, err := parsePlayerDataFile(filePath)
			assert.Nil(t, err, "Error parsing file %s: %v", filePath, err)
			// Real test
			runProcessPlayerDataTest(t, test)

			// Test that minification is idempotent
			test.before = test.after
			test.name = test.name + " (minified)"
			runProcessPlayerDataTest(t, test)
			wg.Done()
		}()
	}

	wg.Wait()
}
