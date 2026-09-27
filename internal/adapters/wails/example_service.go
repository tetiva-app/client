package wails

import (
	"context"

	"github.com/google/uuid"

	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/domain"
	"github.com/tetiva-app/client/internal/domain/entities"
	"github.com/tetiva-app/client/internal/domain/usecase/example"
)

// ExampleService exposes response examples to the Wails frontend.
type ExampleService struct {
	uc example.Usecase
}

func NewExampleService(uc example.Usecase) *ExampleService {
	return &ExampleService{uc: uc}
}

func (s *ExampleService) List(requestID string) Result[[]dto.ExampleResponse] {
	ctx := context.Background()

	id, err := uuid.Parse(requestID)
	if err != nil {
		return Err[[]dto.ExampleResponse](&domain.ValidationError{
			Fields: map[string]string{"requestId": "invalid UUID"},
		})
	}

	examples, err := s.uc.ListByRequest(ctx, id)
	if err != nil {
		return Err[[]dto.ExampleResponse](err)
	}

	return OK(dto.ExamplesToResponse(examples))
}

func (s *ExampleService) Create(req dto.CreateExampleRequest) Result[dto.ExampleResponse] {
	ctx := context.Background()

	requestID, err := uuid.Parse(req.RequestID)
	if err != nil {
		return Err[dto.ExampleResponse](&domain.ValidationError{
			Fields: map[string]string{"requestId": "invalid UUID"},
		})
	}

	input := example.Create{
		RequestID:   requestID,
		Name:        req.Name,
		StatusCode:  req.StatusCode,
		StatusText:  req.StatusText,
		Headers:     dto.HeaderItemsToEntity(req.Headers),
		Body:        req.Body,
		ContentType: req.ContentType,
		Protocol:    entities.Protocol(req.Protocol),
	}
	opt := example.CreateOpt{UserID: defaultUserID}

	e, err := s.uc.Create(ctx, input, opt)
	if err != nil {
		return Err[dto.ExampleResponse](err)
	}

	return OK(dto.ExampleToResponse(e))
}

func (s *ExampleService) Edit(req dto.EditExampleRequest) Result[dto.ExampleResponse] {
	ctx := context.Background()

	id, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[dto.ExampleResponse](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	input := example.Edit{
		Name:        req.Name,
		StatusCode:  req.StatusCode,
		StatusText:  req.StatusText,
		Headers:     dto.HeaderItemsToEntity(req.Headers),
		Body:        req.Body,
		ContentType: req.ContentType,
	}
	opt := example.EditOpt{
		ExampleID: id,
		UserID:    defaultUserID,
		Version:   req.Version,
	}

	e, err := s.uc.Edit(ctx, input, opt)
	if err != nil {
		return Err[dto.ExampleResponse](err)
	}

	return OK(dto.ExampleToResponse(e))
}

func (s *ExampleService) Delete(req dto.DeleteExampleRequest) Result[Empty] {
	ctx := context.Background()

	id, err := uuid.Parse(req.ID)
	if err != nil {
		return Err[Empty](&domain.ValidationError{
			Fields: map[string]string{"id": "invalid UUID"},
		})
	}

	opt := example.DeleteOpt{
		ExampleID: id,
		UserID:    defaultUserID,
		Version:   req.Version,
	}

	if err := s.uc.Delete(ctx, opt); err != nil {
		return Err[Empty](err)
	}

	return OK(Empty{})
}

func (s *ExampleService) ScanSecrets(req dto.ScanExampleRequest) Result[[]string] {
	return OK(example.SuspectedSecrets(dto.HeaderItemsToEntity(req.Headers), req.Body))
}
