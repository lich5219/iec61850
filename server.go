package iec61850

/*
#include <iec61850_server.h>
#include <logging_api.h>
#include <mms_value.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct sGoLogEntryData {
	char* dataRef;
	uint8_t* data;
	int dataSize;
	uint8_t reasonCode;
	struct sGoLogEntryData* next;
} GoLogEntryData;

typedef struct sGoLogEntry {
	uint64_t timestamp;
	uint64_t entryID;
	GoLogEntryData* firstData;
	GoLogEntryData* lastData;
	struct sGoLogEntry* next;
} GoLogEntry;

typedef struct sGoLogStorageData {
	uint64_t nextEntryID;
	int entryCount;
	char* filePath;
	GoLogEntry* firstEntry;
	GoLogEntry* lastEntry;
} GoLogStorageData;

static char* goLogStringDuplicate(const char* value)
{
	if (value == NULL)
		return NULL;
	size_t len = strlen(value);
	char* copy = (char*) malloc(len + 1);
	if (copy == NULL)
		return NULL;
	memcpy(copy, value, len + 1);
	return copy;
}

static void goLogAppendEntryToMemory(GoLogStorageData* storage, GoLogEntry* entry)
{
	if (storage->lastEntry != NULL)
		storage->lastEntry->next = entry;
	else
		storage->firstEntry = entry;
	storage->lastEntry = entry;
	storage->entryCount++;
	if (entry->entryID >= storage->nextEntryID)
		storage->nextEntryID = entry->entryID + 1;
}

static void goLogFreeEntry(GoLogEntry* entry)
{
	if (entry == NULL)
		return;
	GoLogEntryData* data = entry->firstData;
	while (data != NULL) {
		GoLogEntryData* nextData = data->next;
		if (data->dataRef != NULL)
			free(data->dataRef);
		if (data->data != NULL)
			free(data->data);
		free(data);
		data = nextData;
	}
	free(entry);
}

static void goLogPersist(LogStorage self)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	if (storage == NULL || storage->filePath == NULL)
		return;
	FILE* file = fopen(storage->filePath, "wb");
	if (file == NULL)
		return;
	GoLogEntry* entry = storage->firstEntry;
	while (entry != NULL) {
		char tag = 'E';
		fwrite(&tag, sizeof(char), 1, file);
		fwrite(&entry->timestamp, sizeof(uint64_t), 1, file);
		fwrite(&entry->entryID, sizeof(uint64_t), 1, file);
		GoLogEntryData* data = entry->firstData;
		while (data != NULL) {
			uint32_t refLen = data->dataRef == NULL ? 0 : (uint32_t) strlen(data->dataRef);
			uint32_t dataSize = data->dataSize < 0 ? 0 : (uint32_t) data->dataSize;
			tag = 'D';
			fwrite(&tag, sizeof(char), 1, file);
			fwrite(&entry->entryID, sizeof(uint64_t), 1, file);
			fwrite(&data->reasonCode, sizeof(uint8_t), 1, file);
			fwrite(&refLen, sizeof(uint32_t), 1, file);
			fwrite(&dataSize, sizeof(uint32_t), 1, file);
			if (refLen > 0)
				fwrite(data->dataRef, sizeof(char), refLen, file);
			if (dataSize > 0)
				fwrite(data->data, sizeof(uint8_t), dataSize, file);
			data = data->next;
		}
		entry = entry->next;
	}
	fclose(file);
}

static void goLogTrim(LogStorage self)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	while (self->maxLogEntries > 0 && storage->entryCount > self->maxLogEntries && storage->firstEntry != NULL) {
		GoLogEntry* oldEntry = storage->firstEntry;
		storage->firstEntry = oldEntry->next;
		if (storage->lastEntry == oldEntry)
			storage->lastEntry = NULL;
		storage->entryCount--;
		goLogFreeEntry(oldEntry);
	}
}

static GoLogEntry* goLogFindEntry(GoLogStorageData* storage, uint64_t entryID)
{
	GoLogEntry* entry = storage->firstEntry;
	while (entry != NULL && entry->entryID != entryID)
		entry = entry->next;
	return entry;
}

static bool goLogAppendDataToEntry(GoLogEntry* entry, const char* dataRef, uint8_t* data, int dataSize, uint8_t reasonCode)
{
	GoLogEntryData* entryData = (GoLogEntryData*) calloc(1, sizeof(GoLogEntryData));
	if (entryData == NULL)
		return false;
	if (dataRef != NULL)
		entryData->dataRef = goLogStringDuplicate(dataRef);
	if (data != NULL && dataSize > 0) {
		entryData->data = (uint8_t*) malloc(dataSize);
		if (entryData->data == NULL) {
			if (entryData->dataRef != NULL)
				free(entryData->dataRef);
			free(entryData);
			return false;
		}
		memcpy(entryData->data, data, dataSize);
		entryData->dataSize = dataSize;
	}
	entryData->reasonCode = reasonCode;
	if (entry->lastData != NULL)
		entry->lastData->next = entryData;
	else
		entry->firstData = entryData;
	entry->lastData = entryData;
	return true;
}

static void goLogLoad(LogStorage self)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	if (storage == NULL || storage->filePath == NULL)
		return;
	FILE* file = fopen(storage->filePath, "rb");
	if (file == NULL)
		return;
	while (true) {
		char tag = 0;
		if (fread(&tag, sizeof(char), 1, file) != 1)
			break;
		if (tag == 'E') {
			GoLogEntry* entry = (GoLogEntry*) calloc(1, sizeof(GoLogEntry));
			if (entry == NULL)
				break;
			if (fread(&entry->timestamp, sizeof(uint64_t), 1, file) != 1 || fread(&entry->entryID, sizeof(uint64_t), 1, file) != 1) {
				goLogFreeEntry(entry);
				break;
			}
			goLogAppendEntryToMemory(storage, entry);
		}
		else if (tag == 'D') {
			uint64_t entryID = 0;
			uint8_t reasonCode = 0;
			uint32_t refLen = 0;
			uint32_t dataSize = 0;
			if (fread(&entryID, sizeof(uint64_t), 1, file) != 1 || fread(&reasonCode, sizeof(uint8_t), 1, file) != 1 ||
					fread(&refLen, sizeof(uint32_t), 1, file) != 1 || fread(&dataSize, sizeof(uint32_t), 1, file) != 1)
				break;
			char* dataRef = NULL;
			uint8_t* data = NULL;
			if (refLen > 0) {
				dataRef = (char*) malloc(refLen + 1);
				if (dataRef == NULL)
					break;
				if (fread(dataRef, sizeof(char), refLen, file) != refLen) {
					free(dataRef);
					break;
				}
				dataRef[refLen] = 0;
			}
			if (dataSize > 0) {
				data = (uint8_t*) malloc(dataSize);
				if (data == NULL) {
					if (dataRef != NULL)
						free(dataRef);
					break;
				}
				if (fread(data, sizeof(uint8_t), dataSize, file) != dataSize) {
					if (dataRef != NULL)
						free(dataRef);
					free(data);
					break;
				}
			}
			GoLogEntry* entry = goLogFindEntry(storage, entryID);
			if (entry != NULL)
				goLogAppendDataToEntry(entry, dataRef, data, (int) dataSize, reasonCode);
			if (dataRef != NULL)
				free(dataRef);
			if (data != NULL)
				free(data);
		}
		else {
			break;
		}
	}
	fclose(file);
	goLogTrim(self);
	goLogPersist(self);
}

static uint64_t goLogAddEntry(LogStorage self, uint64_t timestamp)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	GoLogEntry* entry = (GoLogEntry*) calloc(1, sizeof(GoLogEntry));
	if (entry == NULL)
		return 0;
	entry->timestamp = timestamp;
	entry->entryID = storage->nextEntryID++;
	goLogAppendEntryToMemory(storage, entry);
	goLogTrim(self);
	goLogPersist(self);
	return entry->entryID;
}

static bool goLogAddEntryData(LogStorage self, uint64_t entryID, const char* dataRef, uint8_t* data, int dataSize, uint8_t reasonCode)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	GoLogEntry* entry = goLogFindEntry(storage, entryID);
	if (entry == NULL)
		return false;
	if (!goLogAppendDataToEntry(entry, dataRef, data, dataSize, reasonCode))
		return false;
	goLogPersist(self);
	return true;
}

static bool goLogGetEntries(LogStorage self, uint64_t startingTime, uint64_t endingTime, LogEntryCallback entryCallback, LogEntryDataCallback entryDataCallback, void* parameter)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	GoLogEntry* entry = storage->firstEntry;
	while (entry != NULL) {
		if (entry->timestamp >= startingTime && (endingTime == 0 || entry->timestamp <= endingTime)) {
			if (entryCallback != NULL && !entryCallback(parameter, entry->timestamp, entry->entryID, true))
				return false;
			GoLogEntryData* data = entry->firstData;
			while (data != NULL) {
				if (entryDataCallback != NULL && !entryDataCallback(parameter, data->dataRef, data->data, data->dataSize, data->reasonCode, true))
					return false;
				data = data->next;
			}
			if (entryDataCallback != NULL && !entryDataCallback(parameter, NULL, NULL, 0, 0, false))
				return false;
		}
		entry = entry->next;
	}
	if (entryCallback != NULL)
		entryCallback(parameter, 0, 0, false);
	return true;
}

static bool goLogGetEntriesAfter(LogStorage self, uint64_t startingTime, uint64_t entryID, LogEntryCallback entryCallback, LogEntryDataCallback entryDataCallback, void* parameter)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	GoLogEntry* entry = storage->firstEntry;
	while (entry != NULL) {
		if (entry->timestamp > startingTime || (entry->timestamp == startingTime && entry->entryID > entryID)) {
			if (entryCallback != NULL && !entryCallback(parameter, entry->timestamp, entry->entryID, true))
				return false;
			GoLogEntryData* data = entry->firstData;
			while (data != NULL) {
				if (entryDataCallback != NULL && !entryDataCallback(parameter, data->dataRef, data->data, data->dataSize, data->reasonCode, true))
					return false;
				data = data->next;
			}
			if (entryDataCallback != NULL && !entryDataCallback(parameter, NULL, NULL, 0, 0, false))
				return false;
		}
		entry = entry->next;
	}
	if (entryCallback != NULL)
		entryCallback(parameter, 0, 0, false);
	return true;
}

static bool goLogGetOldestAndNewestEntries(LogStorage self, uint64_t* newEntry, uint64_t* newEntryTime, uint64_t* oldEntry, uint64_t* oldEntryTime)
{
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	if (storage->firstEntry == NULL || storage->lastEntry == NULL)
		return false;
	if (newEntry != NULL)
		*newEntry = storage->lastEntry->entryID;
	if (newEntryTime != NULL)
		*newEntryTime = storage->lastEntry->timestamp;
	if (oldEntry != NULL)
		*oldEntry = storage->firstEntry->entryID;
	if (oldEntryTime != NULL)
		*oldEntryTime = storage->firstEntry->timestamp;
	return true;
}

static void goLogDestroy(LogStorage self)
{
	if (self == NULL)
		return;
	GoLogStorageData* storage = (GoLogStorageData*) self->instanceData;
	if (storage != NULL) {
		GoLogEntry* entry = storage->firstEntry;
		while (entry != NULL) {
			GoLogEntry* nextEntry = entry->next;
			goLogFreeEntry(entry);
			entry = nextEntry;
		}
		if (storage->filePath != NULL)
			free(storage->filePath);
		free(storage);
	}
	free(self);
}

static LogStorage goLogStorageCreate(int maxEntries, const char* filePath)
{
	LogStorage storage = (LogStorage) calloc(1, sizeof(struct sLogStorage));
	if (storage == NULL)
		return NULL;
	GoLogStorageData* data = (GoLogStorageData*) calloc(1, sizeof(GoLogStorageData));
	if (data == NULL) {
		free(storage);
		return NULL;
	}
	data->nextEntryID = 1;
	data->filePath = goLogStringDuplicate(filePath);
	storage->instanceData = data;
	storage->maxLogEntries = maxEntries;
	storage->addEntry = goLogAddEntry;
	storage->addEntryData = goLogAddEntryData;
	storage->getEntries = goLogGetEntries;
	storage->getEntriesAfter = goLogGetEntriesAfter;
	storage->getOldestAndNewestEntries = goLogGetOldestAndNewestEntries;
	storage->destroy = goLogDestroy;
	goLogLoad(storage);
	return storage;
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

type IedServer struct {
	server              C.IedServer
	serverConfig        ServerConfig
	tlsConfig           C.TLSConfiguration
	clientAuthenticator ClientAuthenticator
}

func NewServerWithTlsSupport(serverConfig ServerConfig, tlsConfig *TLSConfig, iedModel *IedModel) (*IedServer, error) {
	cTlsConfig, err := tlsConfig.createCTlsConfig()
	if err != nil {
		return nil, err
	}

	config := serverConfig.createIedServerConfig(serverConfig)
	defer C.IedServerConfig_destroy(config)
	return &IedServer{
		server:       C.IedServer_createWithConfig(iedModel.Model, cTlsConfig, config),
		serverConfig: serverConfig,
		tlsConfig:    cTlsConfig,
	}, nil
}

func NewServerWithConfig(serverConfig ServerConfig, iedModel *IedModel) *IedServer {
	config := serverConfig.createIedServerConfig(serverConfig)
	defer C.IedServerConfig_destroy(config)
	return &IedServer{
		server:       C.IedServer_createWithConfig(iedModel.Model, nil, config),
		serverConfig: serverConfig,
	}
}

// NewServer creates a new instance of the IedServer using the provided _iedModel.
func NewServer(iedModel *IedModel) *IedServer {
	return &IedServer{
		server: C.IedServer_create(iedModel.Model),
	}
}

// Start initiates the IedServer on the provided port.
func (is *IedServer) Start(port int) {
	C.IedServer_start(is.server, C.int(port))
	// If there's another way to detect the error, handle it here.
}

// IsRunning checks if the IedServer is currently running.
func (is *IedServer) IsRunning() bool {
	return bool(C.IedServer_isRunning(is.server))
}

// Stop terminates the IedServer.
func (is *IedServer) Stop() {
	C.IedServer_stop(is.server)
}

// Destroy frees all resources associated with the IedServer.
func (is *IedServer) Destroy() {
	C.IedServer_destroy(is.server)
}

// LockDataModel locks the data _iedModel of the IedServer.
func (is *IedServer) LockDataModel() {
	C.IedServer_lockDataModel(is.server)
}

// UnlockDataModel unlocks the data _iedModel of the IedServer.
func (is *IedServer) UnlockDataModel() {
	C.IedServer_unlockDataModel(is.server)
}

// UpdateUTCTimeAttributeValue updates a DataAttribute with a UTC time value.
func (is *IedServer) UpdateUTCTimeAttributeValue(node *ModelNode, value int64) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateUTCTimeAttributeValue(is.server, (*C.DataAttribute)(node._modelNode), C.uint64_t(value))
}

func (is *IedServer) SetGooseInterfaceId(interfaceId string) {
	cInterfaceId := C.CString(interfaceId)
	defer C.free(unsafe.Pointer(cInterfaceId))
	C.IedServer_setGooseInterfaceId(is.server, cInterfaceId)
}

func (is *IedServer) SetGooseInterfaceIdEx(logicalNode *ModelNode, gcbName string, interfaceId string) {
	if logicalNode == nil || logicalNode._modelNode == nil {
		return
	}
	cGCBName := C.CString(gcbName)
	cInterfaceId := C.CString(interfaceId)
	defer C.free(unsafe.Pointer(cGCBName))
	defer C.free(unsafe.Pointer(cInterfaceId))
	C.IedServer_setGooseInterfaceIdEx(is.server, (*C.LogicalNode)(logicalNode._modelNode), cGCBName, cInterfaceId)
}

func (is *IedServer) UseGooseVlanTagEx(logicalNode *ModelNode, gcbName string, useVlanTag bool) {
	if logicalNode == nil || logicalNode._modelNode == nil {
		return
	}
	cGCBName := C.CString(gcbName)
	defer C.free(unsafe.Pointer(cGCBName))
	C.IedServer_useGooseVlanTag(is.server, (*C.LogicalNode)(logicalNode._modelNode), cGCBName, C.bool(useVlanTag))
}

func (is *IedServer) SetLogStorage(logRef, storageDir string, maxEntries int) error {
	if is == nil || is.server == nil {
		return fmt.Errorf("IEC61850 server is nil")
	}
	if logRef == "" {
		return fmt.Errorf("IEC61850 log reference is empty")
	}
	if maxEntries <= 0 {
		return fmt.Errorf("IEC61850 log maxEntries must be positive")
	}
	if storageDir != "" {
		if err := os.MkdirAll(storageDir, 0o755); err != nil {
			return err
		}
	}
	fileName := strings.NewReplacer("/", "_", "\\", "_", "$", "_", ".", "_").Replace(logRef) + ".log"
	filePath := filepath.Join(storageDir, fileName)
	cLogRef := C.CString(logRef)
	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cLogRef))
	defer C.free(unsafe.Pointer(cFilePath))
	storage := C.goLogStorageCreate(C.int(maxEntries), cFilePath)
	if storage == nil {
		return fmt.Errorf("create IEC61850 log storage failed")
	}
	C.IedServer_setLogStorage(is.server, cLogRef, storage)
	return nil
}

func (is *IedServer) EnableGoosePublishing() {
	C.IedServer_enableGoosePublishing(is.server)
}

func (is *IedServer) UpdateBooleanAttributeValue(node *ModelNode, value bool) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateBooleanAttributeValue(is.server, (*C.DataAttribute)(node._modelNode), C.bool(value))
}

// UpdateFloatAttributeValue updates a DataAttribute with a float value.
func (is *IedServer) UpdateFloatAttributeValue(node *ModelNode, value float32) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateFloatAttributeValue(is.server, (*C.DataAttribute)(node._modelNode), C.float(value))
}

func (is *IedServer) UpdateDoubleAttributeValue(node *ModelNode, value float64) {
	if node == nil || node._modelNode == nil {
		return
	}
	mmsValue := C.MmsValue_newDouble(C.double(value))
	defer C.MmsValue_delete(mmsValue)
	C.IedServer_updateAttributeValue(is.server, (*C.DataAttribute)(node._modelNode), mmsValue)
}

// UpdateInt32AttributeValue updates a DataAttribute with an Int32 value.
func (is *IedServer) UpdateInt32AttributeValue(node *ModelNode, value int32) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateInt32AttributeValue(is.server, (*C.DataAttribute)(node._modelNode), C.int32_t(value))
}

func (is *IedServer) UpdateInt64AttributeValue(node *ModelNode, value int64) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateInt64AttributeValue(is.server, (*C.DataAttribute)(node._modelNode), C.int64_t(value))
}

func (is *IedServer) UpdateUnsignedAttributeValue(node *ModelNode, value uint32) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateUnsignedAttributeValue(is.server, (*C.DataAttribute)(node._modelNode), C.uint32_t(value))
}

func (is *IedServer) UpdateMmsStringAttributeValue(node *ModelNode, value string) {
	if node == nil || node._modelNode == nil {
		return
	}
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	C.IedServer_updateVisibleStringAttributeValue(is.server, (*C.DataAttribute)(node._modelNode), cValue)
}

// UpdateVisibleStringAttributeValue updates a DataAttribute with a visible string value.
func (is *IedServer) UpdateVisibleStringAttributeValue(attr *DataAttribute, value string) {
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	C.IedServer_updateVisibleStringAttributeValue(is.server, attr.attribute, cValue)
}

// UpdateQuality updates the quality attribute with an UInt16 value
func (is *IedServer) UpdateQuality(node *ModelNode, quality uint16) {
	if node == nil || node._modelNode == nil {
		return
	}
	C.IedServer_updateQuality(is.server, (*C.DataAttribute)(node._modelNode), C.ushort(quality))
}

// GetAttributeValue reads the value of the attribute in the server
func (is *IedServer) GetAttributeValue(node *ModelNode) (*MmsValue, error) {
	mmsValue := C.IedServer_getAttributeValue(is.server, (*C.DataAttribute)(node._modelNode))
	mmsType := MmsType(C.MmsValue_getType(mmsValue))

	value, err := toGoValue(mmsValue, mmsType)
	if err != nil {
		return nil, err
	}
	return &MmsValue{mmsType, value}, nil
}

// GetUTCTimeAttributeValue reads the value of a time attribute in the server
func (is *IedServer) GetUTCTimeAttributeValue(node *ModelNode) int64 {
	timestamp := C.IedServer_getUTCTimeAttributeValue(is.server, (*C.DataAttribute)(node._modelNode))
	return int64(timestamp)
}

// GetNumberOfOpenConnections reads the amount of connections with the server
func (is *IedServer) GetNumberOfOpenConnections() int {
	return int(C.IedServer_getNumberOfOpenConnections(is.server))
}

// SetServerIdentity updates the server identity of the IedServer
func (is *IedServer) SetServerIdentity(vendor string, model string, version string) {
	cVendor := C.CString(vendor)
	cModel := C.CString(model)
	cVersion := C.CString(version)

	defer func() {
		C.free(unsafe.Pointer(cVendor))
		C.free(unsafe.Pointer(cModel))
		C.free(unsafe.Pointer(cVersion))
	}()

	C.IedServer_setServerIdentity(is.server, cVendor, cModel, cVersion)
}
