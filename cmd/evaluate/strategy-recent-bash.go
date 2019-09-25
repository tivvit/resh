package main

import "github.com/curusarn/resh/pkg/records"

type strategyRecentBash struct {
	histfile         []string
	histfileSnapshot map[string][]string
	history          map[string][]string
}

func (s *strategyRecentBash) init() {
	s.histfileSnapshot = map[string][]string{}
	s.history = map[string][]string{}
}

func (s *strategyRecentBash) GetTitleAndDescription() (string, string) {
	return "recent (bash-like)", "Behave like bash"
}

func (s *strategyRecentBash) GetCandidates(strippedRecord records.EnrichedRecord) []string {
	// populate the local history from histfile
	if s.histfileSnapshot[strippedRecord.SessionID] == nil {
		s.histfileSnapshot[strippedRecord.SessionID] = s.histfile
	}
	return append(s.history[strippedRecord.SessionID], s.histfileSnapshot[strippedRecord.SessionID]...)
}

func (s *strategyRecentBash) AddHistoryRecord(record *records.EnrichedRecord) error {
	// remove previous occurance of record
	for i, cmd := range s.history[record.SessionID] {
		if cmd == record.CmdLine {
			s.history[record.SessionID] = append(s.history[record.SessionID][:i], s.history[record.SessionID][i+1:]...)
		}
	}
	// append new record
	s.history[record.SessionID] = append([]string{record.CmdLine}, s.history[record.SessionID]...)

	if record.LastRecordOfSession {
		// append history of the session to histfile and clear session history
		s.histfile = append(s.history[record.SessionID], s.histfile...)
		s.histfileSnapshot[record.SessionID] = nil
		s.history[record.SessionID] = nil
	}
	return nil
}

func (s *strategyRecentBash) ResetHistory() error {
	s.init()
	return nil
}
