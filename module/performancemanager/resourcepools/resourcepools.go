package resourcepools

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/metricbeat/mb"
	"time"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "resourcepools", New)
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
		string(pm.ResourcePools)    : {"parent", "summary.configuredMemoryMB"},
		string(pm.Clusters)         : {"parent"},
		string(pm.Folders)          : {"parent"},
		string(pm.Datacenters)      : {},
		string(pm.Vapps)            : {"parent"},
		string(pm.ComputeResources) : {"name", "parent"},
	}

	vspherePm, err := performancemanager.Connect(m.Usernames[performancemanager.IndexOf(m.Host(), m.Hosts)], m.Passwords[performancemanager.IndexOf(m.Host(), m.Hosts)], m.Host(), m.Insecure, m.Period, m.MaxMetrics, data)

	if err != nil {
		m.Logger().Panic(err)
		return
	}

	m.Logger().Info("Starting collect Resource Pools metrics from Vcenter : " + vspherePm.Config.Vcenter.Host + " ", time.Now())

	resourcePools := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)

	for _, resourcePool := range resourcePools {
		if resourcePool.Error != nil {
			m.Logger().Error(vspherePm.Config.Vcenter.Host + " => " + resourcePool.Entity.String() + " => ",  resourcePool.Error)
			continue
		}
		metadata := performancemanager.MetaData(vspherePm, resourcePool)
		metadata["Ram"] = common.MapStr{
			"ConfiguredMemoryMB": vspherePm.GetProperty(resourcePool, "summary.configuredMemoryMB"),
		}

		for _, metric := range resourcePool.Metrics {
			report.Event(mb.Event{
				MetricSetFields: common.MapStr{
					"metaData": metadata,
					"metric" : performancemanager.Metric(metric),
				},
			})
		}
	}

	m.Logger().Info("Finishing collect Resource Pools metrics from Vcenter : " + vspherePm.Config.Vcenter.Host + " ", time.Now())
}
