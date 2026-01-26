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
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("Warning: Could not create upload dir: %v\n", err)
	}
	return &UploadHandler{
		repoService: service,
		uploadDir:   uploadDir,
	}
}

func (h *UploadHandler) HandleUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded or file too big"})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .zip files are allowed"})
		return
	}

	projectID := uuid.New()
	targetDir := filepath.Join(h.uploadDir, projectID.String())

	tempFile, err := os.CreateTemp("", "upload-*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save temp file"})
		return
	}

	if err := unzip(tempFile, header.Size, targetDir); err != nil {
		os.RemoveAll(targetDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unzip: " + err.Error()})
		return
	}

	projectName := c.PostForm("name")
	if projectName == "" {
		projectName = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}

	repo, err := h.repoService.Create(c.Request.Context(), projectName, targetDir, true)
	if err != nil {
		os.RemoveAll(targetDir)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

func unzip(r io.ReaderAt, size int64, dest string) error {
	reader, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	for _, f := range reader.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			// Error check hinzugefügt
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
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

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
