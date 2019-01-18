package virtualmachines

import (
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/CCSGroupInternational/vsphere-perfmanager/config"
	 pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"time"
	"github.com/elastic/beats/libbeat/common"
	"github.com/vmware/govmomi/vim25/types"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "virtualmachines", New)
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
	cfgwarn.Experimental("The performancemanager virtualmachines metricset is experimental.")

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
	vspherePmConfig := config.VspherePerfManagerConfig{
		Vcenter: config.Vcenter {
			Username : m.Username,
			Password : m.Password,
			Host     : m.Hosts[0],
			Insecure : m.Insecure,
		},
		QueryInterval: m.Period,
		Metrics: map[config.EntitiesType][]config.MetricDef{
			config.VMs: {
				config.MetricDef{
					Metric:   "net.packets*",
					Entities: []string{"dropbox"},
					Instance: []string{"vmnic\\d"},
				},
			},
		},
		Properties: []types.PropertySpec{{
			Type: string(config.VMs),
			PathSet: []string{"runtime.host"},
		}},
	}

	vspherePerfManager, err := pm.Init(&vspherePmConfig)

	if err == nil {

	}

	vms, err := vspherePerfManager.Vms()

	if err == nil {

	}

	for _, vm := range vms {
		for _, metric := range vm.Metrics {
			report.Event(mb.Event{
				MetricSetFields: common.MapStr{
					"name"   : vm.GetProperty("name").(string),
					"hostId" : vm.GetProperty("runtime.host").(types.ManagedObjectReference).Value,
					"metric" : common.MapStr{
						"info" : common.MapStr{
							"metric"    : metric.Info.Metric,
							"statsType" : metric.Info.StatsType,
							"unitInfo"  : metric.Info.UnitInfo,
						},
						"result": common.MapStr{
							"value"    : metric.Value.Value,
							"instance" : metric.Value.Instance,
						},
					},
				},
			})
		}

	}
}
