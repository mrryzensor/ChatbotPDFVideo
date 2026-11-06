package video

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chatbot-pdf-video/backend/config"
)

type VideoInfo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Thumbnail   string  `json:"thumbnail"`
	Duration    float64 `json:"duration"`
	Uploader    string  `json:"uploader"`
	Description string  `json:"description"`
}

type DownloadJob struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Format    string    `json:"format"` // "video" or "audio"
	Quality   string    `json:"quality"`
	Status    string    `json:"status"` // "queued", "downloading", "completed", "error"
	Progress  string    `json:"progress"`
	Filename  string    `json:"filename"`
	FilePath  string    `json:"filepath"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type VideoService struct {
	jobsMu sync.RWMutex
	jobs   map[string]*DownloadJob
	outDir string
}

func NewVideoService(outDir string) *VideoService {
	_ = os.MkdirAll(outDir, 0755)
	return &VideoService{
		jobs:   make(map[string]*DownloadJob),
		outDir: outDir,
	}
}

// NormalizeURL cleans tracking query parameters and fixes common link formats
// (e.g. youtu.be, youtube.com/shorts, facebook, twitter, instagram, tiktok)
func NormalizeURL(rawInput string) string {
	rawInput = strings.TrimSpace(rawInput)
	if rawInput == "" {
		return ""
	}

	// Add protocol if missing
	if !strings.HasPrefix(rawInput, "http://") && !strings.HasPrefix(rawInput, "https://") {
		rawInput = "https://" + rawInput
	}

	parsed, err := url.Parse(rawInput)
	if err != nil {
		return rawInput
	}

	host := strings.ToLower(parsed.Host)

	// YouTube normalization
	if strings.Contains(host, "youtu.be") {
		// e.g. https://youtu.be/VIDEO_ID?si=xxx -> https://www.youtube.com/watch?v=VIDEO_ID
		vidID := strings.Trim(parsed.Path, "/")
		if vidID != "" {
			return fmt.Sprintf("https://www.youtube.com/watch?v=%s", vidID)
		}
	} else if strings.Contains(host, "youtube.com") {
		// Handle Shorts: /shorts/VIDEO_ID -> /watch?v=VIDEO_ID
		if strings.HasPrefix(parsed.Path, "/shorts/") {
			vidID := strings.TrimPrefix(parsed.Path, "/shorts/")
			vidID = strings.Trim(vidID, "/")
			return fmt.Sprintf("https://www.youtube.com/watch?v=%s", vidID)
		}

		// Keep only 'v' parameter for standard watch URLs, strip tracking (si, pp, t, feature)
		if parsed.Path == "/watch" {
			q := parsed.Query()
			v := q.Get("v")
			if v != "" {
				return fmt.Sprintf("https://www.youtube.com/watch?v=%s", v)
			}
		}
	}

	// Remove general marketing/tracking query params (fbclid, igshid, utm_*, etc.)
	q := parsed.Query()
	for k := range q {
		lowerK := strings.ToLower(k)
		if strings.HasPrefix(lowerK, "utm_") || lowerK == "fbclid" || lowerK == "igshid" || lowerK == "si" {
			q.Del(k)
		}
	}
	parsed.RawQuery = q.Encode()

	return parsed.String()
}

func (s *VideoService) getYtDlpBin() string {
	cfg := config.GetConfig()
	if cfg.YtDlpPath != "" {
		if _, err := os.Stat(cfg.YtDlpPath); err == nil {
			return cfg.YtDlpPath
		}
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	wingetYtDlp := filepath.Join(localAppData, `Microsoft\WinGet\Packages\yt-dlp.yt-dlp_Microsoft.Winget.Source_8wekyb3d8bbwe\yt-dlp.exe`)
	if _, err := os.Stat(wingetYtDlp); err == nil {
		return wingetYtDlp
	}

	return "yt-dlp"
}

func (s *VideoService) getFFmpegBin() string {
	cfg := config.GetConfig()
	if cfg.FFmpegPath != "" {
		if _, err := os.Stat(cfg.FFmpegPath); err == nil {
			return cfg.FFmpegPath
		}
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	wingetFFmpeg := filepath.Join(localAppData, `Microsoft\WinGet\Packages\yt-dlp.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-N-127043-g5a54fcf75e-win64-gpl\bin\ffmpeg.exe`)
	if _, err := os.Stat(wingetFFmpeg); err == nil {
		return wingetFFmpeg
	}

	gyanFFmpeg := filepath.Join(localAppData, `Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-9.0.2-full_build\bin\ffmpeg.exe`)
	if _, err := os.Stat(gyanFFmpeg); err == nil {
		return gyanFFmpeg
	}

	return "ffmpeg"
}

func (s *VideoService) getDenoBin() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	wingetDeno := filepath.Join(localAppData, `Microsoft\WinGet\Packages\DenoLand.Deno_Microsoft.Winget.Source_8wekyb3d8bbwe\deno.exe`)
	if _, err := os.Stat(wingetDeno); err == nil {
		return wingetDeno
	}
	return ""
}

// GetInfo extracts video metadata using robust extractor flags & normalized print format
func (s *VideoService) GetInfo(rawURL string) (*VideoInfo, error) {
	normURL := NormalizeURL(rawURL)
	ytBin := s.getYtDlpBin()
	denoBin := s.getDenoBin()

	// Try with robust extractor args (android,web avoids bot-checks)
	args := []string{
		"--no-warnings",
		"--extractor-args", "youtube:player_client=android,web",
		"--print", "%(id)s|||%(title)s|||%(uploader)s|||%(duration)s|||%(thumbnail)s",
	}

	if denoBin != "" {
		args = append(args, "--js-runtimes", fmt.Sprintf("deno:%s", denoBin))
	}

	args = append(args, normURL)

	cmd := exec.Command(ytBin, args...)
	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	if err != nil {
		// Parse human-friendly error message
		if strings.Contains(outputStr, "This video is unavailable") {
			return nil, fmt.Errorf("el video no está disponible (verifica que el enlace sea correcto y público)")
		}
		if strings.Contains(outputStr, "Private video") {
			return nil, fmt.Errorf("el video es privado o requiere inicio de sesión")
		}
		if strings.Contains(outputStr, "Sign in to confirm") {
			return nil, fmt.Errorf("la plataforma requiere verificación o captcha para este contenido")
		}

		// Fallback: try default without client override
		cmdFallback := exec.Command(ytBin, "--no-warnings", "--print", "%(id)s|||%(title)s|||%(uploader)s|||%(duration)s|||%(thumbnail)s", normURL)
		fallbackOut, fallbackErr := cmdFallback.CombinedOutput()
		if fallbackErr == nil {
			outputStr = string(fallbackOut)
		} else {
			return nil, fmt.Errorf("error al obtener info del video: %s", cleanError(outputStr))
		}
	}

	// Find line containing delimiter "|||"
	lines := strings.Split(outputStr, "\n")
	var dataLine string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "|||") {
			dataLine = l
			break
		}
	}

	if dataLine == "" {
		return nil, fmt.Errorf("no se pudo extraer la información del enlace ingresado")
	}

	parts := strings.Split(dataLine, "|||")
	info := &VideoInfo{
		ID:        strings.TrimSpace(parts[0]),
		Title:     strings.TrimSpace(parts[1]),
		Uploader:  "Desconocido",
		Thumbnail: "",
	}

	if len(parts) > 2 && parts[2] != "NA" && parts[2] != "" {
		info.Uploader = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		dVal, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		info.Duration = dVal
	}
	if len(parts) > 4 && parts[4] != "NA" && parts[4] != "" {
		info.Thumbnail = strings.TrimSpace(parts[4])
	}

	return info, nil
}

// StartDownload starts a background download job
func (s *VideoService) StartDownload(targetURL, format, quality string) (*DownloadJob, error) {
	normURL := NormalizeURL(targetURL)
	jobID := fmt.Sprintf("%d", time.Now().UnixNano())
	job := &DownloadJob{
		ID:        jobID,
		URL:       normURL,
		Format:    format,
		Quality:   quality,
		Status:    "queued",
		Progress:  "0%",
		CreatedAt: time.Now(),
	}

	s.jobsMu.Lock()
	s.jobs[jobID] = job
	s.jobsMu.Unlock()

	go s.runDownload(job)

	return job, nil
}

func (s *VideoService) runDownload(job *DownloadJob) {
	s.updateJob(job.ID, func(j *DownloadJob) {
		j.Status = "downloading"
		j.Progress = "Iniciando descarga..."
	})

	ytBin := s.getYtDlpBin()
	ffmpegBin := s.getFFmpegBin()
	denoBin := s.getDenoBin()

	// Safe filename template using alphanumeric IDs
	outputTemplate := filepath.Join(s.outDir, fmt.Sprintf("%s_%%(title).60B.%%(ext)s", job.ID))

	args := []string{
		"--no-playlist",
		"--newline",
		"--extractor-args", "youtube:player_client=android,web",
		"--ffmpeg-location", filepath.Dir(ffmpegBin),
		"-o", outputTemplate,
	}

	if denoBin != "" {
		args = append(args, "--js-runtimes", fmt.Sprintf("deno:%s", denoBin))
	}

	if job.Format == "audio" {
		args = append(args, "-x", "--audio-format", "mp3")
	} else {
		// Video formats
		if job.Quality == "1080p" {
			args = append(args, "-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]/best")
		} else if job.Quality == "720p" {
			args = append(args, "-f", "bestvideo[height<=720]+bestaudio/best[height<=720]/best")
		} else {
			args = append(args, "-f", "bestvideo+bestaudio/best")
		}
		args = append(args, "--merge-output-format", "mp4")
	}

	args = append(args, job.URL)

	log.Printf("[Video Download] Iniciando comando: %s %v", ytBin, args)
	cmd := exec.Command(ytBin, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.updateJob(job.ID, func(j *DownloadJob) {
			j.Status = "error"
			j.Error = err.Error()
		})
		return
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		s.updateJob(job.ID, func(j *DownloadJob) {
			j.Status = "error"
			j.Error = err.Error()
		})
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "[download]") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, "%") {
					s.updateJob(job.ID, func(j *DownloadJob) {
						j.Progress = part
					})
					break
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		s.updateJob(job.ID, func(j *DownloadJob) {
			j.Status = "error"
			j.Error = fmt.Sprintf("Error en yt-dlp: %v", err)
		})
		return
	}

	// Locate resulting file
	matches, _ := filepath.Glob(filepath.Join(s.outDir, fmt.Sprintf("%s_*", job.ID)))
	var downloadedFile string
	if len(matches) > 0 {
		downloadedFile = matches[0]
	}

	s.updateJob(job.ID, func(j *DownloadJob) {
		j.Status = "completed"
		j.Progress = "100%"
		j.FilePath = downloadedFile
		if downloadedFile != "" {
			j.Filename = filepath.Base(downloadedFile)
		}
	})
}

func (s *VideoService) updateJob(id string, updateFn func(*DownloadJob)) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	if job, ok := s.jobs[id]; ok {
		updateFn(job)
	}
}

func (s *VideoService) GetJob(id string) *DownloadJob {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	if job, ok := s.jobs[id]; ok {
		copy := *job
		return &copy
	}
	return nil
}

func (s *VideoService) ListJobs() []*DownloadJob {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	list := make([]*DownloadJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		copy := *j
		list = append(list, &copy)
	}
	return list
}

func cleanError(raw string) string {
	re := regexp.MustCompile(`(?i)ERROR:\s*(.+)`)
	match := re.FindStringSubmatch(raw)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return "No se pudo procesar el video"
}
