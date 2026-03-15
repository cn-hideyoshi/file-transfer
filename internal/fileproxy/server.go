package fileproxy

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	rootDir string
}

type fileEntry struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	IsDir        bool      `json:"is_dir"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	SizeText     string    `json:"-"`
}

type directoryPageData struct {
	Title      string
	CurrentURL string
	ParentURL  string
	RootDir    string
	Entries    []fileEntry
}

func NewServer(rootDir string) (*Server, error) {
	return &Server{rootDir: rootDir}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/files/", s.handleFiles)
	return loggingMiddleware(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]string{
			"message":        "local file proxy is running",
			"root_dir":       s.rootDir,
			"browse_api":     "/files/",
			"download_param": "append ?download=1 to force attachment",
			"health_check":   "/healthz",
		})
		return
	}

	http.Redirect(w, r, "/files/", http.StatusFound)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/files")
	if relPath == "" {
		relPath = "/"
	}

	targetPath, err := s.resolvePath(relPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "failed to read target path", http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		s.serveDirectory(w, r, targetPath, relPath)
		return
	}

	s.serveFile(w, r, targetPath, info)
}

func (s *Server) resolvePath(requestPath string) (string, error) {
	cleaned := filepath.Clean("/" + requestPath)
	trimmed := strings.TrimPrefix(cleaned, "/")
	target := filepath.Join(s.rootDir, trimmed)

	rel, err := filepath.Rel(s.rootDir, target)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes configured root")
	}

	return target, nil
}

func (s *Server) serveDirectory(w http.ResponseWriter, r *http.Request, targetPath, relPath string) {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		http.Error(w, "failed to read directory", http.StatusInternalServerError)
		return
	}

	items := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		entryPath := strings.TrimSuffix(relPath, "/") + "/" + entry.Name()
		items = append(items, fileEntry{
			Name:         entry.Name(),
			Path:         "/files" + entryPath,
			IsDir:        entry.IsDir(),
			Size:         info.Size(),
			LastModified: info.ModTime(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})

	currentURL := "/files" + strings.TrimSuffix(relPath, "/") + "/"
	for i := range items {
		items[i].SizeText = formatSize(items[i].Size, items[i].IsDir)
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"path":    currentURL,
			"entries": items,
		})
		return
	}

	parentURL := ""
	if currentURL != "/files/" {
		parentURL = filepath.ToSlash(filepath.Dir(strings.TrimSuffix(currentURL, "/")))
		if parentURL == "." {
			parentURL = "/files/"
		} else {
			parentURL += "/"
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := directoryPage().Execute(w, directoryPageData{
		Title:      "目录浏览",
		CurrentURL: currentURL,
		ParentURL:  parentURL,
		RootDir:    s.rootDir,
		Entries:    items,
	}); err != nil {
		http.Error(w, "failed to render directory page", http.StatusInternalServerError)
	}
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, targetPath string, info os.FileInfo) {
	file, err := os.Open(targetPath)
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if shouldDownload(r) {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
	}
	if contentType := mime.TypeByExtension(filepath.Ext(info.Name())); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func shouldDownload(r *http.Request) bool {
	value := r.URL.Query().Get("download")
	if value == "" {
		return false
	}

	ok, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return ok
}

func wantsJSON(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("format"), "json") {
		return true
	}

	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

func formatSize(size int64, isDir bool) string {
	if isDir {
		return "-"
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}

	return fmt.Sprintf("%.1f PB", value/1024)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write json: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
