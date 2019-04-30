package vspherePerfManager

import (
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vim25/methods"
	"context"
	"github.com/vmware/govmomi/vim25/mo"
	u "github.com/ahl5esoft/golang-underscore"
	"time"
	"regexp"
)

type Metric struct {
	Info  metricInfo
	Value metricValue
}

type metricInfo struct {
	Metric    string
	StatsType string
	UnitInfo  string
}

type metricValue struct {
	Value     int64
	Instance  string
	Timestamp time.Time
}

// ProviderSummary wraps the QueryPerfProviderSummary method, caching the value based on entity.Type.
func (v *VspherePerfManager) ProviderSummary(entity types.ManagedObjectReference) (*types.PerfProviderSummary, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := types.QueryPerfProviderSummary{
		This:   *v.client.ServiceContent.PerfManager,
		Entity: entity,
	}

	res, err := methods.QueryPerfProviderSummary(ctx, v.client, &req)
	if err != nil {
		return nil, err
	}

	return &res.Returnval, nil
}

func (v *VspherePerfManager) getAvailablePerfMetrics(entity types.ManagedObjectReference, intervalId int32, startTime *time.Time) []types.PerfMetricId {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	perfReq := types.QueryAvailablePerfMetric{
		This:       *v.client.ServiceContent.PerfManager,
		Entity:     entity,
		BeginTime:  startTime,
		IntervalId: intervalId,
	}

	perfRes, _ := methods.QueryAvailablePerfMetric(ctx, v.client.RoundTripper, &perfReq )
	return perfRes.Returnval
}

func (v *VspherePerfManager) filterWithConfig(metrics []types.PerfMetricId, entity ManagedObject) []types.PerfMetricId {
	var filteredMetrics []types.PerfMetricId

	if len(v.Config.Metrics[PmSupportedEntities(entity.Entity.Type)]) == 0 {
		return metrics
	}

	for _, metric := range metrics {

		if v.isMetricDefMatch(entity, metric) {
			filteredMetrics = append(filteredMetrics, metric)
		}

	}

	return filteredMetrics

}

func createPerfQuerySpec(entity types.ManagedObjectReference, metricsIds []types.PerfMetricId, intervalId int32, startTime *time.Time) []types.PerfQuerySpec {
	return []types.PerfQuerySpec{{
		Entity:     entity,
		StartTime:  startTime,
		MetricId:   metricsIds,
		IntervalId: intervalId,
	}}
}

func (v *VspherePerfManager) getMetricsInfo() (map[int32]metricInfo, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var perfmanager mo.PerformanceManager
	err := v.client.RetrieveOne(ctx, *v.client.ServiceContent.PerfManager, nil, &perfmanager)
	if err != nil {
		return nil, err
	}

	metrics := make(map[int32]metricInfo)

	for _, metric := range perfmanager.PerfCounter {

		metrics[metric.Key] = metricInfo{
			Metric   :  metric.GroupInfo.GetElementDescription().Key + "." + metric.NameInfo.GetElementDescription().Key + "." + string(metric.RollupType),
			StatsType : string(metric.StatsType),
			UnitInfo  : metric.UnitInfo.GetElementDescription().Key,
		}
	}

	return metrics, nil

}

func (v *VspherePerfManager) isMetricDefMatch(entity ManagedObject, metric types.PerfMetricId) bool {
	return u.Any(v.Config.Metrics[PmSupportedEntities(entity.Entity.Type)], func(metricDef MetricDef, _ int) bool {
		return isMatch(v.GetProperty(entity, "name").(string), metricDef.Entities) &&
			      isMatch(v.metricsInfo[metric.CounterId].Metric, metricDef.Metrics) &&
					isMatch(metric.Instance, metricDef.Instances)
	})
}

func isMatch(value string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if pattern == ALL[0] {
			return true
		}
		re := regexp.MustCompile("(?i)"+pattern)
		if re.MatchString(value) {
			return true
		}
	}
	return false
}