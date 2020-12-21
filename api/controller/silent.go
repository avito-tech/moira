package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/api"
	"go.avito.ru/DO/moira/api/dto"
	"go.avito.ru/DO/moira/netbox"
)

const prefixDC = "dc_"
const prefixRack = "rack_"

type range_ struct {
	start int
	end   int
}

type SilentPatternManager struct {
	config api.Config
}

func CreateSilentPatternManager(config api.Config) *SilentPatternManager {
	return &SilentPatternManager{
		config: config,
	}
}

func (spm *SilentPatternManager) GetSilentPatterns(database moira.Database, patternType moira.SilentPatternType) (*dto.SilentPatternList, error) {
	silentPatterns, err := database.GetSilentPatternsTyped(patternType)
	if err != nil {
		return nil, err
	}

	spList := dto.SilentPatternList{
		List: silentPatterns,
	}

	return &spList, nil
}

func (spm *SilentPatternManager) CreateSilentPatterns(dataBase moira.Database, rawSilentPatterns *dto.SilentPatternList, login string) error {
	client := netbox.CreateClient(spm.config.Netbox)
	errorMessages := make([]string, 0, 100)
	now := time.Now().Unix()
	silentPatterns := make(map[moira.SilentPatternType][]*moira.SilentPatternData)

	silentPatterns[moira.SPTMetric] = make([]*moira.SilentPatternData, 0, 100)
	silentPatterns[moira.SPTTag] = make([]*moira.SilentPatternData, 0, 100)

	for _, rawSilentPattern := range rawSilentPatterns.List {
		parsedPatterns, err := parsePatternString(rawSilentPattern.Pattern, client)
		if err != nil {
			errorMessages = append(errorMessages, err.Error())
			continue
		}

		patternType := rawSilentPattern.Type
		for _, parsedPattern := range parsedPatterns {
			silentPatterns[patternType] = append(silentPatterns[patternType], &moira.SilentPatternData{
				Login:   login,
				Pattern: parsedPattern,
				Created: now,
				Until:   rawSilentPattern.Until,
				Type:    patternType,
			})
		}
	}

	if err := joinErrorMessages(errorMessages); err != nil {
		return err
	}

	for spt, spl := range silentPatterns {
		if len(spl) == 0 {
			continue
		}

		if err := dataBase.LockSilentPatterns(spt); err != nil {
			return err
		}
		if err := dataBase.SaveSilentPatterns(spt, spl...); err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
		_ = dataBase.UnlockSilentPatterns(spt)
	}

	return joinErrorMessages(errorMessages)
}

func (spm *SilentPatternManager) UpdateSilentPatterns(dataBase moira.Database, rawSilentPatterns *dto.SilentPatternList, login string) error {
	errorMessages := make([]string, 0, 100)
	now := time.Now().Unix()
	silentPatterns := make(map[moira.SilentPatternType][]*moira.SilentPatternData)

	silentPatterns[moira.SPTMetric] = make([]*moira.SilentPatternData, 0, 100)
	silentPatterns[moira.SPTTag] = make([]*moira.SilentPatternData, 0, 100)

	for _, rsp := range rawSilentPatterns.List {
		rsp.Created = now
		rsp.Login = login
		silentPatterns[rsp.Type] = append(silentPatterns[rsp.Type], rsp)
	}

	for spt, spl := range silentPatterns {
		if len(spl) == 0 {
			continue
		}

		if err := dataBase.LockSilentPatterns(spt); err != nil {
			return err
		}
		if err := dataBase.SaveSilentPatterns(spt, spl...); err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
		_ = dataBase.UnlockSilentPatterns(spt)
	}

	return joinErrorMessages(errorMessages)
}

func (spm *SilentPatternManager) RemoveSilentPatterns(dataBase moira.Database, rawSilentPatterns *dto.SilentPatternList) error {
	errorMessages := make([]string, 0, 100)
	silentPatterns := make(map[moira.SilentPatternType][]*moira.SilentPatternData)

	silentPatterns[moira.SPTMetric] = make([]*moira.SilentPatternData, 0, 100)
	silentPatterns[moira.SPTTag] = make([]*moira.SilentPatternData, 0, 100)

	for _, rsp := range rawSilentPatterns.List {
		if rsp.ID == "" {
			errorMessages = append(errorMessages, "Missing silent pattern id")
			continue
		}
		silentPatterns[rsp.Type] = append(silentPatterns[rsp.Type], rsp)
	}

	if err := joinErrorMessages(errorMessages); err != nil {
		return err
	}

	for spt, spl := range silentPatterns {
		if len(spl) == 0 {
			continue
		}

		if err := dataBase.LockSilentPatterns(spt); err != nil {
			return err
		}
		if err := dataBase.RemoveSilentPatterns(spt, spl...); err != nil {
			errorMessages = append(errorMessages, err.Error())
		}
		_ = dataBase.UnlockSilentPatterns(spt)
	}

	return joinErrorMessages(errorMessages)
}

func joinErrorMessages(messages []string) error {
	if len(messages) > 0 {
		return fmt.Errorf(strings.Join(messages, "\n"))
	}
	return nil
}

func parsePatternString(pattern string, client *netbox.Client) (result []string, err error) {
	var parser = &patternParser{
		client:  client,
		pattern: pattern,
	}

	defer func() {
		// recovering from (possible) panic
		if r := recover(); r != nil {
			if r1, ok := r.(error); ok {
				err = r1
			} else {
				// propagating panic if it doesn't contain an error
				panic(r)
			}
		}
	}()

	if strings.Contains(pattern, "[") {
		return parser.expandPatternsRange(), nil
	} else if strings.HasPrefix(pattern, prefixDC) {
		return parser.expandRackGroup(), nil
	} else if strings.HasPrefix(pattern, prefixRack) {
		return parser.expandRack(), nil
	} else {
		return []string{pattern}, nil
	}
}

type patternParser struct {
	client  *netbox.Client
	pattern string
}

// expandPatternsRange expands patterns range given as string like "avi-rabbitmq0[1-5]"
// to the slice of strings containing single pattern each
func (parser *patternParser) expandPatternsRange() []string {
	var (
		err                  error
		rangeStart, rangeEnd int
		result               []string
	)

	arrStartIndex := strings.Index(parser.pattern, "[")
	arrEndIndex := strings.Index(parser.pattern, "]")

	serverName := parser.pattern[:arrStartIndex]
	rangeSpec := parser.pattern[arrStartIndex+1 : arrEndIndex]

	ranges := strings.Split(rangeSpec, ",")
	intRanges := make([]range_, len(ranges))

	for i := range ranges {
		rangeParts := strings.Split(ranges[i], "-")

		if len(rangeParts) == 1 {
			if rangeStart, err = strconv.Atoi(rangeParts[0]); err != nil {
				panic(err)
			} else {
				intRanges[i] = range_{
					start: rangeStart,
					end:   -1,
				}
			}
		} else if len(rangeParts) == 2 {
			if rangeStart, err = strconv.Atoi(rangeParts[0]); err != nil {
				panic(err)
			} else if rangeEnd, err = strconv.Atoi(rangeParts[1]); err != nil {
				panic(err)
			} else {
				intRanges[i] = range_{
					start: rangeStart,
					end:   rangeEnd,
				}
			}
		}
	}

	for _, r := range intRanges {
		if r.end > 0 {
			for i := r.start; i <= r.end; i++ {
				result = append(result, serverName+strconv.Itoa(i))
			}
		} else {
			result = append(result, serverName+strconv.Itoa(r.start))
		}
	}

	return result
}

// expandRack expands single rack given as string
// to the slice of strings containing single pattern each
func (parser *patternParser) expandRack() []string {
	if parser.client == nil {
		panic(errors.New("Netbox client must be enabled and set"))
	}

	pattern := parser.pattern[len(prefixRack):]
	racks := parser.client.RackList(&pattern)

	// there should be exactly one rack with this name
	if len(racks.List) != 1 {
		panic(errors.New(fmt.Sprintf("Could not find rack with name \"%s\" (%d found)", pattern, len(racks.List))))
	}

	// obtaining device list for this rack
	devices := parser.client.DeviceList(nil, &racks.List[0].Id)
	return parser.saturateDeviceList(devices)
}

// expandRackGroup expands single rack group given as string
// to the slice of strings containing single pattern each
func (parser *patternParser) expandRackGroup() []string {
	if parser.client == nil {
		panic(errors.New("Netbox client must be enabled and set"))
	}

	pattern := parser.pattern[len(prefixDC):]
	rackGroups := parser.client.RackGroupList(&pattern)

	// there should be exactly one rack group with this name
	if len(rackGroups.List) != 1 {
		panic(errors.New(fmt.Sprintf("Could not find rack group with name \"%s\" (%d found)", pattern, len(rackGroups.List))))
	}

	// obtaining device list for this rack group
	devices := parser.client.DeviceList(&rackGroups.List[0].Id, nil)
	return parser.saturateDeviceList(devices)
}

// pickFromDeviceList processes device list and returns only ids and names from it
func (parser *patternParser) pickFromDeviceList(devices netbox.DeviceList) (ids []int, names []string) {
	ids = make([]int, 0, len(devices.List))
	names = make([]string, 0, len(devices.List))
	qty := 0

	// will process only devices with this roles
	considerableRoles := map[string]bool{
		"server": true,
		"switch": true,
	}

	for _, device := range devices.List {
		if device.Role == nil || !considerableRoles[device.Role.Slug] {
			continue
		}

		ids = append(ids, device.Id)
		names = append(names, device.Name)
		qty++
	}

	if qty == 0 {
		// no devices found - it is not exactly an error but we make it that way
		// so that user can notice he's adding an empty list
		panic(errors.New(fmt.Sprintf("0 devices will be added for pattern \"%s\"", parser.pattern)))
	}

	return
}

// saturateDeviceList requests nested containers from device list and
// returns flattened list of all names (both containers' and devices' names)
func (parser *patternParser) saturateDeviceList(devices netbox.DeviceList) []string {
	ids, names := parser.pickFromDeviceList(devices)
	containers := parser.client.ContainerList(ids)

	totalQty := len(names) + len(containers.List)
	result := make([]string, 0, totalQty)
	usedNames := make(map[string]bool, totalQty) // avoid duplicated names in result

	// devices' names
	for _, name := range names {
		if !usedNames[name] {
			result = append(result, name)
			usedNames[name] = true
		}
	}

	// containers' names
	for _, container := range containers.List {
		if !usedNames[container.Name] {
			result = append(result, container.Name)
			usedNames[container.Name] = true
		}
	}

	return result
}
