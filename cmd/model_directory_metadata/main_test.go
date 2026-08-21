package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCommandOptionsRequiresExplicitSQLDSN(t *testing.T) {
	_, err := parseCommandOptions([]string{"--file", "metadata.json", "--dry-run"}, func(string) string { return "" })
	require.ErrorContains(t, err, "SQL_DSN")
}

func TestParseCommandOptionsRequiresExactlyOneMode(t *testing.T) {
	lookupEnv := func(key string) string {
		if key == "SQL_DSN" {
			return "local"
		}
		return ""
	}

	_, err := parseCommandOptions([]string{"--file", "metadata.json"}, lookupEnv)
	require.ErrorContains(t, err, "exactly one")
	_, err = parseCommandOptions([]string{"--file", "metadata.json", "--dry-run", "--apply"}, lookupEnv)
	require.ErrorContains(t, err, "exactly one")
}

func TestParseCommandOptionsRequiresFile(t *testing.T) {
	_, err := parseCommandOptions([]string{"--apply"}, func(string) string { return "local" })
	require.ErrorContains(t, err, "file")
}
