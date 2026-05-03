package sim

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func ReadAggTradeEvents(path string) ([]AggTradeEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]AggTradeEvent, 0)
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event AggTradeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		if event.Type != "" && event.Type != "trade" {
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Time == events[j].Time {
			return events[i].AggID < events[j].AggID
		}
		return events[i].Time < events[j].Time
	})
	return events, nil
}

func ReadOrderIntents(path string) ([]OrderIntent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	intents := make([]OrderIntent, 0)
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var intent OrderIntent
		if err := json.Unmarshal([]byte(line), &intent); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		if intent.Action == "" {
			intent.Action = "submit"
		}
		if intent.Type == "" && isSubmitIntent(intent.Action) {
			intent.Type = OrderTypeLimit
		}
		intents = append(intents, intent)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(intents, func(i, j int) bool {
		if intents[i].Time == intents[j].Time {
			leftPriority := intentPriority(intents[i].Action)
			rightPriority := intentPriority(intents[j].Action)
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			return intents[i].ClientID < intents[j].ClientID
		}
		return intents[i].Time < intents[j].Time
	})
	return intents, nil
}

func isSubmitIntent(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "submit", "place":
		return true
	default:
		return false
	}
}

func intentPriority(action string) int {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "seed_position":
		return 0
	case "submit", "place", "":
		return 1
	case "cancel":
		return 2
	default:
		return 3
	}
}
