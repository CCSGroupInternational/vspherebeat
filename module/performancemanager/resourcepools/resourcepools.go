package resourcepools

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
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
	Username   string
	Password   string
	Insecure   bool
	Counters   []interface{}
	Rollup     []interface{}
	MaxQueries int
}

// New creates a new instance of the MetricSet. New is responsible for unpacking
// any MetricSet specific configuration options if there are any.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	cfgwarn.Experimental("The performancemanager resourcepools metricset is experimental.")

	config := struct{
		Period     time.Duration `config:"period"`
		Hosts      []string      `config:"hosts"`
		Username   string        `config:"username"`
		Password   string        `config:"password"`
		Insecure   bool          `config:"insecure"`
		Counters   []interface{} `config:"counters"`
		Rollup     []interface{} `config:"rollup"`
		MaxQueries int           `config:"maxQueries"`
	}{}
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet: base,
		Period:        config.Period,
		Hosts:         config.Hosts,
		Username:      config.Username,
		Password:      config.Password,
		Insecure:      config.Insecure,
		Counters:      config.Counters,
		Rollup:        config.Rollup,
		MaxQueries:    config.MaxQueries,
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

	for _, host := range  m.Hosts {
		vspherePm, err := performancemanager.Connect(m.Username, m.Password, host, m.Insecure, m.Period, m.MaxQueries, data)

		if err != nil {
			m.Logger().Panic(err)
			return
		}

		resourcePools := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)

		for _, resourcePool := range resourcePools {
			if resourcePool.Error != nil {
				m.Logger().Error(resourcePool.Entity.String() + " => ",  resourcePool.Error)
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
	}
}
