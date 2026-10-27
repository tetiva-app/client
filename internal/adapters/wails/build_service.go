package wails

import (
	"github.com/tetiva-app/client/internal/adapters/wails/dto"
	"github.com/tetiva-app/client/internal/constants"
)

type BuildService struct{}

func NewBuildService() *BuildService {
	return &BuildService{}
}

func (s *BuildService) Info() Result[dto.BuildInfo] {
	flavor := constants.FlavorStable
	if constants.IsPreview() {
		flavor = constants.FlavorPreview
	}
	return OK(dto.BuildInfo{Flavor: flavor})
}
