package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"coderefinery/internal/core/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	repoService *services.RepositoryService
	uploadDir   string
}

func NewUploadHandler(service *services.RepositoryService, uploadDir string) *UploadHandler {
    // Verzeichnis sicherstellen
    if err := os.MkdirAll(uploadDir, 0755); err != nil {
        // Panic ist hier okay beim Startup, oder Error loggen
        fmt.Printf("Warning: Could not create upload dir: %v\n", err)
    }
	return &UploadHandler{
		repoService: service,
		uploadDir:   uploadDir,
	}
}

// HandleUpload verarbeitet den POST Request
func (h *UploadHandler) HandleUpload(c *gin.Context) {
	// 1. Multipart Form parsen (Max 50MB)
    // Gin macht das oft automatisch, aber wir setzen hier explizit Grenzen via Config im Main
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded or file too big"})
		return
	}
    defer file.Close()

    // Validierung: Muss ZIP sein
    if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Only .zip files are allowed"})
        return
    }

	// 2. Zielpfad vorbereiten
	projectID := uuid.New()
	targetDir := filepath.Join(h.uploadDir, projectID.String())

	// 3. Entpacken
	if err := unzip(file, header.Size, targetDir); err != nil {
        // Aufräumen falls halb entpackt
        os.RemoveAll(targetDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unzip: " + err.Error()})
		return
	}

	// 4. Repo erstellen
    // Name aus Dateiname ableiten (projekt.zip -> projekt)
	projectName := c.PostForm("name")
    if projectName == "" {
        projectName = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
    }

    // Aufruf mit isManaged = true
	repo, err := h.repoService.Create(c.Request.Context(), projectName, targetDir, true)
	if err != nil {
		os.RemoveAll(targetDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// unzip entpackt sicher und verhindert Zip Slip Vulnerability
func unzip(r io.ReaderAt, size int64, dest string) error {
	reader, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	for _, f := range reader.File {
		// Konstruiere den vollen Pfad
		fpath := filepath.Join(dest, f.Name)

		// ZIP SLIP PROTECTION: Prüfen, ob der Pfad noch innerhalb von dest liegt
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		// Limitieren der Dateigröße beim Entpacken (optional, gegen Zip Bombs)
		_, err = io.Copy(outFile, rc) // Hier könnte man io.CopyN nutzen

        outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
