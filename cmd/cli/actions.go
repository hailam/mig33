package main

import "fmt"

type action struct {
	moderator, room, status, note string
	revoke, reports               bool
	reportID, reviewID            int64
	limit                         int
}

func (action action) selected() int {
	count := 0
	for _, selected := range []bool{action.moderator != "", action.reports, action.reportID != 0, action.reviewID != 0} {
		if selected {
			count++
		}
	}
	return count
}

func (action action) validate() error {
	if action.selected() > 1 {
		return fmt.Errorf("choose only one operator action")
	}
	if action.selected() != 1 {
		return fmt.Errorf("CLI requires -reports, -report-id, -review-report or -room-moderator")
	}
	if action.revoke && action.moderator == "" {
		return fmt.Errorf("-revoke requires -room-moderator")
	}
	if action.reportID < 0 || action.reviewID < 0 {
		return fmt.Errorf("report IDs must be positive")
	}
	if action.limit < 1 || action.limit > 1000 {
		return fmt.Errorf("-report-limit must be between 1 and 1000")
	}
	return nil
}
