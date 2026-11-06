package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chatbot-pdf-video/backend/config"
	"chatbot-pdf-video/backend/gemini"
	"chatbot-pdf-video/backend/pdf"
	"chatbot-pdf-video/backend/video"
	"chatbot-pdf-video/backend/whatsapp"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local SaaS dev
	},
}

type Server struct {
	gemini *gemini.GeminiService
	wa     *whatsapp.WhatsAppManager
	pdf    *pdf.PDFService
	video  *video.VideoService
}

func main() {
	cfg := config.LoadConfig()

	// Detect yt-dlp & ffmpeg binaries if not configured
	localAppData := os.Getenv("LOCALAPPDATA")
	wingetYtDlp := filepath.Join(localAppData, `Microsoft\WinGet\Packages\yt-dlp.yt-dlp_Microsoft.Winget.Source_8wekyb3d8bbwe\yt-dlp.exe`)
	if _, err := os.Stat(wingetYtDlp); err == nil && cfg.YtDlpPath == "yt-dlp" {
		cfg.YtDlpPath = wingetYtDlp
	}

	wingetFFmpeg := filepath.Join(localAppData, `Microsoft\WinGet\Packages\yt-dlp.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-N-127043-g5a54fcf75e-win64-gpl\bin\ffmpeg.exe`)
	if _, err := os.Stat(wingetFFmpeg); err == nil && cfg.FFmpegPath == "ffmpeg" {
		cfg.FFmpegPath = wingetFFmpeg
	}
	_ = config.SaveConfig(cfg)

	geminiSvc := gemini.NewGeminiService()
	pdfSvc := pdf.NewPDFService()
	videoSvc := video.NewVideoService("temp_downloads")

	waMgr, err := whatsapp.NewWhatsAppManager(geminiSvc)
	if err != nil {
		log.Fatalf("Error inicializando WhatsApp: %v", err)
	}

	// Try auto-starting WhatsApp
	go func() {
		if err := waMgr.Start(); err != nil {
			log.Printf("[WhatsApp] Not auto-connected: %v", err)
		}
	}()

	srv := &Server{
		gemini: geminiSvc,
		wa:     waMgr,
		pdf:    pdfSvc,
		video:  videoSvc,
	}

	mux := http.NewServeMux()

	// CORS wrapper
	enableCORS := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next(w, r)
		}
	}

	// API Endpoints
	// Config
	mux.HandleFunc("/api/config", enableCORS(srv.handleConfig))

	// Gemini Test / Models
	mux.HandleFunc("/api/gemini/models", enableCORS(srv.handleGeminiModels))
	mux.HandleFunc("/api/gemini/test", enableCORS(srv.handleGeminiTest))

	// WhatsApp
	mux.HandleFunc("/api/whatsapp/status", enableCORS(srv.handleWhatsAppStatus))
	mux.HandleFunc("/api/whatsapp/connect", enableCORS(srv.handleWhatsAppConnect))
	mux.HandleFunc("/api/whatsapp/disconnect", enableCORS(srv.handleWhatsAppDisconnect))
	mux.HandleFunc("/api/whatsapp/logout", enableCORS(srv.handleWhatsAppLogout))
	mux.HandleFunc("/api/whatsapp/messages", enableCORS(srv.handleWhatsAppMessages))
	mux.HandleFunc("/api/whatsapp/send", enableCORS(srv.handleWhatsAppSend))
	mux.HandleFunc("/ws/whatsapp", srv.handleWhatsAppWS)

	// PDF Operations
	mux.HandleFunc("/api/pdf/merge", enableCORS(srv.handlePDFMerge))
	mux.HandleFunc("/api/pdf/split", enableCORS(srv.handlePDFSplit))
	mux.HandleFunc("/api/pdf/compress", enableCORS(srv.handlePDFCompress))

	// Video Operations
	mux.HandleFunc("/api/video/info", enableCORS(srv.handleVideoInfo))
	mux.HandleFunc("/api/video/download", enableCORS(srv.handleVideoDownload))
	mux.HandleFunc("/api/video/jobs", enableCORS(srv.handleVideoJobs))

	// Static downloads
	fileServer := http.FileServer(http.Dir("temp_downloads"))
	mux.Handle("/downloads/", http.StripPrefix("/downloads/", fileServer))

	// Frontend Static assets (if dist exists)
	if _, err := os.Stat("frontend/dist"); err == nil {
		frontendFS := http.FileServer(http.Dir("frontend/dist"))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Join("frontend/dist", r.URL.Path)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, "frontend/dist/index.html")
				return
			}
			frontendFS.ServeHTTP(w, r)
		})
	}

	port := 8080
	log.Printf("🚀 Servidor OmniBot Suite iniciado en http://localhost:%d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		log.Fatalf("Fallo al iniciar servidor: %v", err)
	}
}

// Config Handlers
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cfg := config.GetConfig()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
		return
	}

	if r.Method == "POST" {
		var newCfg config.AppConfig
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.SaveConfig(newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "config": newCfg})
		return
	}
}

// Gemini Handlers
func (s *Server) handleGeminiModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := config.GetConfig()
	if len(cfg.GeminiAPIKeys) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cheapest_model": "Sin llaves configuradas",
			"status":         "no_keys",
			"total_keys":     0,
		})
		return
	}

	key := cfg.GeminiAPIKeys[0]
	cheapest, err := s.gemini.DiscoverCheapestFlashModel(key)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cheapest_model": "gemini-2.5-flash (predeterminado)",
			"status":         "fallback",
			"error":          err.Error(),
			"total_keys":     len(cfg.GeminiAPIKeys),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"cheapest_model": cheapest,
		"status":         "ok",
		"total_keys":     len(cfg.GeminiAPIKeys),
	})
}

func (s *Server) handleGeminiTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Prompt string `json:"prompt"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Prompt == "" {
		body.Prompt = "Hola, responde con un saludo cordial y confirma que estás activo."
	}

	reply, model, keyUsed, err := s.gemini.GenerateReply(body.Prompt, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"reply":      reply,
		"model_used": model,
		"key_used":   keyUsed,
	})
}

// WhatsApp Handlers
func (s *Server) handleWhatsAppStatus(w http.ResponseWriter, r *http.Request) {
	status, qr := s.wa.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  status,
		"qr_code": qr,
	})
}

func (s *Server) handleWhatsAppConnect(w http.ResponseWriter, r *http.Request) {
	go func() {
		if err := s.wa.Start(); err != nil {
			log.Printf("[WhatsApp Connect Error]: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Iniciando conexión..."})
}

func (s *Server) handleWhatsAppDisconnect(w http.ResponseWriter, r *http.Request) {
	_ = s.wa.Disconnect()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Desconectado"})
}

func (s *Server) handleWhatsAppLogout(w http.ResponseWriter, r *http.Request) {
	_ = s.wa.Logout()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Sesión cerrada"})
}

func (s *Server) handleWhatsAppMessages(w http.ResponseWriter, r *http.Request) {
	msgs := s.wa.GetMessages()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (s *Server) handleWhatsAppSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Recipient string `json:"recipient"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.wa.SendManualMessage(body.Recipient, body.Message); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleWhatsAppWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch := s.wa.Subscribe()
	defer s.wa.Unsubscribe(ch)

	// Send current status right away
	st, qr := s.wa.GetStatus()
	_ = conn.WriteJSON(map[string]interface{}{
		"status":  st,
		"qr_code": qr,
	})

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ch:
			st, qr := s.wa.GetStatus()
			_ = conn.WriteJSON(map[string]interface{}{
				"status":   st,
				"qr_code":  qr,
				"messages": s.wa.GetMessages(),
			})
		case <-ticker.C:
			st, qr := s.wa.GetStatus()
			err := conn.WriteJSON(map[string]interface{}{
				"status":   st,
				"qr_code":  qr,
				"messages": s.wa.GetMessages(),
			})
			if err != nil {
				return
			}
		}
	}
}

// PDF Handlers
func (s *Server) handlePDFMerge(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB
		http.Error(w, "Error al procesar archivos: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) < 2 {
		http.Error(w, "Se requieren al menos 2 archivos PDF para unir", http.StatusBadRequest)
		return
	}

	_ = os.MkdirAll("uploads", 0755)
	var tempPaths []string
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		tempName := filepath.Join("uploads", fmt.Sprintf("upload_%d_%s", time.Now().UnixNano(), fileHeader.Filename))
		out, err := os.Create(tempName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = io.Copy(out, file)
		out.Close()
		tempPaths = append(tempPaths, tempName)
	}

	mergedName := fmt.Sprintf("merged_%d.pdf", time.Now().UnixNano())
	mergedPath := filepath.Join("temp_downloads", mergedName)

	if err := s.pdf.MergePDFs(tempPaths, mergedPath); err != nil {
		http.Error(w, fmt.Sprintf("Error uniendo PDFs: %v", err), http.StatusInternalServerError)
		return
	}

	// Clean upload temp files
	for _, p := range tempPaths {
		_ = os.Remove(p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"filename":     mergedName,
		"download_url": "/downloads/" + mergedName,
	})
}

func (s *Server) handlePDFSplit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Archivo requerido", http.StatusBadRequest)
		return
	}
	defer file.Close()

	spanStr := r.FormValue("span")
	span := 1
	if sVal, err := strconv.Atoi(spanStr); err == nil && sVal > 0 {
		span = sVal
	}

	tempUpload := filepath.Join("uploads", fmt.Sprintf("split_%d_%s", time.Now().UnixNano(), fileHeader.Filename))
	out, err := os.Create(tempUpload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = io.Copy(out, file)
	out.Close()
	defer os.Remove(tempUpload)

	outDir := filepath.Join("temp_downloads", fmt.Sprintf("split_res_%d", time.Now().UnixNano()))
	createdFiles, err := s.pdf.SplitPDF(tempUpload, outDir, span)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error dividiendo PDF: %v", err), http.StatusInternalServerError)
		return
	}

	type DownloadItem struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	var downloads []DownloadItem
	for _, f := range createdFiles {
		rel, _ := filepath.Rel("temp_downloads", f)
		downloads = append(downloads, DownloadItem{
			Name: filepath.Base(f),
			URL:  "/downloads/" + filepath.ToSlash(rel),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"files":   downloads,
	})
}

func (s *Server) handlePDFCompress(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Archivo requerido", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempUpload := filepath.Join("uploads", fmt.Sprintf("comp_%d_%s", time.Now().UnixNano(), fileHeader.Filename))
	out, err := os.Create(tempUpload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = io.Copy(out, file)
	out.Close()
	defer os.Remove(tempUpload)

	compressedName := fmt.Sprintf("compressed_%d_%s", time.Now().UnixNano(), fileHeader.Filename)
	compressedPath := filepath.Join("temp_downloads", compressedName)

	if err := s.pdf.CompressPDF(tempUpload, compressedPath); err != nil {
		http.Error(w, fmt.Sprintf("Error comprimiendo PDF: %v", err), http.StatusInternalServerError)
		return
	}

	origInfo, _ := os.Stat(tempUpload)
	compInfo, _ := os.Stat(compressedPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"filename":        compressedName,
		"download_url":    "/downloads/" + compressedName,
		"original_size":   origInfo.Size(),
		"compressed_size": compInfo.Size(),
	})
}

// Video Handlers
func (s *Server) handleVideoInfo(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if strings.TrimSpace(url) == "" {
		http.Error(w, "URL requerida", http.StatusBadRequest)
		return
	}

	info, err := s.video.GetInfo(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (s *Server) handleVideoDownload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL     string `json:"url"`
		Format  string `json:"format"`
		Quality string `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.URL == "" {
		http.Error(w, "URL requerida", http.StatusBadRequest)
		return
	}

	job, err := s.video.StartDownload(body.URL, body.Format, body.Quality)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleVideoJobs(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID != "" {
		job := s.video.GetJob(jobID)
		if job == nil {
			http.Error(w, "Job no encontrado", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
		return
	}

	jobs := s.video.ListJobs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}
