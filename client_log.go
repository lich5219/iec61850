package iec61850

/*
#include <iec61850_client.h>

static void destroy_journal_entries(LinkedList entries) {
	LinkedList_destroyDeep(entries, (LinkedListValueDeleteFunction) MmsJournalEntry_destroy);
}
*/
import "C"

import "unsafe"

type ClientLogEntry struct {
	EntryID       interface{}
	TimeOfEntry   interface{}
	EntryDataList []ClientLogEntryData
}

type ClientLogEntryData struct {
	DataRef    string
	Value      interface{}
	ReasonCode interface{}
}

func (c *Client) QueryLogByTime(logReference string, startTime int64, endTime int64) ([]ClientLogEntry, error) {
	cLogReference := C.CString(logReference)
	defer C.free(unsafe.Pointer(cLogReference))

	var clientError C.IedClientError
	var moreFollows C.bool
	entries := C.IedConnection_queryLogByTime(c.conn, &clientError, cLogReference, C.uint64_t(startTime), C.uint64_t(endTime), &moreFollows)
	if err := GetIedClientError(clientError); err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, nil
	}
	defer C.destroy_journal_entries(entries)

	result := make([]ClientLogEntry, 0)
	for item := entries.next; item != nil; item = item.next {
		entry := C.MmsJournalEntry(item.data)
		if entry == nil {
			continue
		}
		goEntry := ClientLogEntry{}
		if entryID := C.MmsJournalEntry_getEntryID(entry); entryID != nil {
			value, err := toGoValue(entryID, MmsType(C.MmsValue_getType(entryID)))
			if err != nil {
				return nil, err
			}
			goEntry.EntryID = value
		}
		if occurrenceTime := C.MmsJournalEntry_getOccurenceTime(entry); occurrenceTime != nil {
			value, err := toGoValue(occurrenceTime, MmsType(C.MmsValue_getType(occurrenceTime)))
			if err != nil {
				return nil, err
			}
			goEntry.TimeOfEntry = value
		}
		variables := C.MmsJournalEntry_getJournalVariables(entry)
		for variableItem := variables.next; variableItem != nil; variableItem = variableItem.next {
			variable := C.MmsJournalVariable(variableItem.data)
			if variable == nil {
				continue
			}
			data := ClientLogEntryData{DataRef: C.GoString(C.MmsJournalVariable_getTag(variable))}
			if value := C.MmsJournalVariable_getValue(variable); value != nil {
				goValue, err := toGoValue(value, MmsType(C.MmsValue_getType(value)))
				if err != nil {
					return nil, err
				}
				data.Value = goValue
			}
			goEntry.EntryDataList = append(goEntry.EntryDataList, data)
		}
		result = append(result, goEntry)
	}
	return result, nil
}
