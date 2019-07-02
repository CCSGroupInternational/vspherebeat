package datastoresclusters

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
	mb.Registry.MustAddMetricSet("performancemanager", "datastoresclusters", New)
}

// MetricSet holds any configuration or state information. It must implement
// the mb.MetricSet interface. And this is best achieved by embedding
// mb.BaseMetricSet because it implements all of the required mb.MetricSet
// interface methods except for Fetch.
type MetricSet struct {
	mb.BaseMetricSet
	Period                   time.Duration
	Hosts                    []string
	Usernames                []string
	Passwords                []string
	Insecure                 bool
	Counters                 []interface{}
	Rollup                   []interface{}
	MaxMetrics               int
	VcenterConnectionRetries int
}

// New creates a new instance of the MetricSet. New is responsible for unpacking
// any MetricSet specific configuration options if there are any.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	config := struct{
		Period                   time.Duration `config:"period"`
		Hosts                    []string      `config:"hosts"`
		Usernames                []string      `config:"usernames"`
		Passwords                []string      `config:"passwords"`
		Insecure                 bool          `config:"insecure"`
		Counters                 []interface{} `config:"counters"`
		Rollup                   []interface{} `config:"rollup"`
		MaxMetrics               int           `config:"maxMetrics"`
		VcenterConnectionRetries int           `config:"vcenterConnectionRetries"`
	}{}
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet:            base,
		Period:                   config.Period,
		Hosts:                    config.Hosts,
		Usernames:                config.Usernames,
		Passwords:                config.Passwords,
		Insecure:                 config.Insecure,
		Counters:                 config.Counters,
		Rollup:                   config.Rollup,
		MaxMetrics:               config.MaxMetrics,
		VcenterConnectionRetries: config.VcenterConnectionRetries,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right
// format. It publishes the event which is then forwarded to the output. In case
// of an error set the Error field of mb.Event or simply call report.Error().
func (m *MetricSet) Fetch(report mb.ReporterV2) {

	t1 := time.Now()

	data := map[string][]string{
		string(pm.DatastoreClusters): {"parent", "summary.capacity"},
		string(pm.Folders):           {"parent"},
		string(pm.Datacenters):       {},
	}

	vspherePm, err := performancemanager.Connect(m.Usernames[performancemanager.IndexOf(m.Host(), m.Hosts)], m.Passwords[performancemanager.IndexOf(m.Host(), m.Hosts)], m.Host(), m.Insecure, m.Period, m.MaxMetrics, data, m.VcenterConnectionRetries)

	if err != nil {
		m.Logger().Error(err)
		return
	}

	t2 := time.Now()
	datastoresClusters := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)
	m.Logger().Info("datastoresclusters:collect:" + m.Host() + ":" + time.Now().Sub(t2).String())
	count := 0

	for _, datastoreCluster := range datastoresClusters {
		if datastoreCluster.Error != nil {
			m.Logger().Error(vspherePm.Config.Vcenter.Host + " => " + datastoreCluster.Entity.String() + " => ",  datastoreCluster.Error)
			continue
		}
		metadata := performancemanager.MetaData(vspherePm, datastoreCluster)
		// Provisioned Values
		metadata["Storage"] = common.MapStr{
			"Capacity": vspherePm.GetProperty(datastoreCluster, "summary.capacity"),
		}
		for _, metric := range datastoreCluster.Metrics {
			report.Event(mb.Event{
				MetricSetFields: common.MapStr{
					"metaData": metadata,
					"metric" : performancemanager.Metric(metric),
				},
			})
			count++
		}
	}

	m.Logger().Info("datastoresclusters:finish:" + m.Host() + ":" + time.Now().Sub(t1).String())
	m.Logger().Info("datastoresclusters:events:" + m.Host() + ":" + strconv.Itoa(count))
}
