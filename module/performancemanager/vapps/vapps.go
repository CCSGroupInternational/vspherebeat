package vapps

import (
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/metricbeat/mb"
	"time"
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "vapps", New)
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
	cfgwarn.Experimental("The performancemanager vapps metricset is experimental.")

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

	data := map[string][]string{
		string(pm.Vapps):         {"parent"},
		string(pm.ResourcePools): {"parent"},
		string(pm.Clusters):      {"parent"},
		string(pm.Folders):       {"parent"},
		string(pm.Datacenters):   {},
	}

	vspherePm, err := performancemanager.Connect(m.Username, m.Password, m.Hosts[0], m.Insecure, data)

	if err == nil {

	}

	vapps := vspherePm.Get(pm.Vapps)

	for _, vapp := range vapps {
		for _, metric := range vapp.Metrics {
			metaData := common.MapStr{
				"name"   :  vspherePm.GetProperty(vapp, "name").(string),
			}
			var cluster pm.ManagedObject
			switch parentType := vspherePm.GetProperty(vapp, "parent").(pm.ManagedObject).Entity.Type; parentType {
			case string(pm.ResourcePools):
				resourcePool := vspherePm.GetProperty(vapp, "parent").(pm.ManagedObject)
				metaData["resourcePool"] = vspherePm.GetProperty(resourcePool, "name").(string)
				cluster = vspherePm.GetProperty(resourcePool, "parent").(pm.ManagedObject)
				metaData["cluster"] = vspherePm.GetProperty(cluster, "name").(string)
			}

			metaData["datacenter"] = vspherePm.GetProperty(performancemanager.Datacenter(vspherePm, cluster), "name").(string)

			report.Event(mb.Event{
				MetricSetFields: common.MapStr{
					"metaData": metaData,
					"metric" : performancemanager.Metric(metric),
				},
			})
		}

	}
}
