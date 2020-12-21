package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
)

// Sender implements moira sender interface via script execution
type Sender struct {
	Exec string
	log  *logging.Logger
}

type scriptNotification struct {
	Events    []moira.NotificationEvent `json:"events"`
	Trigger   moira.TriggerData         `json:"trigger"`
	Contact   moira.ContactData         `json:"contact"`
	Throttled bool                      `json:"throttled"`
	Timestamp int64                     `json:"timestamp"`
}

// Init read yaml config
func (sender *Sender) Init(senderSettings map[string]string, _ *time.Location) error {
	if senderSettings["name"] == "" {
		return fmt.Errorf("Required name for sender type script")
	}
	args := strings.Split(senderSettings["exec"], " ")
	scriptFile := args[0]
	infoFile, err := os.Stat(scriptFile)
	if err != nil {
		return fmt.Errorf("File %s not found", scriptFile)
	}
	if !infoFile.Mode().IsRegular() {
		return fmt.Errorf("%s not file", scriptFile)
	}
	sender.Exec = senderSettings["exec"]
	sender.log = logging.GetLogger("")
	return nil
}

// SendEvents implements Sender interface Send
func (sender *Sender) SendEvents(events moira.NotificationEvents, contact moira.ContactData, trigger moira.TriggerData, throttled, _ bool) error {
	execString := strings.Replace(sender.Exec, "${trigger_name}", trigger.Name, -1)
	execString = strings.Replace(execString, "${contact_value}", contact.Value, -1)

	sender.log.InfoE("SendEvents via script sender", map[string]interface{}{
		"contact":     contact,
		"trigger":     trigger,
		"exec_string": execString,
	})

	args := strings.Split(execString, " ")
	scriptFile := args[0]
	infoFile, err := os.Stat(scriptFile)
	if err != nil {
		return fmt.Errorf("File %s not found", scriptFile)
	}
	if !infoFile.Mode().IsRegular() {
		return fmt.Errorf("%s not file", scriptFile)
	}

	scriptMessage := &scriptNotification{
		Events:    events,
		Trigger:   trigger,
		Contact:   contact,
		Throttled: throttled,
	}
	scriptJSON, err := json.MarshalIndent(scriptMessage, "", "\t")
	if err != nil {
		return fmt.Errorf("Failed marshal json")
	}
	sender.log.InfoE("Built script package", scriptMessage)

	var (
		scriptStdout bytes.Buffer
		scriptStderr bytes.Buffer
	)

	c := exec.Command(scriptFile, args[1:]...)
	c.Stdin = bytes.NewReader(scriptJSON)
	c.Stdout = &scriptStdout
	c.Stderr = &scriptStderr

	sender.log.DebugF("Executing script: %s", scriptFile)
	err = c.Run()
	sender.log.DebugF("Finished executing: %s", scriptFile)

	if err != nil {
		sender.log.ErrorE(fmt.Sprintf("Failed to exec %s, error: %v", sender.Exec, err), map[string]interface{}{
			"stdout": scriptStdout.String(),
			"stderr": scriptStderr.String(),
		})
		return fmt.Errorf("Failed exec [%s] Error [%s] Output: [%s]", sender.Exec, err.Error(), scriptStdout.String())
	}

	sender.log.InfoF("Successfully finished %s", sender.Exec)
	return nil
}
