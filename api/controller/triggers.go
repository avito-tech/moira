package controller

import (
	"fmt"
	"strings"

	"github.com/satori/go.uuid"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/database"
)

// CreateTrigger creates new trigger
func CreateTrigger(
	dataBase moira.Database,
	triggerInheritanceDatabase moira.TriggerInheritanceDatabase,
	trigger *dto.TriggerModel,
	timeSeriesNames map[string]bool,
) (*dto.SaveTriggerResponse, *api.ErrorResponse) {
	if trigger.ID == "" {
		trigger.ID = uuid.NewV4().String()
	} else {
		exists, err := isTriggerExists(dataBase, trigger.ID)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}
		if exists {
			return nil, api.ErrorInvalidRequest(fmt.Errorf("Trigger with this ID already exists"))
		}
	}
	resp, err := saveTrigger(dataBase, triggerInheritanceDatabase, trigger.ToMoiraTrigger(), trigger.ID, timeSeriesNames)
	if resp != nil {
		resp.Message = "trigger created"
	}
	return resp, err
}

func isTriggerExists(dataBase moira.Database, triggerID string) (bool, error) {
	_, err := dataBase.GetTrigger(triggerID)
	if err == database.ErrNil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetAllTriggers gets all moira triggers
func GetAllTriggers(database moira.Database) (*dto.TriggersList, *api.ErrorResponse) {
	triggerIDs, err := database.GetTriggerIDs(false)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	triggerChecks, err := database.GetTriggerChecks(triggerIDs)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}
	triggersList := dto.TriggersList{
		List: make([]moira.TriggerCheck, 0),
	}
	for _, triggerCheck := range triggerChecks {
		if triggerCheck != nil {
			triggersList.List = append(triggersList.List, *triggerCheck)
		}
	}
	return &triggersList, nil
}

// GetTriggerPage gets trigger page and filter trigger by tags, errors and name
func GetTriggerPage(database moira.Database, page int64, size int64, onlyErrors bool, filterTags []string, filterName string) (*dto.TriggersList, *api.ErrorResponse) {
	filterName = strings.ToUpper(filterName)
	triggerIDs, err := database.GetTriggerCheckIDs(filterTags, onlyErrors)
	if err != nil {
		return nil, api.ErrorInternalServer(err)
	}

	var total, takeFrom, takeTo int64
	var triggerChecksFiltered []*moira.TriggerCheck
	if filterName != "" {
		// we need to filter triggers by name, so we take all (!) the triggers with given ids
		// then apply filter and only after all apply page constraints
		triggerChecks, err := database.GetTriggerChecks(triggerIDs)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}

		triggerChecksFiltered = make([]*moira.TriggerCheck, 0, len(triggerChecks))
		for _, triggerCheck := range triggerChecks {
			if triggerCheck != nil && strings.Contains(strings.ToUpper(triggerCheck.Name), filterName) {
				triggerChecksFiltered = append(triggerChecksFiltered, triggerCheck)
			}
		}

		total = int64(len(triggerChecksFiltered))
		takeFrom, takeTo = calculateRange(total, page, size)
		triggerChecksFiltered = triggerChecksFiltered[takeFrom:takeTo]
	} else {
		// we don't need to filter by name, so we can apply page constraints at first
		// and then take only those triggers which are left
		total = int64(len(triggerIDs))
		takeFrom, takeTo = calculateRange(total, page, size)

		triggerIDs = triggerIDs[takeFrom:takeTo]
		triggerChecksFiltered, err = database.GetTriggerChecks(triggerIDs)
		if err != nil {
			return nil, api.ErrorInternalServer(err)
		}
	}

	triggers := dto.TriggersList{
		List:  make([]moira.TriggerCheck, 0, len(triggerChecksFiltered)),
		Total: &total,
		Page:  &page,
		Size:  &size,
	}

	for _, triggerCheck := range triggerChecksFiltered {
		if triggerCheck != nil {
			triggers.List = append(triggers.List, *triggerCheck)
		}
	}

	return &triggers, nil
}

func calculateRange(total, page, size int64) (int64, int64) {
	from := page * size
	to := (page + 1) * size

	if from > total {
		from = total
	}

	if to > total {
		to = total
	}

	return from, to
}

func getTriggerIdsRange(triggerIDs []string, total int64, page int64, size int64) []string {
	from := page * size
	to := (page + 1) * size

	if from > total {
		from = total
	}

	if to > total {
		to = total
	}

	return triggerIDs[from:to]
}
