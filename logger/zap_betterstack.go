package zap_betterstack

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Custom log level for NOTICE (below DebugLevel, non-error informational logs)
const NoticeLevel zapcore.Level = -2

// logEntry represents a single log entry for Better Stack
type logEntry struct {
	Timestamp  string         `json:"timestamp"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	TraceID    string         `json:"traceID"` // to track the flow of request
	Layer      string         `json:"layer"`   // repo or service etc,, in the clean architecture layers.
	Attributes map[string]any `json:"attributes"`
}

// BetterStackLogStreamer is a service for streaming service-level logs
type BetterStackLogStreamer struct {
	sourceToken string
	environment string
	uploadURL   string
	logger      *zap.Logger
	client      *http.Client
	fileWriter  io.Writer
	fileMu      sync.Mutex
}

// NewBetterStackLogStreamer creates a new BetterStackLogStreamer instance
func NewBetterStackLogStreamer(sourceToken, environment, uploadURL string, logger *zap.Logger) *BetterStackLogStreamer {
	streamer := &BetterStackLogStreamer{
		sourceToken: sourceToken,
		environment: environment,
		uploadURL:   uploadURL,
		logger:      logger,
	}

	// Initialize file writer for development
	if environment == "development" {
		f, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("Failed to open log file", zap.Error(err))
			streamer.fileWriter = os.Stderr
		} else {
			streamer.fileWriter = f
		}
	}

	// Initialize HTTP client for production
	if environment == "production" {
		streamer.client = &http.Client{Timeout: 10 * time.Second}
	}

	return streamer
}

// Log streams a service-level log to a file (development) or Better Stack (production)
func (s *BetterStackLogStreamer) Log(level zapcore.Level, traceID string, message string, attributes map[string]any, layer string, err error) {
	
	if traceID == ""{
		return
	}
	
	// Map zap level to Better Stack level string
	var levelStr string
	switch level {
	case zapcore.ErrorLevel:
		levelStr = "ERROR"
	case zapcore.WarnLevel:
		levelStr = "WARN"
	case zapcore.InfoLevel:
		levelStr = "INFO"
	case NoticeLevel:
		levelStr = "NOTICE"
	case zapcore.DebugLevel:
		levelStr = "DEBUG"
	default:
		levelStr = "UNKNOWN"
	}

	// Initialize attributes if nil
	if attributes == nil {
		attributes = make(map[string]any)
	}

	// Create log entry
	entry := logEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Level:      levelStr,
		Message:    message,
		TraceID:    traceID,
		Layer:      layer,
		Attributes: attributes,
	}

	// Marshal log to JSON
	body, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		s.logger.Error("Failed to marshal log", zap.Error(marshalErr))
		return
	}

	if s.environment == "development" {
		// Write to file in development
		s.fileMu.Lock()
		defer s.fileMu.Unlock()
		_, writeErr := s.fileWriter.Write(append(body, '\n'))
		if writeErr != nil {
			s.logger.Error("Failed to write log to file", zap.Error(writeErr))
		}
	} else {
		// Send to Better Stack in production
		req, err := http.NewRequest("POST", s.uploadURL, bytes.NewReader(body))
		if err != nil {
			s.logger.Error("Failed to create HTTP request", zap.Error(err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.sourceToken)

		// Send log asynchronously
		go func() {
			resp, err := s.client.Do(req)
			if err != nil {
				s.logger.Error("Failed to send log to Better Stack", zap.Error(err))
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusAccepted {
				s.logger.Error("Unexpected response from Better Stack", zap.String("status", resp.Status))
			}
		}()
	}

	// Also log to Zap for console visibility
	s.logger.Log(level, message, zap.Any("attributes", attributes))
}
