package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL        = "https://api.openai.com/v1"
	defaultModel          = "whisper-1"
	defaultPort           = "8080"
	defaultMaxUploadBytes = 25 << 20
	defaultTimeout        = 5 * time.Minute
	defaultBackend        = "api"
	backendAPI            = "api"
	backendLocal          = "local"
)

//go:embed web/index.html
var indexHTML string

type config struct {
	apiKey         string
	baseURL        string
	model          string
	backend        string
	localBinary    string
	localModelPath string
	port           string
	maxUploadBytes int64
	requestTimeout time.Duration
}

type server struct {
	cfg    config
	client *http.Client
}

type errorResponse struct {
	Error string `json:"error"`
}

type transcriptionResponse struct {
	Text    string `json:"text"`
	Backend string `json:"backend"`
}

type sendTextRequest struct {
	URL      string `json:"url"`
	Text     string `json:"text"`
	FullText string `json:"full_text"`
}

type sendTextPayload struct {
	Text     string `json:"text"`
	FullText string `json:"full_text"`
	Source   string `json:"source"`
	SentAt   string `json:"sent_at"`
}

type sendTextResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body,omitempty"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	srv := &server{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.requestTimeout,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", srv.app)
	mux.HandleFunc("GET /v1/transcribe", srv.app)
	mux.HandleFunc("GET /healthz", srv.health)
	mux.HandleFunc("POST /v1/transcribe", srv.transcribe)
	mux.HandleFunc("POST /v1/send-text", srv.sendText)

	httpServer := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on :%s", cfg.port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func loadConfig() (config, error) {
	loadEnvFile(".env.local")

	cfg := config{
		apiKey:         strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		baseURL:        getEnv("OPENAI_BASE_URL", defaultBaseURL),
		model:          getEnv("TRANSCRIPTION_MODEL", defaultModel),
		backend:        normalizeBackend(getEnv("TRANSCRIPTION_BACKEND", defaultBackend)),
		localBinary:    strings.TrimSpace(os.Getenv("WHISPER_CPP_BINARY")),
		localModelPath: strings.TrimSpace(os.Getenv("WHISPER_MODEL_PATH")),
		port:           getEnv("PORT", defaultPort),
		maxUploadBytes: getEnvInt64("MAX_UPLOAD_BYTES", defaultMaxUploadBytes),
		requestTimeout: time.Duration(getEnvInt64("REQUEST_TIMEOUT_SECONDS", int64(defaultTimeout/time.Second))) * time.Second,
	}
	switch cfg.backend {
	case backendAPI, backendLocal:
	default:
		return cfg, fmt.Errorf("unsupported TRANSCRIPTION_BACKEND %q: use api or local", cfg.backend)
	}
	return cfg, nil
}

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func normalizeBackend(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) app(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, indexHTML)
}

func (s *server) transcribe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.maxUploadBytes)
	if err := r.ParseMultipartForm(s.cfg.maxUploadBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid multipart form or file too large"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "multipart field 'file' is required"})
		return
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.requestTimeout)
	defer cancel()

	backend := normalizeBackend(r.FormValue("backend"))
	if backend == "" {
		backend = s.cfg.backend
	}

	switch backend {
	case backendAPI:
		s.transcribeWithAPI(ctx, w, r, file, header.Filename)
	case backendLocal:
		s.transcribeWithLocal(ctx, w, r, file, header.Filename)
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unsupported backend: use api or local"})
	}
}

func (s *server) sendText(w http.ResponseWriter, r *http.Request) {
	var input sendTextRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}

	targetURL := strings.TrimSpace(input.URL)
	text := strings.TrimSpace(input.Text)
	fullText := strings.TrimSpace(input.FullText)
	if text == "" {
		text = fullText
	}
	if targetURL == "" || text == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url and text are required"})
		return
	}

	parsedURL, err := url.ParseRequestURI(targetURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url must be a valid http or https URL"})
		return
	}

	payload := sendTextPayload{
		Text:     text,
		FullText: fullText,
		Source:   "whisper-transcription-service",
		SentAt:   time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to encode payload"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create send request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "send request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: fmt.Sprintf("target returned %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))})
		return
	}

	writeJSON(w, http.StatusOK, sendTextResponse{
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(responseBody)),
	})
}

func (s *server) transcribeWithAPI(ctx context.Context, w http.ResponseWriter, r *http.Request, file multipart.File, filename string) {
	if s.cfg.apiKey == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "OPENAI_API_KEY is required for api backend"})
		return
	}

	body, contentType, err := buildOpenAITranscriptionBody(file, filename, s.cfg.model, r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.baseURL, "/")+"/audio/transcriptions", body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create upstream request"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := s.client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "transcription request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	forwardUpstreamResponse(w, resp)
}

func (s *server) transcribeWithLocal(ctx context.Context, w http.ResponseWriter, r *http.Request, file multipart.File, filename string) {
	if s.cfg.localBinary == "" || s.cfg.localModelPath == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "WHISPER_CPP_BINARY and WHISPER_MODEL_PATH are required for local backend"})
		return
	}

	inputPath, cleanup, err := saveUploadToTemp(file, filename)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer cleanup()

	outputPrefix := strings.TrimSuffix(inputPath, filepath.Ext(inputPath))
	args := []string{
		"-m", s.cfg.localModelPath,
		"-f", inputPath,
		"-otxt",
		"-of", outputPrefix,
	}
	if language := strings.TrimSpace(r.FormValue("language")); language != "" {
		args = append(args, "-l", language)
	}
	if prompt := strings.TrimSpace(r.FormValue("prompt")); prompt != "" {
		args = append(args, "--prompt", prompt)
	}

	cmd := exec.CommandContext(ctx, s.cfg.localBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "local whisper failed: " + strings.TrimSpace(string(output))})
		return
	}

	textPath := outputPrefix + ".txt"
	textBytes, err := os.ReadFile(textPath)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "local whisper did not produce text output"})
		return
	}
	_ = os.Remove(textPath)

	writeJSON(w, http.StatusOK, transcriptionResponse{
		Text:    strings.TrimSpace(string(textBytes)),
		Backend: backendLocal,
	})
}

func saveUploadToTemp(file multipart.File, filename string) (string, func(), error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".wav"
	}

	tempFile, err := os.CreateTemp("", "transcription-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary audio file")
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", nil, fmt.Errorf("failed to save uploaded audio")
	}

	cleanup := func() {
		_ = os.Remove(tempFile.Name())
	}
	return tempFile.Name(), cleanup, nil
}

func buildOpenAITranscriptionBody(file multipart.File, filename, defaultModel string, r *http.Request) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", fmt.Errorf("failed to prepare file field")
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("failed to read uploaded file")
	}

	model := strings.TrimSpace(r.FormValue("model"))
	if model == "" {
		model = defaultModel
	}
	if err := writer.WriteField("model", model); err != nil {
		return nil, "", fmt.Errorf("failed to write model field")
	}

	for _, field := range []string{"language", "prompt", "response_format", "temperature"} {
		if err := copyOptionalField(writer, r, field); err != nil {
			return nil, "", err
		}
	}

	for _, field := range []string{"timestamp_granularities", "timestamp_granularities[]"} {
		for _, value := range r.MultipartForm.Value[field] {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if err := writer.WriteField("timestamp_granularities[]", value); err != nil {
				return nil, "", fmt.Errorf("failed to write timestamp_granularities field")
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize multipart body")
	}

	return &body, writer.FormDataContentType(), nil
}

func copyOptionalField(writer *multipart.Writer, r *http.Request, field string) error {
	value := strings.TrimSpace(r.FormValue(field))
	if value == "" {
		return nil
	}
	if err := writer.WriteField(field, value); err != nil {
		return fmt.Errorf("failed to write %s field", field)
	}
	return nil
}

func forwardUpstreamResponse(w http.ResponseWriter, resp *http.Response) {
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("failed to write upstream response: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write json response: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
