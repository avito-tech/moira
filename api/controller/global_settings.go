package controller

import (
	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
)

func GetGlobalSettings(database moira.Database) (*dto.GlobalSettings, *api.ErrorResponse) {
	if globalSettings, err := database.GetGlobalSettings(); err != nil {
		return nil, api.ErrorInternalServer(err)
	} else {
		globalSettingsDto := dto.GlobalSettings(globalSettings)
		return &globalSettingsDto, nil
	}
}

func SetGlobalSettings(database moira.Database, newSettingsDto *dto.GlobalSettings) *api.ErrorResponse {
	newSettings := moira.GlobalSettings(*newSettingsDto)
	if err := database.SetGlobalSettings(newSettings); err != nil {
		return api.ErrorInternalServer(err)
	} else {
		return nil
	}
}
