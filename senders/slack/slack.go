package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database/redis"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/notifier/grafana"
)

// Sender implements moira sender interface via slack
type Sender struct {
	APIToken      string
	DataBase      moira.Database
	FrontURI      string
	location      *time.Location
	webdav        *grafana.WebdavUploader
	ackWithButton bool
}

const (
	colorGood            = "good"
	colorDanger          = "danger"
	ackCallbackID        = "trigger_escalation_ack:"
	messenger            = "slack"
	maxMetricsPerMessage = 10
)

var baseParams = slack.PostMessageParameters{
	Username: "Moira",
	Markdown: true,
}

var threadTTL float64

func init() {
	threadTTL = redis.ThreadTTL.Seconds()
}

// Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, location *time.Location) error {
	logger := logging.GetLogger("")

	sender.APIToken = senderSettings["api_token"]
	if sender.APIToken == "" {
		return fmt.Errorf("Can not read slack api_token from config")
	}

	sender.FrontURI = senderSettings["front_uri"]
	sender.location = location

	if senderSettings["ack_with_button"] == "true" {
		sender.ackWithButton = true
		logger.Info("Ack with button is enabled")
	}

	webdavURL := senderSettings["webdav_url"]
	if webdavURL == "" {
		logger.Info("Webdav is NOT configured")
	} else {
		webdavUser := senderSettings["webdav_user"]
		webdavPassword := senderSettings["webdav_password"]
		webdavPublicURL := senderSettings["webdav_public_url"]
		sender.webdav = grafana.NewWebdavImageUploader(webdavURL, webdavUser, webdavPassword, webdavPublicURL)
		logger.Info("Webdav is configured")
	}

	delayedActionWorker := DelayedActionWorker{
		APIToken: sender.APIToken,
		DataBase: sender.DataBase,
		logger:   logger,
	}
	go delayedActionWorker.Start()

	userGroupsWorker := NewUserGroupsWorker(sender.APIToken, sender.DataBase, logger)
	go userGroupsWorker.Start()

	return nil
}

// SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData, throttled, needAck bool) error {
	logger := logging.GetLogger(trigger.ID)
	api := NewSlack(sender.APIToken, logger)

	state := events.GetSubjectState()
	msgOptions := sender.formatStartingMessage(events, &trigger, state, needAck, throttled)

	var (
		err       error
		channelId string
		messageTs string
	)

	// all that it is necessary for compact OKs is to update thread
	isCompactOK := isCompactOKStyleEnabled(&trigger) && state == moira.OK
	if isCompactOK {
		logger.InfoF("Sending compact OK for user %s", contact.Value)
		channelId, err = sender.DataBase.GetIDByUsername(messenger, contact.Value)
		if err == nil {
			return sender.updateDashboards(events, &contact, &trigger, messageTs, channelId, state, needAck, throttled)
		}

		// but if update failed then it will be sent as usual message
		logger.WarnF("Failed to get Slack channel ID from username [%s]: %v", contact.Value, err)
	}

	// in all other cases - send full message
	channelId, messageTs, err = api.PostMessage(contact.Value, msgOptions...)
	if err != nil {
		switch err.Error() {
		case "not_in_channel":
			logger.InfoF("Not in channel %s, trying to join...", contact.Value)
			if _, err := api.findAndJoinChannel(contact.Value); err != nil {
				return fmt.Errorf("Failed to join Slack channel [%s]: %s", contact.Value, err.Error())
			}

			// return error anyway, message will be delivered next time
			return fmt.Errorf("Successfully joined Slack channel [%s], retry next time...", contact.Value)
		default:
			return fmt.Errorf("Failed to send message to slack [%s]: %s", contact.Value, err.Error())
		}
	}

	// cache channel and thread data
	if needAck {
		threadLink := &moira.SlackThreadLink{Contact: channelId, ThreadTs: messageTs}
		for _, event := range events {
			if event.State != moira.OK {
				_ = sender.DataBase.AddUnacknowledgedMessage(trigger.ID, event.Metric, threadLink)
			}
		}
	}
	_ = sender.DataBase.SetUsernameID(messenger, contact.Value, channelId)

	// if the message is compact OK and it has been sent as usual message - than it's all that can be done here
	if isCompactOK {
		return nil
	}

	err = sender.startNewThread(events, &contact, &trigger, messageTs)
	if err != nil {
		return err
	}
	return sender.updateDashboards(events, &contact, &trigger, messageTs, channelId, state, needAck, throttled)
}

func (sender *Sender) formatStartingMessage(
	events moira.NotificationEvents, trigger *moira.TriggerData,
	state string, needAck bool, throttled bool,
) []slack.MsgOption {
	logger := logging.GetLogger(trigger.ID)
	tags := trigger.GetTags()
	triggerURL := fmt.Sprintf("%s/trigger/%s", sender.FrontURI, events[0].TriggerID)

	message := strings.Builder{}
	message.WriteString(fmt.Sprintf("*%s* %s <%s|%s>\n", state, tags, triggerURL, trigger.Name))

	if trigger.Desc != "" {
		if !isDescriptionInCommentEnabled(trigger) {
			message.WriteString(trigger.Desc + "\n")
		} else if commands := extractAvitoErrbotCommands(trigger.Desc); commands != "" {
			message.WriteString(commands + "\n")
		}
	}

	if needAck && !sender.ackWithButton {
		message.WriteString(fmt.Sprintf("Please acknowledge this alert <%s|HERE> or it will be escalated \n", triggerURL))
	}

	sender.writeEventMessages(&message, events)
	if throttled {
		message.WriteString("\nPlease, *fix your system or tune this trigger* to generate less events.")
	}

	messageBody := message.String()
	logger.DebugE(fmt.Sprintf("Prepared message body for triggerID %s", trigger.ID), map[string]interface{}{
		"message_body": messageBody,
		"trigger_id":   trigger.ID,
	})

	var (
		color, icon string
		params      = baseParams
		attachments = make([]slack.Attachment, 0, 2)
		result      = []slack.MsgOption{slack.MsgOptionText(messageBody, false)}
	)

	if state == moira.OK {
		icon = fmt.Sprintf("%s/public/fav72_ok.png", sender.FrontURI)
		color = colorGood
	} else {
		icon = fmt.Sprintf("%s/public/fav72_error.png", sender.FrontURI)
		color = colorDanger
	}

	params.IconURL = icon
	result = append(result, slack.MsgOptionPostMessageParameters(params))

	if trigger.Dashboard != "" && sender.webdav != nil {
		imgData, err := grafana.MakeScreenshot(trigger.Dashboard)
		if err != nil {
			logger.WarnF("Unable to make screenshot: %v", err)
		} else {
			url, err := sender.webdav.Upload(imgData)
			if err != nil {
				logger.WarnF("Webdav error %v", err)
			} else {
				attachments = append(attachments, slack.Attachment{
					Color:    color,
					ImageURL: url,
					Fallback: "",
				})
			}
		}
	}
	if needAck && sender.ackWithButton {
		ackValue := map[string]interface{}{
			"metrics": extractFailedMetrics(events),
		}
		ackValueEncoded, err := json.Marshal(ackValue)
		if len(ackValueEncoded) >= 2000 {
			err = fmt.Errorf("exceeded max value length accepted in Slack")
		}
		if err != nil {
			logger.ErrorE("Could not create value for Slack", map[string]interface{}{
				"Trigger": trigger,
				"Events":  events,
			})
			ackValueEncoded = nil
		}

		attachments = append(attachments, slack.Attachment{
			CallbackID: ackCallbackID + trigger.ID,
			Actions: []slack.AttachmentAction{{
				Name:  "trigger_id",
				Text:  "Acknowledge",
				Type:  "button",
				Style: "primary",

				Value: string(ackValueEncoded),
			}},
		})
	}

	if len(attachments) > 0 {
		result = append(result, slack.MsgOptionAttachments(attachments...))
	}

	return result
}

func (sender *Sender) markMessageAsOK(
	events moira.NotificationEvents,
	contact *moira.ContactData,
	channelId string,
	messageTs string,
	logger *logging.Logger,
) {
	itemRef := slack.ItemRef{Channel: channelId, Timestamp: messageTs}
	args, err := json.Marshal(itemRef)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to marshal Slack API action [AddReaction] into JSON: %v", err))
		return
	}

	_ = sender.DataBase.SaveSlackDelayedAction(moira.SlackDelayedAction{
		Action:      "AddReaction",
		EncodedArgs: args,
		Contact:     *contact,
	})

	logger.InfoE("created delayed Slack action: AddReaction", map[string]interface{}{
		"contact":      contact,
		"events":       events,
		"slackItemRef": itemRef,
	})
}

// startNewThread appends typical messages (e.g. attachments) to created one in thread-mode representation
func (sender *Sender) startNewThread(
	events moira.NotificationEvents,
	contact *moira.ContactData,
	trigger *moira.TriggerData,
	messageTs string,
) error {
	logger := logging.GetLogger(trigger.ID)
	api := NewSlack(sender.APIToken, logger)

	threadParams := baseParams
	threadParams.ThreadTimestamp = messageTs
	threadParamsOption := slack.MsgOptionPostMessageParameters(threadParams)

	// bind rendered dashboard to created thread
	dashboard := moira.MakeDashboardFromEvents(events)
	dashboardRendered := RenderDashboardForSlack(dashboard, maxMetricsPerMessage)
	_, dashboardTs, err := api.PostMessage(
		contact.Value,
		slack.MsgOptionText(dashboardRendered, false),
		threadParamsOption,
	)
	if err != nil {
		switch err.Error() {
		case "message_not_found", "is_inactive":
			// ignore these errors
			logger.WarnF("Failed to send dashboard message to slack [%s], will not retry: %v", contact.Value, err)
		default:
			return fmt.Errorf("Failed to send dashboard message to slack [%s]: %s", contact.Value, err.Error())
		}
	}

	// cache thread data
	if dashboardTs != "" {
		_ = sender.DataBase.UpdateSlackDashboard(contact.Value, dashboardTs, dashboard, contact.Expiration)
		for _, event := range events {
			if event.State != moira.OK {
				_ = sender.DataBase.AddSlackThreadLinks(contact.Value, trigger.ID, dashboardTs, messageTs, contact.Expiration)
			}
		}
	}

	// add description to the thread
	if trigger.Desc != "" && isDescriptionInCommentEnabled(trigger) {
		messageWithoutCommands := removeAvitoErrbotCommands(trigger.Desc) + "\n"
		_, _, err := api.PostMessage(
			contact.Value,
			slack.MsgOptionText(messageWithoutCommands, false),
			threadParamsOption,
		)
		if err != nil {
			switch err.Error() {
			case "message_not_found", "is_inactive":
				// ignore these errors
				logger.WarnF("Failed to send description message to slack [%s], will not retry: %v", contact.Value, err)
			default:
				return fmt.Errorf("Failed to send description message to slack [%s]: %s", contact.Value, err.Error())
			}
		}
	}

	// add images if there are any
	attachments := make([]slack.Attachment, 0)
	images := make(map[string]bool)
	for _, event := range events {
		if event.Context == nil {
			continue
		}

		for _, image := range event.Context.Images {
			if !images[image.URL] {
				attachments = append(attachments, slack.Attachment{
					ImageURL: image.URL,
					// `Fallback` must be non-empty, or Slack will ignore the message silently
					Fallback: image.SourceURL,
					Text:     formatLink(image.Caption, image.SourceURL),
				})
				images[image.URL] = true
			}
		}
	}
	if len(attachments) > 0 {
		_, _, err := api.PostMessage(
			contact.Value,
			slack.MsgOptionText("", false),
			slack.MsgOptionAttachments(attachments...),
			threadParamsOption,
		)
		if err != nil {
			switch err.Error() {
			case "message_not_found", "is_inactive":
				// ignore these errors
				logger.WarnF("Failed to send images to slack [%s], will not retry: %v", contact.Value, err)
			default:
				return fmt.Errorf("Failed to send images to slack [%s]: %s", contact.Value, err.Error())
			}
		}
	}

	if context := events.GetContext(); context != nil && context.DeployStatuses != "" {
		_, _, err := api.PostMessage(
			contact.Value,
			slack.MsgOptionText(context.DeployStatuses+"\n", false),
			threadParamsOption,
		)
		if err != nil {
			switch err.Error() {
			case "message_not_found", "is_inactive":
				// ignore these errors
				logger.WarnF("Failed to send deploy statuses message to slack [%s], will not retry: %v", contact.Value, err)
			default:
				return fmt.Errorf("Failed to send deploy statuses message to slack [%s]: %s", contact.Value, err.Error())
			}
		}
	}

	// look for DBaaS services
	if context := events.GetContext(); context != nil {
		metricNames := map[string][]string{
			"metrics": extractMetrics(events),
		}
		metricNamesEncoded, err := json.Marshal(metricNames)
		if len(metricNamesEncoded) >= 2000 {
			err = fmt.Errorf("exceeded max value length accepted in Slack")
		}
		if err != nil {
			logger.ErrorE("Could not create value for Slack", map[string]interface{}{
				"Trigger": trigger,
				"Events":  events,
			})
			metricNamesEncoded = nil
		}

		for _, service := range context.ServiceChannels.DBaaS {
			// add buttons
			blockID := fmt.Sprintf("dbaas-metric-control:%s:%s", service.ServiceName, contact.Value)
			_, _, _ = api.PostMessage(
				contact.Value,
				slack.MsgOptionBlocks(
					slack.NewActionBlock(
						blockID,

						&slack.ButtonBlockElement{
							Type:     "button",
							ActionID: "edit-threshold",
							Text: &slack.TextBlockObject{
								Type: "plain_text",
								Text: "Edit",
							},
							Value: string(metricNamesEncoded),
						},

						&slack.ButtonBlockElement{
							Type:     "button",
							ActionID: "mute",
							Text: &slack.TextBlockObject{
								Type: "plain_text",
								Text: "Mute",
							},
						},
					),
				),
				threadParamsOption,
			)
		}
	}

	return nil
}

func (sender *Sender) updateDashboard(
	events moira.NotificationEvents, contact *moira.ContactData, trigger *moira.TriggerData,
	channelId string, threadTs string, dashboardTs string,
	oksWithoutDashboards map[moira.NotificationEvent]bool,
) (isEverythingOK bool, err error) {
	logger := logging.GetLogger(trigger.ID)
	api := NewSlack(sender.APIToken, logger)

	// TODO(pafomin@): this code should not be necessary after 2021-01-01
	// because all dashboards should already have expiry times set in Redis
	if i, err := strconv.ParseFloat(dashboardTs, 32); err == nil {
		if float64(time.Now().Unix())-i >= threadTTL {
			return true, nil
		}
	}

	dashboard, _ := sender.DataBase.GetSlackDashboard(contact.Value, dashboardTs)
	hasChanged, changes := dashboard.Update(events, trigger.Name)

	if hasChanged {
		text := RenderDashboardForSlack(dashboard, 0)
		_, _, _, err := api.UpdateMessage(channelId, dashboardTs, text, true)
		if err != nil {
			switch err.Error() {
			case "message_not_found", "channel_not_found", "is_inactive", "msg_too_long", "cant_update_message":
				// ignore these errors
				logger.WarnF("Failed to update dashboard message in slack [%s], will not retry: %v", contact.Value, err)
			default:
				return false, fmt.Errorf("Failed to update dashboard message in slack [%s]: %s", contact.Value, err.Error())
			}
		}

		message := strings.Builder{}
		sender.writeEventMessages(&message, changes)
		for _, change := range changes {
			delete(oksWithoutDashboards, change)
		}

		threadParams := baseParams
		threadParams.ThreadTimestamp = threadTs

		_, _, err = api.PostMessage(
			contact.Value,
			slack.MsgOptionText(message.String(), false),
			slack.MsgOptionPostMessageParameters(threadParams),
		)
		if err != nil {
			return false, fmt.Errorf("Failed to send message to slack [%s]: %s", contact.Value, err.Error())
		}

	}

	if !dashboard.IsEverythingOK() {
		_ = sender.DataBase.UpdateSlackDashboard(contact.Value, dashboardTs, dashboard, contact.Expiration)
		return false, nil
	} else {
		sender.markMessageAsOK(events, contact, channelId, threadTs, logger)
		return true, nil
	}
}

func (sender *Sender) updateDashboards(
	events moira.NotificationEvents, contact *moira.ContactData, trigger *moira.TriggerData,
	currentThreadTs string, channelId string, state string, needAck bool, throttled bool,
) error {
	logger := logging.GetLogger(trigger.ID)
	api := NewSlack(sender.APIToken, logger)

	completedThreads := make([]string, 0)
	completedDashboards := make([]string, 0)
	oksWithoutDashboards := make(map[moira.NotificationEvent]bool)
	for _, event := range events {
		if event.State == moira.OK {
			oksWithoutDashboards[event] = true
		}
	}
	if threads, err := sender.DataBase.GetSlackThreadLinks(contact.Value, trigger.ID); err == nil {
		for threadTs, dashboardTs := range threads {
			if threadTs == currentThreadTs {
				continue
			}

			isCompleted, err := sender.updateDashboard(events, contact, trigger, channelId, threadTs, dashboardTs, oksWithoutDashboards)
			if err != nil {
				return err
			}
			if isCompleted {
				completedThreads = append(completedThreads, threadTs)
				completedDashboards = append(completedDashboards, dashboardTs)
			}
		}
	}

	if state == moira.OK && len(oksWithoutDashboards) > 0 {
		eventsWithoutDashboards := make(moira.NotificationEvents, 0, len(oksWithoutDashboards))
		for key := range oksWithoutDashboards {
			eventsWithoutDashboards = append(eventsWithoutDashboards, key)
		}

		msgOptions := sender.formatStartingMessage(eventsWithoutDashboards, trigger, state, needAck, throttled)
		_, _, err := api.PostMessage(contact.Value, msgOptions...)
		if err != nil {
			return fmt.Errorf("Failed to send message to slack [%s]: %s", contact.Value, err.Error())
		}
	}

	_ = sender.DataBase.RemoveSlackThreadLinks(contact.Value, trigger.ID, completedDashboards, completedThreads)
	_ = sender.DataBase.RemoveSlackDashboards(contact.Value, completedDashboards)
	return nil
}

func (sender *Sender) writeEventMessage(messageBuffer *strings.Builder, event moira.NotificationEvent) {
	value := strconv.FormatFloat(moira.UseFloat64(event.Value), 'f', -1, 64)
	messageBuffer.WriteString(fmt.Sprintf("\n%s: %s = %s (%s to %s)", time.Unix(event.Timestamp, 0).In(sender.location).Format("15:04"), event.Metric, value, event.OldState, event.State))
	if len(moira.UseString(event.Message)) > 0 {
		messageBuffer.WriteString(fmt.Sprintf(". %s", moira.UseString(event.Message)))
	}
}

func (sender *Sender) writeEventMessages(messageBuffer *strings.Builder, events moira.NotificationEvents) {
	messageBuffer.WriteString("```")

	type compactEvent struct {
		event moira.NotificationEvent
		extra int

		metricName string
		finalState string
	}
	eventsByMetricName := make(map[string]*compactEvent)
	for _, event := range events {
		metricName := event.Metric
		data, found := eventsByMetricName[metricName]
		if !found {
			data = new(compactEvent)
			data.metricName = metricName
			eventsByMetricName[metricName] = data
		} else {
			data.extra += 1
		}
		data.event = event
		data.finalState = event.State
	}
	metricsToWrite := make([]*compactEvent, 0, len(eventsByMetricName))
	for _, data := range eventsByMetricName {
		metricsToWrite = append(metricsToWrite, data)
	}
	sort.SliceStable(metricsToWrite, func(i, j int) bool {
		return metricsToWrite[i].event.Timestamp < metricsToWrite[j].event.Timestamp
	})
	sort.SliceStable(metricsToWrite, func(i, j int) bool {
		// if metric i is in error...
		if metricsToWrite[i].finalState != moira.OK {
			// if metric j is NOT in error...
			// ...then show metric i before metric j
			if metricsToWrite[j].finalState == moira.OK {
				return true
			}
		}
		return false
	})

	numOfMetricsToOutput := min(len(metricsToWrite), maxMetricsPerMessage)
	for i := 0; i < numOfMetricsToOutput; i++ {
		data := metricsToWrite[i]
		sender.writeEventMessage(messageBuffer, data.event)
		if data.extra > 0 {
			message := fmt.Sprintf("\n(and %d more)", data.extra)
			messageBuffer.WriteString(message)
		}
	}
	if numOfMetricsToOutput < len(metricsToWrite) {
		message := fmt.Sprintf("\nand %d more metrics", len(metricsToWrite)-numOfMetricsToOutput)
		messageBuffer.WriteString(message)
	}

	messageBuffer.WriteString("```")
}

func (sender *Sender) SendEventsWithInheritance(
	events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData,
	ancestorTriggerID, ancestorMetric string,
) error {
	logger := logging.GetLogger(trigger.ID)
	api := NewSlack(sender.APIToken, logger)

	// state := events.GetSubjectState()
	state := "child trigger:"
	tags := trigger.GetTags()
	triggerURL := fmt.Sprintf("%s/trigger/%s", sender.FrontURI, events[0].TriggerID)

	header := fmt.Sprintf("*%s* %s <%s|%s>\n", state, tags, triggerURL, trigger.Name)

	threadsWithDashboards, err := sender.DataBase.GetAllInheritedTriggerDashboards(
		trigger.ID,
		ancestorTriggerID, ancestorMetric,
	)
	if err != nil {
		return fmt.Errorf("Could not get ancestor dashboards: %s", err.Error())
	}
	err = sender.updateParentDashboards(ancestorTriggerID, ancestorMetric, events, contact, trigger, header, threadsWithDashboards)
	if err != nil {
		return err
	}

	threadLinks, err := sender.DataBase.GetAllSlackThreadLinks(ancestorTriggerID)
	if err != nil {
		return fmt.Errorf("could not get Slack threads(%v): %s", trigger.ID, err.Error())
	}
	for _, link := range threadLinks {
		// ensure that the thread does have the metric we want
		ancestorDashboard, err := sender.DataBase.GetSlackDashboard(link.Contact, link.DashboardTs)
		if err != nil {
			logger.ErrorE("could not get ancestor dashboard", map[string]interface{}{
				"Trigger":           trigger,
				"AncestorTriggerID": ancestorTriggerID,
				"AncestorMetric":    ancestorMetric,
				"ThreadLink":        link,
				"Events":            events,
			})
		}
		if metricIsOk, found := ancestorDashboard[ancestorMetric]; metricIsOk || !found {
			continue
		}

		// create a new dashboard
		var message bytes.Buffer
		message.WriteString(header)
		dashboard := moira.MakeDashboardFromEvents(events)
		message.WriteString(RenderDashboardForSlack(dashboard, maxMetricsPerMessage))

		threadParams := baseParams
		threadParams.ThreadTimestamp = link.ThreadTs

		// TODO: send this asynchronously
		_, newDashboardTs, err := api.PostMessage(
			link.Contact,
			slack.MsgOptionText(message.String(), false),
			slack.MsgOptionPostMessageParameters(threadParams),
		)
		if err != nil {
			switch err.Error() {
			case "message_not_found", "is_inactive":
				// ignore these errors
				logger.WarnF("Failed to send message to slack [%s], will not retry: %v", link.Contact, err)
			default:
				return fmt.Errorf("Failed to send message to slack [%s]: %s", link.Contact, err.Error())
			}
		}
		_ = sender.DataBase.SaveInheritedTriggerDashboard(
			link.Contact, link.ThreadTs,
			trigger.ID, ancestorTriggerID, ancestorMetric,
			newDashboardTs,
		)
		_ = sender.DataBase.UpdateSlackDashboard(link.Contact, newDashboardTs, dashboard, nil)
	}

	// TODO: update all non-inheritance dashboards

	return nil
}

func (sender *Sender) updateParentDashboards(
	ancestorTriggerID, ancestorMetric string,
	events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData,
	header string, threadsWithDashboards []moira.SlackThreadLink,
) error {
	logger := logging.GetLogger(trigger.ID)
	api := NewSlack(sender.APIToken, logger)

	metricsPostedToThreads := make(map[moira.SlackThreadLink]map[string]interface{})
	changesByThread := make(map[moira.SlackThreadLink][]moira.NotificationEvent)

	for _, link := range threadsWithDashboards {

		// TODO(pafomin@): this code should not be necessary after 2021-01-01
		// because all dashboards should already have expiry times set in Redis
		if i, err := strconv.ParseFloat(link.DashboardTs, 32); err == nil {
			if float64(time.Now().Unix())-i >= threadTTL {
				_ = sender.DataBase.DeleteInheritedTriggerDashboard(
					link.Contact, link.ThreadTs,
					trigger.ID, ancestorTriggerID, ancestorMetric,
					link.DashboardTs,
				)
				_ = sender.DataBase.RemoveSlackDashboards(link.Contact, []string{link.DashboardTs})
				continue
			}
		}

		dashboard, err := sender.DataBase.GetSlackDashboard(link.Contact, link.DashboardTs)
		if err != nil {
			logger.ErrorF("Error while getting Slack dashboard, channel %s, ts %s: %v", link.Contact, link.DashboardTs, err)
			continue
		}

		if hasChanged, changes := dashboard.Update(events, trigger.Name); hasChanged {
			channelId, err := sender.DataBase.GetIDByUsername(messenger, link.Contact)
			if err != nil {
				return fmt.Errorf("Failed to get Slack channel ID from username [%s]: %s", link.Contact, err.Error())
			}

			text := header + RenderDashboardForSlack(dashboard, maxMetricsPerMessage)
			_, _, _, err = api.UpdateMessage(channelId, link.DashboardTs, text, false)
			if err != nil {
				switch err.Error() {
				case "message_not_found", "channel_not_found", "is_inactive", "msg_too_long", "cant_update_message":
					// ignore these errors
					logger.WarnF("Failed to update dashboard message in slack [%s], will not retry: %s", link.Contact, err.Error())
				default:
					return fmt.Errorf("Failed to update dashboard message in slack [%s]: %s", link.Contact, err.Error())
				}
			}

			threadLink := moira.SlackThreadLink{
				Contact:  link.Contact,
				ThreadTs: link.ThreadTs,
			}
			for _, change := range changes {
				metrics, ok := metricsPostedToThreads[threadLink]
				if !ok {
					metrics = make(map[string]interface{})
					metricsPostedToThreads[threadLink] = metrics
				}
				if _, found := metrics[change.Metric]; !found {
					changesByThread[threadLink] = append(changesByThread[threadLink], change)
					metrics[change.Metric] = true
				}
			}

			if !dashboard.IsEverythingOK() {
				_ = sender.DataBase.UpdateSlackDashboard(link.Contact, link.DashboardTs, dashboard, nil)
			} else {
				contactData := &moira.ContactData{
					Type:  contact.Type,
					Value: link.Contact,
				}
				sender.markMessageAsOK(events, contactData, channelId, link.DashboardTs, logger)

				_ = sender.DataBase.DeleteInheritedTriggerDashboard(
					link.Contact, link.ThreadTs,
					trigger.ID, ancestorTriggerID, ancestorMetric,
					link.DashboardTs,
				)
				_ = sender.DataBase.RemoveSlackDashboards(link.Contact, []string{link.DashboardTs})
			}
		}
	}

	for threadLink, changes := range changesByThread {
		message := strings.Builder{}
		message.WriteString(header)
		sender.writeEventMessages(&message, changes)

		threadParams := baseParams
		threadParams.ThreadTimestamp = threadLink.ThreadTs

		_, _, err := api.PostMessage(
			threadLink.Contact,
			slack.MsgOptionText(message.String(), false),
			slack.MsgOptionPostMessageParameters(threadParams),
		)
		if err != nil {
			logger.ErrorF("Failed to send message to slack [%s]: %s", threadLink.Contact, err.Error())
		}
	}

	return nil
}
