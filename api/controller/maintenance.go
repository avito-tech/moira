package controller

import (
	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
)

func GetMaintenanceSilent(database moira.Database, spt moira.SilentPatternType) (dto.Maintenance, *api.ErrorResponse) {
	maintenance, err := database.GetMaintenanceSilent(spt)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	return dto.Maintenance(maintenance), nil
}

func GetMaintenanceTrigger(database moira.Database, id string) (dto.Maintenance, *api.ErrorResponse) {
	maintenance, err := database.GetMaintenanceTrigger(id)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	return dto.Maintenance(maintenance), nil
}
