package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func loadEnvFile(filename string) (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if equal := strings.Index(line, "="); equal >= 0 {
			key := line[:equal]
			value := line[equal+1:]
			if strings.HasPrefix(value, "\"") {
				value, err = strconv.Unquote(value)
				if err != nil {
					return nil, fmt.Errorf("failed to unquote: %w", err)
				}
			}

			env[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return env, nil
}
