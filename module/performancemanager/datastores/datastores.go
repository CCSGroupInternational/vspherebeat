package datastores

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/metricbeat/mb"
	"strconv"
	"time"
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
	Period     time.Duration
	Hosts      []string
	Usernames  []string
	Passwords  []string
	Insecure   bool
	Counters   []interface{}
	Rollup     []interface{}
	MaxMetrics int
}

// New creates a new instance of the MetricSet. New is responsible for unpacking
// any MetricSet specific configuration options if there are any.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	config := struct{
		Period     time.Duration `config:"period"`
		Hosts      []string      `config:"hosts"`
		Usernames  []string      `config:"usernames"`
		Passwords  []string      `config:"passwords"`
		Insecure   bool          `config:"insecure"`
		Counters   []interface{} `config:"counters"`
		Rollup     []interface{} `config:"rollup"`
		MaxMetrics int           `config:"maxMetrics"`
	}{}
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet: base,
		Period:        config.Period,
		Hosts:         config.Hosts,
		Usernames:     config.Usernames,
		Passwords:     config.Passwords,
		Insecure:      config.Insecure,
		Counters:      config.Counters,
		Rollup:        config.Rollup,
		MaxMetrics:    config.MaxMetrics,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right
// format. It publishes the event which is then forwarded to the output. In case
// of an error set the Error field of mb.Event or simply call report.Error().
func (m *MetricSet) Fetch(report mb.ReporterV2) {

	data := map[string][]string{
		string(pm.Datastores):        {"summary.url", "parent", "info.maxVirtualDiskCapacity", "summary.capacity"},
		string(pm.VMs):               {},
		string(pm.DatastoreClusters): {"parent"},
		string(pm.Folders):           {"parent"},
		string(pm.Datacenters):       {},
	}

	for i, host := range  m.Hosts {
		vspherePm, err := performancemanager.Connect(m.Usernames[i], m.Passwords[i], host, m.Insecure, m.Period, m.MaxMetrics, data)

		if err != nil {
			m.Logger().Panic(err)
			return
		}

		m.Logger().Info("Starting collect Datastores metrics from Vcenter : " + vspherePm.Config.Vcenter.Host + " ", time.Now())

		datastores := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)

		for _, datastore := range datastores {
			if datastore.Error != nil {
				m.Logger().Error(vspherePm.Config.Vcenter.Host + " => " + datastore.Entity.String() + " => ",  datastore.Error)
				continue
			}
			metaData := performancemanager.MetaData(vspherePm, datastore)
			metaData["url"] = vspherePm.GetProperty(datastore, "summary.url").(string)
			// Provisioned Values
			metaData["Storage"] = common.MapStr{
				"Capacity"              : vspherePm.GetProperty(datastore, "summary.capacity"),
				"MaxVirtualDiskCapacity": vspherePm.GetProperty(datastore, "info.maxVirtualDiskCapacity"),
			}
			for _, metric := range datastore.Metrics {
				var instance string
				if len(metric.Value.Instance) != 0 {
					if _, err := strconv.Atoi(metric.Value.Instance); err == nil {
						instance = vspherePm.GetProperty(vspherePm.GetObject(string(pm.VMs), "vm-" + metric.Value.Instance), "name").(string)
					} else {
						instance = metric.Value.Instance
					}
				} else {
					instance = "*"
				}

				report.Event(mb.Event{
					MetricSetFields: common.MapStr{
						"metaData": metaData,
						"metric" : performancemanager.MetricWithCustomInstance(metric, instance),
					},
				})
			}
		}

		m.Logger().Info("Finishing collect Datastores metrics from Vcenter : " + vspherePm.Config.Vcenter.Host + " ", time.Now())
	}
}
