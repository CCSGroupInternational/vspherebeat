package clusters

import (
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/metricbeat/mb"
	"time"
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "clusters", New)
}

// MetricSet holds any configuration or state information. It must implement
// the mb.MetricSet interface. And this is best achieved by embedding
// mb.BaseMetricSet because it implements all of the required mb.MetricSet
// interface methods except for Fetch.
type MetricSet struct {
	mb.BaseMetricSet
	Period   time.Duration
	Hosts    []string
	Username string
	Password string
	Insecure bool
}

// New creates a new instance of the MetricSet. New is responsible for unpacking
// any MetricSet specific configuration options if there are any.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	cfgwarn.Experimental("The performancemanager clusters metricset is experimental.")

	config := struct{
		Period   time.Duration `config:"period"`
		Hosts    []string      `config:"hosts"`
		Username string        `config:"username"`
		Password string        `config:"password"`
		Insecure bool          `config:"insecure"`
	}{}
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet: base,
		Period:   config.Period,
		Hosts:    config.Hosts,
		Username: config.Username,
		Password: config.Password,
		Insecure: config.Insecure,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right
// format. It publishes the event which is then forwarded to the output. In case
// of an error set the Error field of mb.Event or simply call report.Error().
func (m *MetricSet) Fetch(report mb.ReporterV2) {
	vspherePm := pm.VspherePerfManager{
		Config: pm.Config{
			Vcenter: pm.Vcenter{
				Username : m.Username,
				Password : m.Password,
				Host     : m.Hosts[0],
				Insecure : m.Insecure,
			},
			Samples: 6,
			Data: map[string][]string{
				string(pm.Clusters): {"parent"},
				"Folder": {"parent"},
				string(pm.Datacenter): {},
			},
		},
	}

	err := vspherePm.Init()

	if err == nil {

	}

	clusters := vspherePm.Get(pm.Clusters)

	for _, cluster := range clusters {
		for _, metric := range cluster.Metrics {

			var datacenter pm.ManagedObject
			switch parentType := vspherePm.GetProperty(cluster, "parent").(pm.ManagedObject).Entity.Type; parentType {
			case "Folder":
				for {
					parent := vspherePm.GetProperty(vspherePm.GetProperty(cluster, "parent").(pm.ManagedObject), "parent").(pm.ManagedObject)
					if parent.Entity.Type == string(pm.Datacenter) {
						datacenter = parent
						break
					}
				}
			case string(pm.Datacenter):
				datacenter = vspherePm.GetProperty(cluster, "parent").(pm.ManagedObject)
			}

			report.Event(mb.Event{
				MetricSetFields: common.MapStr{
					"metaData": common.MapStr{
						"name" : vspherePm.GetProperty(cluster, "name").(string),
						"datacenter" : vspherePm.GetProperty(datacenter, "name").(string),
					},
					"metric" : common.MapStr{
						"info" : common.MapStr{
							"metric"    : metric.Info.Metric,
							"statsType" : metric.Info.StatsType,
							"unitInfo"  : metric.Info.UnitInfo,
						},
						"sample": common.MapStr{
							"value"    : metric.Value.Value,
							"instance" : metric.Value.Instance,
						},
					},
				},
			})
		}
	}
}
