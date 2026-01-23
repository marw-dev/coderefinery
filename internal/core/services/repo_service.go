package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"coderefinery/internal/core/domain"
	"coderefinery/internal/core/ports"
	"coderefinery/pkg/errors"

	"github.com/google/uuid"
)

type RepositoryService struct {
	store   ports.RepositoryStore
	indexer ports.Indexer
}

// NewRepositoryService erstellt eine neue Instanz
func NewRepositoryService(store ports.RepositoryStore, indexer ports.Indexer) *RepositoryService {
	return &RepositoryService{
		store:   store,
		indexer: indexer,
	}
}

// Create validiert den Pfad, erstellt den DB-Eintrag und startet das Indexing
func (s *RepositoryService) Create(ctx context.Context, name, path string) (*domain.Repository, error) {
	// 1. Validierung
	if name == "" {
		return nil, errors.New(errors.ErrCodeValidation, "repository name is required")
	}
	if path == "" {
		return nil, errors.New(errors.ErrCodeValidation, "repository path is required")
	}

	// Pfad prüfen
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, errors.Wrap(err, errors.ErrCodeValidation, "path does not exist on server")
	}

	// 2. Entity erstellen
	repo := domain.NewRepository(name, path)

	// 3. Speichern
	if err := s.store.Save(ctx, repo); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to persist repository")
	}

	// 4. Indexing starten
	go s.runIndexing(repo)

	return repo, nil
}

func (s *RepositoryService) Get(ctx context.Context, id uuid.UUID) (*domain.Repository, error) {
	repo, err := s.store.FindByID(ctx, id)
	if err != nil {
		// Hier könnten wir prüfen, ob es ein sql.ErrNoRows ist
		return nil, errors.Wrap(err, errors.ErrCodeNotFound, "repository not found")
	}
	return repo, nil
}

func (s *RepositoryService) List(ctx context.Context) ([]*domain.Repository, error) {
	return s.store.FindAll(ctx)
}

func (s *RepositoryService) Delete(ctx context.Context, id uuid.UUID) error {
	repo, err := s.store.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Erst Index löschen (Vektoren bereinigen)
	if err := s.indexer.DeleteIndex(ctx, repo); err != nil {
		// Wir loggen nur, brechen aber nicht ab, damit der DB-Eintrag trotzdem gelöscht werden kann
		fmt.Printf("Warning: Failed to delete index for repo %s: %v\n", id, err)
	}

	// Dann aus DB entfernen
	return s.store.Delete(ctx, id)
}

func (s *RepositoryService) Reindex(ctx context.Context, id uuid.UUID) error {
	repo, err := s.store.FindByID(ctx, id)
	if err != nil {
		return err
	}

	go s.runIndexing(repo)
	return nil
}

// Interne Hilfsmethode für den Hintergrund-Prozess
func (s *RepositoryService) runIndexing(repo *domain.Repository) {
	// Eigener Context für den Hintergrund-Prozess (damit er nicht abbricht, wenn der HTTP-Request endet)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

	// Status Update: Indexing
	repo.Status = domain.StatusIndexing
	repo.UpdatedAt = time.Now()
	_ = s.store.Save(ctx, repo)

	// Indexer aufrufen
	err := s.indexer.Index(ctx, repo)

	repo.UpdatedAt = time.Now()
	if err != nil {
		repo.Status = domain.StatusError
		repo.ErrorMsg = err.Error()
	} else {
		repo.Status = domain.StatusReady
		repo.LastIndexed = time.Now()
		repo.ErrorMsg = ""
	}

	// Finalen Status speichern
	_ = s.store.Save(ctx, repo)
}

func (s *RepositoryService) DeleteAllIndices(ctx context.Context) error {
	// Delegiert an den Indexer, um alle Vektoren zu löschen
	return s.indexer.DeleteAllIndices(ctx)
}
