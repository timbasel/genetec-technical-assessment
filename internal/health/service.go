package health

import (
	"context"
	"errors"
	"log"
)

type Store interface {
	PingContext(context.Context) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) GetHealth(ctx context.Context) error {
	if s.store == nil {
		return errors.New("data store not initialized")
	}
	if err := s.store.PingContext(ctx); err != nil {
		log.Printf("data store unavailable: %s", err.Error())
		return errors.New("data store unavailable")
	}
	return nil
}
