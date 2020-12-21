package silencer

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gopkg.in/tomb.v2"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/logging"
	"go.avito.ru/DO/moira/netbox"
)

// name of user that creates silent patterns based on inactive servers info
const (
	autoCreateSPDuration = 60 * time.Minute
	autoCreateSPUser     = "moira-cmdb-auto"
	checkInterval        = 10 // seconds
	silencerLockKey      = "moira-silencer-full-update"
)

var (
	blacklist = []string{"servers", "network", "containers", "resources", "apps", "products", "complex", "offices"}
)

var (
	initLock sync.Mutex
	silencer *Silencer
)

// Silencer represents worker that stores and periodically updates silent patterns' cache
// it is an auxiliary component which is launched and maintained by any main component, such as checker or notifier
type Silencer struct {
	mu           sync.Mutex
	isFullUpdate bool
	metrics      moira.Maintenance
	tags         moira.Maintenance

	database moira.Database
	logger   *logging.Logger
	netbox   *netbox.Client

	tomb tomb.Tomb
}

// NewSilencer returns Silencer singleton instance
// it also creates it if necessary
func NewSilencer(database moira.Database, config *netbox.Config) *Silencer {
	if silencer == nil {
		initLock.Lock()
		defer initLock.Unlock()

		if silencer == nil {
			silencer = &Silencer{
				metrics:  moira.NewMaintenance(),
				tags:     moira.NewMaintenance(),
				database: database,
				logger:   logging.GetLogger(""),
			}
		}
	}

	if config != nil && !silencer.isFullUpdate {
		silencer.isFullUpdate = true
		silencer.netbox = netbox.CreateClient(config)
		silencer.netbox.SetTimeout(5 * time.Second)
	}

	return silencer
}

// Start begins the lifecycle of the Silencer
func (worker *Silencer) Start() {
	// starting the timer that updates silent pattern list every 10 seconds
	worker.tomb.Go(func() error {
		checkTicker := time.NewTicker(checkInterval * time.Second)
		defer checkTicker.Stop()

		for {
			select {
			case <-worker.tomb.Dying():
				return nil
			case <-checkTicker.C:
				worker.updateSilentPatterns()
			}
		}
	})
}

// Stop ends the lifecycle of the Silencer
func (worker *Silencer) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}

// IsMetricSilenced can tell whether or not the given metric matches any silent pattern
func (worker *Silencer) IsMetricSilenced(metric string, ts int64) bool {
	metrics := worker.metrics
	for pattern := range metrics {
		// see if the metric matches current pattern in the first place
		if !worker.isPatternMatched(metric, pattern) {
			continue
		}

		// if it does then make sure that timing is correct
		maintained, _ := metrics.Get(metric, ts)
		if maintained {
			return true
		}
	}
	return false
}

// IsTagsSilenced can tell whether or not any of given tags must be silenced
func (worker *Silencer) IsTagsSilenced(tags []string, ts int64) bool {
	maintenance := worker.tags
	for _, tag := range tags {
		if maintained, _ := maintenance.Get(tag, ts); maintained {
			return true
		}
	}
	return false
}

// getInactiveDeviceList requests the list of inactive devices using netbox client
// it also handles panic
func (worker *Silencer) getInactiveDeviceList() netbox.DeviceBriefList {
	defer func() {
		if r := recover(); r != nil {
			worker.logger.Error(fmt.Sprintf("Silencer recovered from netbox client panic: %v (%T)", r, r))
		}
	}()

	return worker.netbox.InactiveDeviceList()
}

// getSilentMetricsQtyEstimate returns estimated quantity of silent metrics
// (both devices and containers) by inactive devices data
func (worker *Silencer) getSilentMetricsQtyEstimate(list netbox.DeviceBriefList) int {
	result := 2 * len(list.List)
	for _, deviceInfo := range list.List {
		result += len(deviceInfo.Containers)
	}
	return result
}

// isPatternMatched can tell whether or not the given metric matches the given silent pattern
func (worker *Silencer) isPatternMatched(metricName, silentPattern string) bool {
	metricParts := strings.Split(metricName, ".")
	patternParts := strings.Split(silentPattern, ".")
	if len(patternParts) > len(metricParts) {
		return false
	}

	pIndex := 0
	matchCount := 0
	isLastMatched := false

	mIndex := 0
	for _, b := range blacklist { // skip blacklist prefix
		if metricParts[0] == b {
			mIndex = 1
			break
		}
	}

	for ; mIndex < len(metricParts); mIndex++ {
		if metricParts[mIndex] == patternParts[pIndex] {
			pIndex++
			matchCount++
			isLastMatched = true
			if matchCount == len(patternParts) {
				return true
			}
		} else {
			pIndex = 0
			if isLastMatched {
				isLastMatched = false
				mIndex -= matchCount
			}
			matchCount = 0
		}
	}

	return false
}

// silentMetricsFullUpdate makes full update of silent metrics:
// it reloads current list from DB and merges it with inactive containers and/or servers
func (worker *Silencer) silentMetricsFullUpdate() error {
	// reload current list
	currentMetrics, err := worker.database.GetSilentPatternsTyped(moira.SPTMetric)
	if err != nil {
		return err
	}

	// get inactive containers and servers data from netbox
	inactiveDeviceList := worker.getInactiveDeviceList()
	inactiveDeviceQty := worker.getSilentMetricsQtyEstimate(inactiveDeviceList)

	if err := worker.database.LockSilentPatterns(moira.SPTMetric); err != nil {
		return err
	}
	defer worker.database.UnlockSilentPatterns(moira.SPTMetric)

	inactiveNames := make(map[string]bool, inactiveDeviceQty)                        // set for processed names
	processedMetrics := make(map[string]*moira.SilentPatternData, inactiveDeviceQty) // map for processed metrics
	metricsUpdateBatch := make([]*moira.SilentPatternData, 0, inactiveDeviceQty)
	metricsRemoveBatch := make([]*moira.SilentPatternData, 0, inactiveDeviceQty)
	result := make([]string, 0, len(currentMetrics)+inactiveDeviceQty)

	// transform servers-and-containers list to the flat map
	for _, deviceInfo := range inactiveDeviceList.List {
		inactiveNames[deviceInfo.Name] = true // name of the server
		if deviceInfo.NamePrevious != "" && deviceInfo.NamePrevious != deviceInfo.Name {
			inactiveNames[deviceInfo.NamePrevious] = true // and its previous name
		}
		for _, containerInfo := range deviceInfo.Containers {
			inactiveNames[containerInfo.Name] = true // names of its containers
		}
	}

	//
	// start processing
	//

	// first, process current list
	for _, currentMetric := range currentMetrics {
		var (
			// leave only those patterns which aren't expired yet
			expired = time.Now().Unix() > currentMetric.Until
			// also pattern could be auto-created based on inactive device list, but this device is inactive no longer
			obsolete = currentMetric.Login == autoCreateSPUser && !inactiveNames[currentMetric.Pattern]

			pattern = currentMetric.Pattern
		)

		if !expired && !obsolete {
			processedMetrics[pattern] = currentMetric
			result = append(result, pattern)
			continue
		}

		worker.logger.InfoE(fmt.Sprintf("Silent metric '%s' added to remove batch", pattern), map[string]interface{}{
			"expired":  expired,
			"obsolete": obsolete,
			"pattern":  pattern,
		})
		metricsRemoveBatch = append(metricsRemoveBatch, currentMetric)
	}

	worker.logger.InfoE(fmt.Sprintf("Got remove batch of %d metrics", len(metricsRemoveBatch)), metricsRemoveBatch)
	if len(metricsRemoveBatch) > 0 {
		if err = worker.database.RemoveSilentPatterns(moira.SPTMetric, metricsRemoveBatch...); err != nil {
			return err
		}
	}

	// next, add or update metrics based on inactive servers-and-containers
	now := time.Now()
	autoPatternCreated := now.Unix()
	autoPatternExpiration := now.Add(autoCreateSPDuration).Unix()
	for inactiveName := range inactiveNames {
		spd, ok := processedMetrics[inactiveName]
		if !ok {
			// this container or server hasn't been added yet, so add it now
			spd = &moira.SilentPatternData{
				Pattern: inactiveName,
				Login:   autoCreateSPUser,
				Created: autoPatternCreated,
			}
			result = append(result, inactiveName)
		}

		if spd.Login == autoCreateSPUser {
			// if it is auto-created pattern then its expiration will be extended
			spd.Until = autoPatternExpiration
			metricsUpdateBatch = append(metricsUpdateBatch, spd)
		}
	}

	worker.logger.InfoE(fmt.Sprintf("Got update batch of %d metrics", len(metricsUpdateBatch)), metricsUpdateBatch)
	if len(metricsUpdateBatch) > 0 {
		if err = worker.database.SaveSilentPatterns(moira.SPTMetric, metricsUpdateBatch...); err != nil {
			return err
		}
	}

	return nil
}

// updateSilentMetrics refreshes silent metrics list and returns new list
func (worker *Silencer) updateSilentMetrics(isFullUpdate bool) (moira.Maintenance, error) {
	if isFullUpdate {
		if err := worker.silentMetricsFullUpdate(); err != nil {
			return nil, err
		}
	}
	return worker.database.GetOrCreateMaintenanceSilent(moira.SPTMetric)
}

// updateSilentTags refreshes silent tags list and returns new map
func (worker *Silencer) updateSilentTags(isFullUpdate bool) (moira.Maintenance, error) {
	// current list
	silentTags, err := worker.database.GetSilentPatternsTyped(moira.SPTTag)
	if err != nil {
		return nil, err
	}

	// updated map
	newSilentTags := make(map[string]bool, len(silentTags))
	// tags to remove
	removeTags := make([]*moira.SilentPatternData, 0, len(silentTags))

	// set the lock in case tags are going to be updated
	if isFullUpdate {
		if err := worker.database.LockSilentPatterns(moira.SPTTag); err != nil {
			return nil, err
		}
		defer worker.database.UnlockSilentPatterns(moira.SPTTag)
	}

	for _, silentTag := range silentTags {
		if isFullUpdate && time.Now().Unix() > silentTag.Until {
			worker.logger.InfoE("Remove silent tag (obsolete)", silentTag)
			removeTags = append(removeTags, silentTag)
		} else {
			newSilentTags[silentTag.Pattern] = true
		}
	}

	if len(removeTags) > 0 {
		if err = worker.database.RemoveSilentPatterns(moira.SPTTag, removeTags...); err != nil {
			return nil, err
		}
	}

	return worker.database.GetOrCreateMaintenanceSilent(moira.SPTTag)
}

// updateSilentPatterns refreshes silent patterns data
func (worker *Silencer) updateSilentPatterns() {
	worker.mu.Lock()
	defer worker.mu.Unlock()

	isFullUpdate := worker.isFullUpdate
	if isFullUpdate {
		isFullUpdate, _ = worker.database.SetLock(silencerLockKey, checkInterval)
	}

	metrics, err := worker.updateSilentMetrics(isFullUpdate)
	if err != nil {
		worker.logger.ErrorF("Failed to update silent metrics: %v", err)
		return
	}
	worker.metrics = metrics

	tags, err := worker.updateSilentTags(isFullUpdate)
	if err != nil {
		worker.logger.ErrorF("Failed to update silent tags: %v", err)
		return
	}
	worker.tags = tags
}
