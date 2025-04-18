package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/Amund211/flashlight/internal/processing"
)

const minifyFixtureDir = "./internal/processing/fixtures/"

func parseMinifyFixtureFile(filePath string) ([]byte, []byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("Error reading file %s: %s", filePath, err.Error())
	}
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) < 2 {
		return nil, nil, fmt.Errorf("File %s should have at least 2 lines", filePath)
	} else if len(lines) > 2 {
		log.Printf("Warning: File %s has more than 2 lines, only the first 2 will be used", filePath)
	}
	return lines[0], lines[1], nil
}

func main() {
	files, err := os.ReadDir(minifyFixtureDir)
	if err != nil {
		log.Fatalf("Error reading fixtures directory: %s", err.Error())
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := path.Join(minifyFixtureDir, file.Name())
		playerData, oldMinified, err := parseMinifyFixtureFile(filePath)
		if err != nil {
			log.Printf("Error parsing file %s: %s", filePath, err.Error())
			continue
		}
		parsedAPIResponse, _, err := processing.ParseHypixelAPIResponse(context.Background(), playerData, 200)
		newMinified, err := processing.MarshalPlayerData(context.Background(), parsedAPIResponse)
		if err != nil {
			log.Printf("Error minifying player data: %s", err.Error())
			continue
		}

		newFixture := bytes.Join([][]byte{playerData, newMinified}, []byte("\n"))

		if !bytes.Equal(newMinified, oldMinified) {
			log.Printf("Updating fixture %s", filePath)
			os.WriteFile(filePath, newFixture, 0644)
		}
	}
}
