package performancemanager

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/elastic/beats/libbeat/common"
	"time"
)

func Connect(user string, pass string, host string, insecure bool, interval time.Duration, maxMetrics int, data map[string][]string) (pm.VspherePerfManager, error) {
	vspherePm := pm.VspherePerfManager{
		Config: pm.Config{
			Vcenter: pm.Vcenter{
				Username: user,
				Password: pass,
				Host:     host,
				Insecure: insecure,
			},
			Interval:   interval,
			Data:       data,
			MaxMetrics: maxMetrics,
		},
	}
	err := vspherePm.Init()
	return vspherePm, err
}

func Fetch(metricset string, metrics []interface{}, rollup []interface{}, vspherePm *pm.VspherePerfManager) []pm.ManagedObject {
	if len(metrics) > 0 {
		vspherePm.Config.Metrics = metricsToFilter(metrics, metricset)
	}
	if len(rollup) > 0 {
		vspherePm.Config.Rollup = rollupFromConfig(rollup, metricset)
	}
	return vspherePm.Get(getObjectsType(metricset))
}

func MetricWithCustomInstance(metric pm.Metric, instance string) common.MapStr {
	return setMetric(metric, instance)
}

func Metric(metric pm.Metric) common.MapStr {
	var instance string
	if len(metric.Value.Instance) == 0 {
		instance = "*"
	} else {
		instance = metric.Value.Instance
	}
	return setMetric(metric, instance)
}

func MetaData(vspherePm pm.VspherePerfManager, object pm.ManagedObject) common.MapStr {
	parentObjects := getParents(vspherePm, object)

	metadata := common.MapStr{
		"name":    vspherePm.GetProperty(object, "name").(string),
		"Vcenter": vspherePm.Config.Vcenter.Host,
	}

	if parentObjects != nil {
		for objectType, parents := range parentObjects {
			objectHierarchy := ""

			for i, parent := range parents {
				objectHierarchy += vspherePm.GetProperty(parent, "name").(string)
				if i+1 < len(parents) {
					objectHierarchy += "/"
				}
			}

			parent := vspherePm.GetProperty(parents[0], "name").(string)
			metadata[objectType] = parent
		}
	}

	return metadata
}

func Datacenter(vspherePm pm.VspherePerfManager, cluster pm.ManagedObject) pm.ManagedObject {
	var datacenter pm.ManagedObject
	switch parentType := vspherePm.GetProperty(cluster, "parent").(pm.ManagedObject).Entity.Type; parentType {
	case "Folder":
		for {
			parent := vspherePm.GetProperty(vspherePm.GetProperty(cluster, "parent").(pm.ManagedObject), "parent").(pm.ManagedObject)
			if parent.Entity.Type == string(pm.Datacenters) {
				datacenter = parent
				break
			}
		}
	case string(pm.Datacenters):
		datacenter = vspherePm.GetProperty(cluster, "parent").(pm.ManagedObject)
	}
	return datacenter
}

func getParents(vspherePm pm.VspherePerfManager, object pm.ManagedObject) map[string][]pm.ManagedObject {
	if object.Entity.Type == string(pm.Datacenters) {
		return nil
	}
	objectTemp := vspherePm.GetProperty(object, "parent")
	parents := make(map[string][]pm.ManagedObject)
	flag := false
	for {
		if objectTemp == nil {
			break
		}

		parentObject := objectTemp.(pm.ManagedObject)
		switch parentType := parentObject.Entity.Type; parentType {
		case string(pm.Datacenters):
			parents[parentType] = []pm.ManagedObject{parentObject}
			flag = true
		default:
			parents[parentType] = append(parents[parentType], parentObject)
			objectTemp = vspherePm.GetProperty(parentObject, "parent").(pm.ManagedObject)
		}
		if flag {
			break
		}
	}
	return parents
}

func setMetric(metric pm.Metric, instance string) common.MapStr {
	return common.MapStr{
		"info": common.MapStr{
			"metric":    metric.Info.Metric,
			"statsType": metric.Info.StatsType,
			"unitInfo":  metric.Info.UnitInfo,
		},
		"sample": common.MapStr{
			"value":     metric.Value.Value,
			"instance":  instance,
			"timestamp": metric.Value.Timestamp,
		},
	}
}

func metricsToFilter(metrics interface{}, metricset string) map[pm.PmSupportedEntities][]pm.MetricDef {

	vsphereMetrics := make(map[pm.PmSupportedEntities][]pm.MetricDef)

	for _, counter := range metrics.([]interface{}) {
		if counter.(map[string]interface{})[metricset] != nil {
			for _, metric := range counter.(map[string]interface{})[metricset].([]interface{}) {
				var metricDef pm.MetricDef
				if metric.(map[string]interface{})["Entities"] != nil {
					entities := metric.(map[string]interface{})["Entities"].([]interface{})
					for _, entity := range entities {
						metricDef.Entities = append(metricDef.Entities, entity.(string))
					}
				}

				if metric.(map[string]interface{})["Metrics"] != nil {
					metrics := metric.(map[string]interface{})["Metrics"].([]interface{})
					for _, met := range metrics {
						metricDef.Metrics = append(metricDef.Metrics, met.(string))
					}
				}

				if metric.(map[string]interface{})["Instances"] != nil {
					instances := metric.(map[string]interface{})["Instances"].([]interface{})
					for _, instance := range instances {
						metricDef.Instances = append(metricDef.Instances, instance.(string))
					}
				}

				vsphereMetrics[getObjectsType(metricset)] = append(vsphereMetrics[getObjectsType(metricset)], metricDef)
			}
			break
		}
	}

	return vsphereMetrics
}

func getObjectsType(metricset string) pm.PmSupportedEntities {
	switch metricset {
	case "virtualmachines":
		return pm.VMs
	case "hosts":
		return pm.Hosts
	case "clusters":
		return pm.Clusters
	case "datastores":
		return pm.Datastores
	case "resourcepools":
		return pm.ResourcePools
	case "datacenters":
		return pm.Datacenters
	case "vapps":
		return pm.Vapps
	case "datastoresclusters":
		return pm.DatastoreClusters
	case "virtualswitches":
		return pm.VirtualSwitches
	}

	// TODO Fix this
	return pm.VMs
}

func rollupFromConfig(rollup interface{}, metricset string) pm.Rollup {
	var rollupConfig pm.Rollup
	if rollup != nil {
		for _, rollupStruct := range rollup.([]interface{}) {
			interval := rollupStruct.(map[string]interface{})["Interval"].(uint64)
			rollupConfig.Interval = time.Duration(interval) * time.Second
			for _, rollupType := range rollupStruct.(map[string]interface{})["RollupType"].([]interface{}) {
				rollupConfig.RollupType = append(rollupConfig.RollupType, pm.RollupTypes(rollupType.(string)))
			}
			if rollupStruct.(map[string]interface{})["Metrics"] != nil {
				for _, metrics := range rollupStruct.(map[string]interface{})["Metrics"].([]interface{}) {
					rollupConfig.Metrics = map[pm.PmSupportedEntities][]pm.RollupMetrics{}
					if metrics.(map[string]interface{})[metricset].(interface{}) != nil {
						for _, rollupMetric := range metrics.(map[string]interface{})[metricset].([]interface{}) {

							var rollupMetrics []string
							var rollupTypes []pm.RollupTypes
							var entities []string
							var instances []string

							if rollupMetric.(map[string]interface{})["Metrics"] != nil {
								for _, item := range rollupMetric.(map[string]interface{})["Metrics"].([]interface{}) {
									rollupMetrics = append(rollupMetrics, item.(string))
								}
							}

							if rollupMetric.(map[string]interface{})["RollupType"] != nil {
								for _, item := range rollupMetric.(map[string]interface{})["RollupType"].([]interface{}) {
									rollupTypes = append(rollupTypes, pm.RollupTypes(item.(string)))
								}
							}

							if rollupMetric.(map[string]interface{})["Entities"] != nil {
								for _, item := range rollupMetric.(map[string]interface{})["Entities"].([]interface{}) {
									entities = append(entities, item.(string))
								}
							}

							if rollupMetric.(map[string]interface{})["Instances"] != nil {
								for _, item := range rollupMetric.(map[string]interface{})["Instances"].([]interface{}) {
									instances = append(instances, item.(string))
								}
							}

							metric := pm.RollupMetrics{
								Metrics:    rollupMetrics,
								RollupType: rollupTypes,
								Instances:  instances,
								Entities:   entities,
								Interval:   time.Duration(rollupMetric.(map[string]interface{})["Interval"].(uint64)) * time.Second,
							}
							rollupConfig.Metrics[getObjectsType(metricset)] = append(rollupConfig.Metrics[getObjectsType(metricset)], metric)
						}
					}
				}
			}
		}
	}
	return rollupConfig
}

func IndexOf(haystack string, needle []string) int {
	for p, v := range needle {
		if v == haystack {
			return p
		}
	}
	return -1
}
