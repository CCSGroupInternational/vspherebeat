package datastores

import (
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/metricbeat/mb"
	"time"
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"strconv"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "datastores", New)
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
	cfgwarn.Experimental("The performancemanager datastores metricset is experimental.")

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
		string(pm.Datastores):        {"summary.url", "parent"},
		string(pm.VMs):               {},
		string(pm.DatastoreClusters): {"parent"},
		string(pm.Folders):           {"parent"},
		string(pm.Datacenters):       {},
	}

	vspherePm, err := performancemanager.Connect(m.Username, m.Password, m.Hosts[0], m.Insecure, data)

	if err == nil {

	}

	datastores := vspherePm.Get(pm.Datastores)
	for _, datastore := range datastores {
		for _, metric := range datastore.Metrics {
			var instance string
			if len(metric.Value.Instance) != 0 {
				if _, err := strconv.Atoi(metric.Value.Instance); err == nil {
					instance = vspherePm.GetProperty(vspherePm.GetObject(string(pm.VMs), "vm-" + metric.Value.Instance), "name").(string)
				} else {
					instance = metric.Value.Instance
				}
			} else {
				instance = metric.Value.Instance
			}

			var datastoreCluster, datacenter pm.ManagedObject
			flag := false
			parentObject := vspherePm.GetProperty(datastore, "parent").(pm.ManagedObject)
			for {
				switch parentType := parentObject.Entity.Type; parentType {
				case string(pm.DatastoreClusters):
					datastoreCluster = parentObject
					parentObject = vspherePm.GetProperty(parentObject, "parent").(pm.ManagedObject)
				case string(pm.Folders):
					parentObject = vspherePm.GetProperty(parentObject, "parent").(pm.ManagedObject)
				case string(pm.Datacenters):
					datacenter = parentObject
					flag = true
				}

				if flag {
					break
				}
			}

			metaData := common.MapStr{
				"name"       : vspherePm.GetProperty(datastore, "name").(string),
				"url"        : vspherePm.GetProperty(datastore, "summary.url").(string),
				"datacenter" : vspherePm.GetProperty(datacenter, "name").(string),
			}
			if len(datastoreCluster.Entity.Value) != 0 {
				metaData["datastoreCluster"] = vspherePm.GetProperty(datastoreCluster, "name").(string)
			}

			report.Event(mb.Event{
				MetricSetFields: common.MapStr{
					"metaData": metaData,
					"metric" : performancemanager.MetricWithCustomInstance(metric, instance),
				},
			})
		}
	}
}
