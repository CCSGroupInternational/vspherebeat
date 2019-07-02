package vspherePerfManager

import (
	"context"
	"github.com/thoas/go-funk"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
	"strings"
	"time"
)

func (v *VspherePerfManager) query(managedObject ManagedObject) ManagedObject {
	managedObject.Error = nil
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	summary, err := v.ProviderSummary(managedObject.Entity)
	if err != nil {
		managedObject.Error = err
		return managedObject
	}

	if summary.RefreshRate == -1 {
		summary.RefreshRate = 300
	}

	startTime, err := getStartTime(v.Config.Interval, summary.RefreshRate, v.client )

	if err != nil {
		managedObject.Error = err
		return managedObject
	}

	availableMetrics, err := v.getAvailablePerfMetrics(managedObject.Entity, summary.RefreshRate, &startTime)
	if err != nil {
		managedObject.Error = err
		return managedObject
	}

	metrics := v.filterWithConfig(availableMetrics.Returnval, managedObject)
	for _, metrics := range v.getDividedMetrics(metrics) {
		metricsSpec := createPerfQuerySpec(managedObject.Entity, metrics, summary.RefreshRate, &startTime)
		if len(metricsSpec[0].MetricId) != 0 {
			perfQueryReq := types.QueryPerf{
				This: *v.client.ServiceContent.PerfManager,
				QuerySpec: metricsSpec,
			}
			perfQueryRes, err := methods.QueryPerf(ctx, v.client.RoundTripper, &perfQueryReq )

			if err != nil {
				managedObject.Error = err
				return managedObject
			}

			if len(perfQueryRes.Returnval) == 0 {
				managedObject.Error = err
				return managedObject
			}

			v.setMetrics(&managedObject, perfQueryRes.Returnval)
		}
	}
	return managedObject
}

func (v *VspherePerfManager) setMetrics(managedObject *ManagedObject, metrics []types.BasePerfEntityMetricBase) {
	for _, base := range metrics {
		metric := base.(*types.PerfEntityMetric)
		for _, baseSerie := range metric.Value {
			serie := baseSerie.(*types.PerfMetricIntSeries)
			v.calculateRollup(*serie, managedObject, *metric)
		}
	}
}

func (v *VspherePerfManager) calculateRollup(serie types.PerfMetricIntSeries, managedObject *ManagedObject, metric types.PerfEntityMetric) {
	rollupInterval := v.getRollupTime(serie.Id.CounterId, serie.Id.Instance, *managedObject) / 20
	cycles := len(serie.Value) / rollupInterval
	for i := 0; i < cycles; i++ {
		result := calculateRollup(getRollup(v.metricsInfo[serie.Id.CounterId].Metric), serie.Value[i*rollupInterval : i*rollupInterval + rollupInterval])
		v.addMetric(result, managedObject, serie.Id.CounterId, serie.Id.Instance, metric.SampleInfo[i*rollupInterval + rollupInterval - 1].Timestamp )
	}
	if len(serie.Value) % rollupInterval != 0 {
		result := calculateRollup(getRollup(v.metricsInfo[serie.Id.CounterId].Metric), serie.Value[len(serie.Value)- (len(serie.Value) % rollupInterval) : ])
		v.addMetric(result, managedObject, serie.Id.CounterId, serie.Id.Instance, metric.SampleInfo[len(metric.SampleInfo)-1].Timestamp)
	}
}

func (v *VspherePerfManager) addMetric(value int64, managedObject *ManagedObject, counterId int32, instance string, timestamp time.Time) {
	managedObject.Metrics = append(managedObject.Metrics, Metric{
		Info: v.metricsInfo[counterId],
		Value: metricValue{
			Value: value,
			Instance: instance,
			Timestamp: timestamp,
		},
	})
}

func (v *VspherePerfManager) getRollupTime(counterId int32, instance string, managedObject ManagedObject) int {
	rollup := v.getRollupIntervalFromConfig(managedObject, counterId, instance)
	if rollup.Interval.Seconds() != 0 {
		return int(rollup.Interval.Seconds())
	}
	if v.Config.Rollup.Interval.Seconds() != 0 {
		if len(v.Config.Rollup.RollupType) != 0 {
			if funk.Contains(v.Config.Rollup.RollupType, getRollup(v.metricsInfo[counterId].Metric )) {
				return int(v.Config.Rollup.Interval.Seconds())
			}
			return 20
		}
		return int(v.Config.Rollup.Interval.Seconds())
	}
	return 20
}

func (v *VspherePerfManager) getRollupIntervalFromConfig(managedObject ManagedObject, counterId int32, instance string) RollupMetrics {
	for _, metric := range v.Config.Rollup.Metrics[PmSupportedEntities(managedObject.Entity.Type)] {
		if isMatch(v.GetProperty(managedObject, "name").(string), metric.Entities) &&
			isMatch(v.metricsInfo[counterId].Metric, metric.Metrics) &&
			isMatch(instance, metric.Instances) {
			if len(metric.RollupType) != 0 {
				if funk.Contains(metric.RollupType, getRollup(v.metricsInfo[counterId].Metric)) {
					return metric
				}
			} else {
				return metric
			}
		}
	}
	return RollupMetrics{
		Interval: time.Duration(0 * time.Second),
	}
}

func (v *VspherePerfManager) getDividedMetrics(metrics []types.PerfMetricId) [][]types.PerfMetricId {
	var dividedMetrics [][]types.PerfMetricId
	var chunkSize int
	if v.Config.MaxMetrics == 0 {
		chunkSize = len(metrics)
	} else {
		chunkSize = v.Config.MaxMetrics
	}
	for i := 0; i < len(metrics); i += chunkSize {
		end := i + chunkSize

		if end > len(metrics) {
			end = len(metrics)
		}
		dividedMetrics = append(dividedMetrics, metrics[i:end])
	}
	return dividedMetrics
}

func getStartTime(interval time.Duration, intervalId int32, client *govmomi.Client) (time.Time, error) {
	samples := int32(interval.Seconds()) / intervalId
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now, err := methods.GetCurrentTime(ctx, client)

	x := intervalId * -1 * samples
	return now.Add(time.Duration(x) * time.Second), err

}

func calculateRollup(rollup RollupTypes, values []int64) int64 {
	switch rollup {
	case Latest:
		return latest(values)
	case Maximum:
		return maximum(values)
	case Minimum:
		return minimum(values)
	case Average:
		return average(values)
	case Summation:
		return summation(values)
	default:
		return 0
	}
}

func getRollup(metric string) RollupTypes {
	switch strings.Split(metric, ".")[2] {
	case "latest":
		return Latest
	case "maximum":
		return Maximum
	case "minimum":
		return Minimum
	case "average":
		return Average
	case "summation":
		return Summation
	default:
		return ""
	}
}

func latest(values []int64) int64 {
	return values[len(values) - 1]
}

func average(values []int64) int64 {
	var total int64 = 0
	for _, value:= range values {
		total += value
	}
	return total/int64(len(values))
}

func summation(values []int64) int64 {
	var total int64 = 0
	for _, value:= range values {
		total += value
	}
	return total
}

func minimum(values []int64) int64 {
	var total int64 = 0
	for _, value:= range values {
		if total < value {
			total = value
		}
	}
	return total
}

func maximum(values []int64) int64 {
	var total int64 = 0
	for _, value:= range values {
		if total > value {
			total = value
		}
	}
	return total
}
