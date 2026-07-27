package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
)

// Larger chunks + a parallel window cut round-trips while staying under the
// elevated WebSocket frame limit (base64 + CKX1 envelope overhead).
const (
	fileStreamChunkSize   = 96 * 1024
	fileStreamConcurrency = 4
)

type fileChunkPayload struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Size   int    `json:"size"`
}

type fileChunkResponse struct {
	Content   string `json:"content"`
	BytesRead int    `json:"bytes_read"`
	FileSize  int64  `json:"file_size"`
	Offset    int64  `json:"offset"`
}

type streamJob struct {
	offset int64
	size   int
}

// handleFileStream streams a remote device file by aggregating file_read_chunk.
// GET /files/stream?device_id=&path=  optional header Range: bytes=N-
//
// Uses a windowed parallel fetch with ordered assembly so multiple chunks are
// in flight on the agent while HTTP bytes are written in offset order.
func (s *Server) handleFileStream(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())
	if !s.permissionChecker.HasPermission(admin, "file:read") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	deviceIDStr := r.URL.Query().Get("device_id")
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if deviceIDStr == "" || path == "" {
		s.writeError(w, http.StatusBadRequest, "MISSING_PARAM", "device_id and path are required")
		return
	}
	deviceID, err := uuid.Parse(deviceIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid device ID")
		return
	}

	rangeStart := parseRangeStart(r.Header.Get("Range"))
	cmdBuilder := dispatcher.NewCommandBuilder(s.dispatcher).WithAdmin(admin).WithContext(getClientIP(r), r.UserAgent())

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)

	probe, firstRaw, errMsg := readDeviceChunk(cmdBuilder, deviceID, path, rangeStart, fileStreamChunkSize)
	if errMsg != "" {
		s.writeError(w, http.StatusBadGateway, "DEVICE_READ_FAILED", errMsg)
		return
	}
	if len(firstRaw) == 0 {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
		return
	}

	totalSize := probe.FileSize
	writeStreamHeaders(w, rangeStart, totalSize)

	if _, err := w.Write(firstRaw); err != nil {
		return
	}
	if flusher != nil {
		flusher.Flush()
	}

	nextOffset := rangeStart + int64(len(firstRaw))
	if totalSize > 0 && nextOffset >= totalSize {
		return
	}
	if totalSize <= 0 && len(firstRaw) < fileStreamChunkSize {
		return
	}

	writeChunk := func(raw []byte) bool {
		if len(raw) == 0 {
			return true
		}
		if _, err := w.Write(raw); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}

	if totalSize > 0 {
		jobs := buildStreamJobs(nextOffset, totalSize)
		if !streamParallel(cmdBuilder, deviceID, path, jobs, nextOffset, totalSize, writeChunk) {
			return
		}
		return
	}

	// Unknown size: keep sequential reads until a short/empty chunk.
	offset := nextOffset
	for {
		chunk, raw, msg := readDeviceChunk(cmdBuilder, deviceID, path, offset, fileStreamChunkSize)
		if msg != "" || len(raw) == 0 {
			return
		}
		if !writeChunk(raw) {
			return
		}
		offset += int64(len(raw))
		if chunk.FileSize > 0 && offset >= chunk.FileSize {
			return
		}
		if len(raw) < fileStreamChunkSize {
			return
		}
	}
}

func parseRangeStart(rh string) int64 {
	if rh == "" || !strings.HasPrefix(rh, "bytes=") {
		return 0
	}
	part := strings.TrimPrefix(rh, "bytes=")
	idx := strings.Index(part, "-")
	if idx < 0 {
		return 0
	}
	startStr := strings.TrimSpace(part[:idx])
	if startStr == "" {
		return 0
	}
	n, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeStreamHeaders(w http.ResponseWriter, rangeStart, totalSize int64) {
	if totalSize < 0 {
		return
	}
	if rangeStart > 0 {
		remaining := totalSize - rangeStart
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("Content-Length", strconv.FormatInt(remaining, 10))
		end := totalSize - 1
		if end < rangeStart {
			end = rangeStart
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, end, totalSize))
		w.WriteHeader(http.StatusPartialContent)
		return
	}
	w.Header().Set("Content-Length", strconv.FormatInt(totalSize, 10))
}

func buildStreamJobs(start, totalSize int64) []streamJob {
	var jobs []streamJob
	for off := start; off < totalSize; off += int64(fileStreamChunkSize) {
		remain := totalSize - off
		sz := fileStreamChunkSize
		if remain < int64(sz) {
			sz = int(remain)
		}
		if sz <= 0 {
			break
		}
		jobs = append(jobs, streamJob{offset: off, size: sz})
	}
	return jobs
}

func readDeviceChunk(
	cmdBuilder *dispatcher.CommandBuilder,
	deviceID uuid.UUID,
	path string,
	offset int64,
	size int,
) (fileChunkResponse, []byte, string) {
	payloadBytes, _ := json.Marshal(fileChunkPayload{
		Path:   path,
		Offset: offset,
		Size:   size,
	})
	result := <-cmdBuilder.FileReadChunk(deviceID, string(payloadBytes))
	if result.Status != string(models.StatusSuccess) {
		msg := result.Error
		if msg == "" {
			msg = "Failed to read file from device"
		}
		return fileChunkResponse{}, nil, msg
	}
	var chunk fileChunkResponse
	if err := json.Unmarshal(result.Data, &chunk); err != nil {
		return fileChunkResponse{}, nil, "Invalid chunk response from device"
	}
	if chunk.BytesRead <= 0 || chunk.Content == "" {
		return chunk, nil, ""
	}
	raw, err := base64.StdEncoding.DecodeString(chunk.Content)
	if err != nil {
		return fileChunkResponse{}, nil, "Chunk decode failed"
	}
	return chunk, raw, ""
}

// streamParallel fetches chunks concurrently and writes them in offset order
// as soon as the next contiguous chunk is ready (continuous pipeline).
func streamParallel(
	cmdBuilder *dispatcher.CommandBuilder,
	deviceID uuid.UUID,
	path string,
	jobs []streamJob,
	nextWrite int64,
	totalSize int64,
	writeChunk func([]byte) bool,
) bool {
	if len(jobs) == 0 {
		return true
	}

	type result struct {
		offset int64
		data   []byte
		err    string
	}

	results := make(chan result, fileStreamConcurrency)
	var wg sync.WaitGroup
	sem := make(chan struct{}, fileStreamConcurrency)

	for _, j := range jobs {
		j := j
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			_, raw, msg := readDeviceChunk(cmdBuilder, deviceID, path, j.offset, j.size)
			results <- result{offset: j.offset, data: raw, err: msg}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	pending := make(map[int64][]byte, fileStreamConcurrency)
	remaining := len(jobs)

	for remaining > 0 || len(pending) > 0 {
		if raw, ok := pending[nextWrite]; ok {
			delete(pending, nextWrite)
			if !writeChunk(raw) {
				return false
			}
			nextWrite += int64(len(raw))
			if nextWrite >= totalSize {
				return true
			}
			continue
		}

		res, ok := <-results
		if !ok {
			if _, have := pending[nextWrite]; !have {
				return false
			}
			continue
		}
		remaining--
		if res.err != "" {
			return false
		}
		pending[res.offset] = res.data

		for {
			raw, have := pending[nextWrite]
			if !have {
				break
			}
			delete(pending, nextWrite)
			if !writeChunk(raw) {
				return false
			}
			nextWrite += int64(len(raw))
			if nextWrite >= totalSize {
				return true
			}
		}
	}
	return nextWrite >= totalSize
}
