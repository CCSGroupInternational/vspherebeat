package performancemanager

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/elastic/beats/libbeat/common"
)


func Connect(user string, pass string, host string, insecure bool, data map[string][]string) (pm.VspherePerfManager, error) {
	vspherePm := pm.VspherePerfManager{
		Config: pm.Config{
			Vcenter: pm.Vcenter{
				Username : user,
				Password : pass,
				Host     : host,
				Insecure : insecure,
			},
			Samples: 6,
			Data: data,
		},
	}
	err := vspherePm.Init()
	return vspherePm, err
}

func MetricWithCustomInstance (metric pm.Metric, instance string) common.MapStr {
	return setMetric(metric, instance)
}

func Metric(metric pm.Metric) common.MapStr {
	return setMetric(metric, metric.Value.Instance)
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

func setMetric (metric pm.Metric, instance string) common.MapStr {
	return common.MapStr{
		"info" : common.MapStr{
			"metric"    : metric.Info.Metric,
			"statsType" : metric.Info.StatsType,
			"unitInfo"  : metric.Info.UnitInfo,
		},
		"sample": common.MapStr{
			"value"    : metric.Value.Value,
			"instance" : instance,
		},
	}
}
