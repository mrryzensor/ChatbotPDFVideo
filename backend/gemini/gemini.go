package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"chatbot-pdf-video/backend/config"
)

type GeminiService struct {
	mu           sync.RWMutex
	lastFailover map[string]time.Time
	httpClient   *http.Client
}

type ModelInfo struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

type GenerateContentRequest struct {
	Contents          []Content         `json:"contents"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
}

type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type GenerateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func NewGeminiService() *GeminiService {
	return &GeminiService{
		lastFailover: make(map[string]time.Time),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DiscoverCheapestFlashModel dynamically queries Google AI Studio API for available models
// and picks the optimal, lowest-cost Gemini Flash model available for the user's API key
func (s *GeminiService) DiscoverCheapestFlashModel(apiKey string) (string, error) {
	if apiKey == "" {
		return "gemini-3.5-flash-lite", nil
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "gemini-3.5-flash-lite", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "gemini-3.5-flash-lite", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "gemini-3.5-flash-lite", fmt.Errorf("list models error (%d): %s", resp.StatusCode, string(body))
	}

	var listResp ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return "gemini-3.5-flash-lite", err
	}

	// Cost priority heuristic for Flash text generation:
	// Prioritize the latest active, most economical Flash-Lite / Flash models verified by Google AI Studio
	rankings := []string{
		"gemini-3.5-flash-lite",
		"gemini-3.1-flash-lite",
		"gemini-3.5-flash",
		"gemini-3.8-flash",
		"gemini-flash-latest",
	}

	foundMap := make(map[string]string)
	for _, m := range listResp.Models {
		cleanName := strings.TrimPrefix(m.Name, "models/")
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}

		if supportsGenerate && strings.Contains(strings.ToLower(cleanName), "flash") {
			foundMap[cleanName] = cleanName
		}
	}

	for _, ranked := range rankings {
		for key := range foundMap {
			if key == ranked {
				return key, nil
			}
		}
	}

	return "gemini-3.5-flash-lite", nil
}

// GenerateReply attempts to produce a response using the available API keys with automatic failover
func (s *GeminiService) GenerateReply(prompt string, history []Content) (string, string, string, error) {
	cfg := config.GetConfig()
	keys := cfg.GeminiAPIKeys
	if len(keys) == 0 {
		return "", "", "", errors.New("no hay llaves de API de Google Gemini configuradas. Por favor agrega tu API Key en la pestaña 'Configuración & API Keys' y haz clic en 'Guardar Configuración'")
	}

	systemInstruction := cfg.SystemPrompt
	if systemInstruction == "" {
		systemInstruction = "Eres un asistente virtual de WhatsApp muy servicial y amigable."
	}

	var lastErr error
	for idx, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		// Candidates to try with failover: primary discovered cheapest model, then fallback models
		targetModel := cfg.PreferredModel
		if targetModel == "" || targetModel == "auto" {
			targetModel = "gemini-3.5-flash-lite"
		}

		modelsToTry := []string{targetModel, "gemini-3.5-flash-lite", "gemini-3.1-flash-lite"}

		for _, modelName := range modelsToTry {
			reply, err := s.callGeminiAPI(key, modelName, systemInstruction, prompt, history)
			if err == nil && reply != "" {
				return reply, modelName, fmt.Sprintf("Key #%d (...%s)", idx+1, keyMask(key)), nil
			}
			lastErr = err
			log.Printf("[Gemini Failover] Error con llave #%d y modelo %s: %v", idx+1, modelName, err)
		}
	}

	return "", "", "", fmt.Errorf("todas las API keys fallaron. Último error: %v", lastErr)
}

func (s *GeminiService) callGeminiAPI(apiKey, model, systemPrompt, prompt string, history []Content) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	var contents []Content
	if len(history) > 0 {
		contents = append(contents, history...)
	}
	contents = append(contents, Content{
		Role: "user",
		Parts: []Part{
			{Text: prompt},
		},
	})

	reqBody := GenerateContentRequest{
		Contents: contents,
		SystemInstruction: &Content{
			Parts: []Part{
				{Text: systemPrompt},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature: 0.7,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var genResp GenerateContentResponse
	if err := json.Unmarshal(bodyBytes, &genResp); err != nil {
		return "", fmt.Errorf("error al decodificar respuesta JSON: %w (raw: %s)", err, string(bodyBytes))
	}

	if genResp.Error != nil {
		return "", fmt.Errorf("Gemini API Error (%d): %s", genResp.Error.Code, genResp.Error.Message)
	}

	if len(genResp.Candidates) == 0 || len(genResp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("Gemini devolvió una respuesta vacía o bloqueada por políticas")
	}

	text := genResp.Candidates[0].Content.Parts[0].Text
	return strings.TrimSpace(text), nil
}

func keyMask(k string) string {
	if len(k) <= 6 {
		return "***"
	}
	return k[len(k)-4:]
}
