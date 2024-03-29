package moira

var eventStateWeight = map[string]int{
	OK:     0,
	WARN:   1,
	ERROR:  100,
	NODATA: 10000,
}

// EventHighDegradationTag is reserved tag that describes High Degradation
var EventHighDegradationTag = "HIGH DEGRADATION"

// EventDegradationTag is reserved tag that describes Degradation
var EventDegradationTag = "DEGRADATION"

// EventProgressTag is reserved tag that describes Progress
var EventProgressTag = "PROGRESS"

// GetEventTags returns additional subscription tags based on trigger state
func (event *NotificationEvent) GetEventTags() []string {
	tags := []string{event.State, event.OldState}
	if oldStateWeight, ok := eventStateWeight[event.OldState]; ok {
		if newStateWeight, ok := eventStateWeight[event.State]; ok {
			if newStateWeight > oldStateWeight {
				if newStateWeight-oldStateWeight >= 100 {
					tags = append(tags, EventHighDegradationTag)
				}
				tags = append(tags, EventDegradationTag)
			}
			if newStateWeight < oldStateWeight {
				tags = append(tags, EventProgressTag)
			}
		}
	}
	return tags
}
