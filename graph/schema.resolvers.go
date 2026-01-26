package graph

import (
	"archive/zip"
	"coderefinery/graph/model"
	"coderefinery/internal/core/domain"
	"coderefinery/internal/infrastructure/middleware"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
)

// CreateRepository is the resolver for the createRepository field.
func (r *mutationResolver) CreateRepository(ctx context.Context, name string, path string) (*model.Repository, error) {
	repo, err := r.RepoService.Create(ctx, name, path, false)
	if err != nil {
		return nil, err
	}
	return mapDomainRepoToModel(repo), nil
}

// ReindexRepository is the resolver for the reindexRepository field.
func (r *mutationResolver) ReindexRepository(ctx context.Context, id string) (bool, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return false, fmt.Errorf("invalid uuid: %w", err)
	}

	if err := r.RepoService.Reindex(ctx, uid); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteRepository is the resolver for the deleteRepository field.
func (r *mutationResolver) DeleteRepository(ctx context.Context, id string) (bool, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return false, fmt.Errorf("invalid uuid: %w", err)
	}

	if err := r.RepoService.Delete(ctx, uid); err != nil {
		return false, err
	}
	return true, nil
}

// UploadRepository is the resolver for the uploadRepository field.
func (r *mutationResolver) UploadRepository(ctx context.Context, name string, file graphql.Upload) (*model.Repository, error) {
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".zip") {
		return nil, fmt.Errorf("only .zip files are allowed")
	}

	projectID := uuid.New()
	uploadDir := r.Config.Storage.UploadDir
	targetDir := filepath.Join(uploadDir, projectID.String())

	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// FIX: Temporäre Datei erstellen, da zip einen ReaderAt benötigt
	tempFile, err := os.CreateTemp("", "upload-*.zip")
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name()) // Aufräumen
	defer tempFile.Close()

	// Inhalt in TempFile kopieren
	if _, err := io.Copy(tempFile, file.File); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	// Entpacken mit der temporären Datei (die ReaderAt implementiert)
	if err := unzipFile(tempFile, file.Size, targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, fmt.Errorf("failed to unzip: %w", err)
	}

	repo, err := r.RepoService.Create(ctx, name, targetDir, true)
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}

	return mapDomainRepoToModel(repo), nil
}

// Register is the resolver for the register field.
func (r *mutationResolver) Register(ctx context.Context, username string, password string) (*model.AuthPayload, error) {
	_, err := r.AuthService.Register(ctx, username, password)
	if err != nil {
		return nil, err
	}

	token, user, err := r.AuthService.Login(ctx, username, password)
	if err != nil {
		return nil, fmt.Errorf("login after registration failed: %w", err)
	}

	return &model.AuthPayload{
		Token: token,
		User:  mapDomainUserToModel(user),
	}, nil
}

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, username string, password string) (*model.AuthPayload, error) {
	token, user, err := r.AuthService.Login(ctx, username, password)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return &model.AuthPayload{
		Token: token,
		User:  mapDomainUserToModel(user),
	}, nil
}

// SetEmbeddingModel is the resolver for the setEmbeddingModel field.
func (r *mutationResolver) SetEmbeddingModel(ctx context.Context, model string) (bool, error) {
	available, err := r.Embedder.ListModels(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list models: %w", err)
	}

	found := false
	for _, m := range available {
		if m == model {
			found = true
			break
		}
	}
	if !found {
		return false, fmt.Errorf("model %s not available", model)
	}

	if r.Embedder.GetCurrentModel() == model {
		return true, nil
	}

	if err := r.Embedder.SetModel(model); err != nil {
		return false, err
	}

	if err := r.RepoService.DeleteAllIndices(ctx); err != nil {
		return false, fmt.Errorf("failed to clear indices: %w", err)
	}

	r.Config.LLM.EmbeddingModel = model

	repos, err := r.RepoService.List(ctx)
	if err != nil {
		return false, err
	}

	for _, repo := range repos {
		go func(id uuid.UUID) {
			_ = r.RepoService.Reindex(context.Background(), id)
		}(repo.ID)
	}

	return true, nil
}

// Repositories is the resolver for the repositories field.
func (r *queryResolver) Repositories(ctx context.Context) ([]*model.Repository, error) {
	repos, err := r.RepoService.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []*model.Repository
	for _, repo := range repos {
		result = append(result, mapDomainRepoToModel(repo))
	}
	return result, nil
}

// Search is the resolver for the search field.
func (r *queryResolver) Search(ctx context.Context, query string, limit *int32) ([]*model.SearchResult, error) {
	l := 10
	if limit != nil {
		l = int(*limit)
	}

	req := domain.SearchRequest{
		Query:    query,
		Limit:    l,
		MinScore: 0.25,
	}

	hits, err := r.Searcher.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	var result []*model.SearchResult
	for _, h := range hits {
		sig := h.Chunk.Signature

		result = append(result, &model.SearchResult{
			FilePath:  h.Chunk.FilePath,
			StartLine: safeInt32(h.Chunk.StartLine),
			EndLine:   safeInt32(h.Chunk.EndLine),
			Content:   h.Chunk.Content,
			Score:     h.CombinedScore,
			Signature: &sig,
		})
	}
	return result, nil
}

// Me is the resolver for the me field.
func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	userID, err := middleware.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, nil
	}

	user, err := r.AuthService.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mapDomainUserToModel(user), nil
}

// LlmInfo is the resolver for the llmInfo field.
func (r *queryResolver) LlmInfo(ctx context.Context) (*model.LLMInfo, error) {
	available, err := r.Embedder.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	return &model.LLMInfo{
		CurrentModel:    r.Embedder.GetCurrentModel(),
		AvailableModels: available,
	}, nil
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

const resolverMaxUnzipSize = 100 * 1024 * 1024

// FIX: Signatur geändert auf io.ReaderAt (statt io.Reader)
func unzipFile(r io.ReaderAt, size int64, dest string) error {
	dest = filepath.Clean(dest)

	// Direkt r nutzen, da es jetzt ReaderAt ist
	reader, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	for _, f := range reader.File {
		//nolint:gosec // G305: Path traversal is checked explicitly in the next line
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, dest+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0750); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0750); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		written, err := io.CopyN(outFile, rc, resolverMaxUnzipSize)
		if written >= resolverMaxUnzipSize {
			_ = outFile.Close()
			_ = rc.Close()
			return fmt.Errorf("zip file too large")
		}
		if err != nil && err != io.EOF {
			_ = outFile.Close()
			_ = rc.Close()
			return err
		}

		if err := outFile.Close(); err != nil {
			_ = rc.Close()
			return err
		}
		_ = rc.Close()
	}
	return nil
}

func mapDomainRepoToModel(d *domain.Repository) *model.Repository {
	return &model.Repository{
		ID:          d.ID.String(),
		Name:        d.Name,
		Path:        d.Path,
		Status:      string(d.Status),
		LastIndexed: nil,
		FileCount:   safeInt32(d.FileCount),
		ChunkCount:  safeInt32(d.ChunkCount),
		ErrorMsg:    &d.ErrorMsg,
	}
}

func mapDomainUserToModel(u *domain.User) *model.User {
	return &model.User{
		ID:       u.ID.String(),
		Username: u.Username,
		Role:     string(u.Role),
	}
}

func safeInt32(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}
